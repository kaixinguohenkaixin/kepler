package endorser

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/vntchain/kepler/protos/common"
	"github.com/vntchain/kepler/utils"
	"github.com/golang/protobuf/proto"
)

const RETRACE_FILE_NAME = "./retrace.json"

type RetraceConf struct {
	PeerClient      *PeerClient
	Channel         string
	RetraceInterval time.Duration
	Creator         []byte
	Signer          interface{}
}

type retracer struct {
	peerClient *PeerClient
	syncStatus map[string]uint64
	syncMu     sync.Mutex
	cache      chan *[]ChaincodeEventInfo
	channel    string
	//chaincode       string
	retraceInterval            time.Duration
	creator                    []byte
	signer                     interface{}
	RegisteredTxEvent          map[string]chan int
	RegisteredEventByEventName map[string]chan ChaincodeEventInfo
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
	tracer.RegisteredTxEvent = make(map[string]chan int)
	tracer.RegisteredEventByEventName = make(map[string]chan ChaincodeEventInfo)

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
	logger.Errorf("syncStatus: %#v", tracer.syncStatus)
	return &tracer
}

func (rt *retracer) Process() {
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
			rt.dispatcher()
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

/*
	c=1 重新发送 mytx
	c=2 mytx 成功
	c=3 所有节点发送成功，整体事件成功
	c=4 回滚
	c=5 回滚成功
*/
func (rt *retracer) RegisterTxId(txid string, c chan int) {
	rt.RegisteredTxEvent[txid] = c
}

func (rt *retracer) UnRegisterTxId(txid string) {
	delete(rt.RegisteredTxEvent, txid)
}

func (rt *retracer) RegisterEventName(eventname string, cc chan ChaincodeEventInfo) {
	rt.RegisteredEventByEventName[eventname] = cc
}

func (rt *retracer) UnRegisterEventName(eventname string) {
	delete(rt.RegisteredEventByEventName, eventname)
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

		txsFltr := utils.TxValidationFlags(block.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER])
		for j, tdata := range block.Data.Data {

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

			tx := payload
			if tx != nil {
				chdr, err := utils.UnmarshalChannelHeader(tx.Header.ChannelHeader)
				if err != nil {
					logger.Panic("Error extracting channel header")
				}
				c, ok := rt.RegisteredTxEvent[chdr.TxId]
				if txsFltr.IsInvalid(j) {
					if ok {
						logger.Errorf("Transaction invalid: TxID: %s---ok:%v\n", chdr.TxId, ok)
						logger.Errorf("the transaction error is %v", tx)
						//c=1 重新发送 mytx
						c <- 1
						rt.UnRegisterTxId(chdr.TxId)
					}
					continue
				} else {
					if ok {
						c <- 2
						rt.UnRegisterTxId(chdr.TxId)
						logger.Errorf("Transaction TxID:%s ---ok:%v", chdr.TxId, ok)
					}
				}
			}

			// chaincode event
			if ccEvent, channelID, err := getChaincodeEvent(payload.Data, channelHeader); err != nil {
				logger.Warningf("getChainCodeEvent return error: %v\n", err)
			} else if ccEvent != nil && ccEvent.Payload != nil {
				logger.Errorf("ccEvent: %#v", *ccEvent)

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
	// logger.Errorf("the getHeight and the channel is ",rt.channel,rt.creator)
	return GetHeight(rt.peerClient, rt.channel, rt.creator, rt.signer.(*ecdsa.PrivateKey))
}

func (rt *retracer) dispatcher() {
	var single ChaincodeEventInfo
	logPtr := <-rt.cache
	for _, single = range *logPtr {
		if log, ok := rt.RegisteredEventByEventName[single.EventName]; ok {
			log <- single
			logger.Infof("serve log: %#v", single)
		} else {
			logger.Warningf("UnRegistered %s\n: \t%#v\n", single.EventName, single)
		}
	}
}

func (rt *retracer) getSyncHeight() uint64 {
	rt.syncMu.Lock()
	defer rt.syncMu.Unlock()
	return rt.syncStatus[rt.channel]
}
