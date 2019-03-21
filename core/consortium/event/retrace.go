package event

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/op/go-logging"
	"github.com/vntchain/kepler/core/consortium/endorser"
	"github.com/vntchain/kepler/protos/common"
	"github.com/vntchain/kepler/utils"
)

var logger = logging.MustGetLogger("core/consortium/event")

const RETRACE_FILE_NAME = "./retrace.json"

type RetraceConf struct {
	PeerClient      *endorser.PeerClient
	Channel         string
	RetraceInterval time.Duration
	Creator         []byte
	Signer          interface{}
}

type retracer struct {
	peerClient *endorser.PeerClient
	syncStatus map[string]uint64
	syncMu     sync.Mutex
	cache      chan *[]ChaincodeEventInfo
	channel    string
	//chaincode       string
	retraceInterval time.Duration
	creator         []byte
	signer          interface{}
}

func InitRetracer(config RetraceConf) *retracer {
	logger.Debugf("Init Retracer")
	var tracer retracer
	tracer.peerClient = config.PeerClient
	tracer.channel = config.Channel
	tracer.retraceInterval = config.RetraceInterval
	tracer.creator = config.Creator
	tracer.signer = config.Signer
	tracer.syncStatus = make(map[string]uint64)

	tracer.cache = make(chan *[]ChaincodeEventInfo, 1)
	_, err := os.Stat(RETRACE_FILE_NAME)
	if os.IsNotExist(err) {
		logger.Debugf(`Can't find serialization file "%s",init event checker for the first time!`, RETRACE_FILE_NAME)
		ioutil.WriteFile(RETRACE_FILE_NAME, []byte("{}"), 0644)
		tracer.syncStatus[config.Channel] = 0
	} else {
		serialization, err := ioutil.ReadFile(RETRACE_FILE_NAME)
		if err != nil {
			panic(fmt.Sprintf(`Initialize retrace history from "%s" error: %s`, RETRACE_FILE_NAME, err))
		}
		err = json.Unmarshal(serialization, &tracer.syncStatus)
		if err != nil {
			panic(fmt.Sprintf(`Unmarshal history handled block error: %s`, err))
		}
	}
	logger.Errorf("[consortium retracer] syncStatus: %#v", tracer.syncStatus)
	return &tracer
}

func (rt *retracer) Process(log chan ChaincodeEventInfo) {
	for {
		time.Sleep(rt.retraceInterval)
		height, _ := rt.getHeight()
		logger.Debugf("Current height:%d", height)
		syncHeight := rt.getSyncHeight()
		logger.Debugf("Current syncHeight:%d", syncHeight)
		if height <= syncHeight {
			continue
		}
		events := rt.seekEventsInBlocks(syncHeight, height)
		if events != nil {
			rt.cache <- events
			rt.dispatcher(log)
		}
		rt.updateStatus(height)
		rt.SyncToFile()

	}

}

func (rt *retracer) updateStatus(height uint64) {
	rt.syncMu.Lock()
	defer rt.syncMu.Unlock()
	rt.syncStatus[rt.channel] = height
}

func (rt *retracer) SyncToFile() {
	rt.syncMu.Lock()
	defer rt.syncMu.Unlock()
	snapShot, err := json.Marshal(rt.syncStatus)
	if len(rt.syncStatus) == 0 || len(snapShot) == 0 {
		snapShot = []byte("{}")
	}
	if err != nil {
		logger.Errorf("Serialization error when marshaling channelHandled: %s", err)
		return
	}
	logger.Debugf("Start persist data to disk,content: %s", string(snapShot))
	f, err := ioutil.TempFile(filepath.Dir(RETRACE_FILE_NAME), fmt.Sprintf(".%s.tmp", filepath.Base(RETRACE_FILE_NAME)))
	if err != nil {
		logger.Errorf("Serialization creating temp file error: %s", err)
		return
	}
	if _, err := f.Write(snapShot); err != nil {
		f.Close()
		os.Remove(f.Name())
		logger.Errorf("Serialization writing to temp file error: %s", err)
		return
	}
	f.Close()
	err = os.Rename(f.Name(), RETRACE_FILE_NAME)
	if err != nil {
		logger.Errorf("Rename temp file to file error: %s", err)
		return
	}
	logger.Debug("Data persisted successful!")
}

//seek logs in block num [from,to)
func (rt *retracer) seekEventsInBlocks(from uint64, to uint64) *[]ChaincodeEventInfo {
	var result []ChaincodeEventInfo

	for i := from; i < to; i++ {
		block, err := GetBlockByNumber(rt.peerClient, rt.channel, i, rt.creator, rt.signer.(*ecdsa.PrivateKey))
		logger.Debugf("Revieve block %d\n", block.Header.GetNumber())
		if err != nil {
			logger.Panic(err)
		}
		txFilter := utils.TxValidationFlags(block.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER])
		for i, tdata := range block.Data.Data {
			if txFilter.IsInvalid(i) {
				//invalid tx
				continue
			}
			env, err := utils.GetEnvelopeFromBlock(tdata)
			if err != nil {
				logger.Errorf("[Event] Extracting Envelope from block error: %s\n", err)
				continue
			}

			// get the payload from the envelope
			payload, err := utils.GetPayload(env)
			if err != nil {
				logger.Errorf("[Event] Extracting Payload from envelope error: %s\n", err)
				continue
			}
			channelHeaderBytes := payload.Header.ChannelHeader
			channelHeader := &common.ChannelHeader{}
			err = proto.Unmarshal(channelHeaderBytes, channelHeader)
			if err != nil {
				logger.Errorf("[Event - %s] Extracting ChannelHeader from payload error: %s\n", channelHeader.ChannelId, err)
				continue
			}

			// chaincode event
			if ccEvent, channelID, err := getChaincodeEvent(payload.Data, channelHeader); err != nil {
				logger.Warningf("getChainCodeEvent return error: %v\n", err)
			} else if ccEvent != nil && ccEvent.Payload != nil {
				logger.Errorf("[consortium retrace] ccEvent: %#v", *ccEvent)
				result = append(result, ChaincodeEventInfo{
					ChaincodeID: ccEvent.ChaincodeId,
					TxID:        ccEvent.TxId,
					EventName:   ccEvent.EventName,
					Payload:     ccEvent.Payload,
					ChannelID:   channelID,
				})
			}
		}
	}

	return &result
}

func (rt *retracer) getHeight() (uint64, error) {
	return GetHeight(rt.peerClient, rt.channel, rt.creator, rt.signer.(*ecdsa.PrivateKey))
}

func (rt *retracer) dispatcher(log chan ChaincodeEventInfo) {
	var single ChaincodeEventInfo
	logPtr := <-rt.cache
	for _, single = range *logPtr {
		log <- single
		logger.Errorf("[consortium retrace]serve log: %#v", single)
	}
}

func (rt *retracer) getSyncHeight() uint64 {
	rt.syncMu.Lock()
	defer rt.syncMu.Unlock()
	return rt.syncStatus[rt.channel]
}
