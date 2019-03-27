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

	"github.com/golang/protobuf/proto"
	"github.com/vntchain/kepler/protos/common"
	"github.com/vntchain/kepler/utils"
)

const RetraceFileName = "./retrace.json"

type RetraceConf struct {
	PeerClient      *PeerClient
	Channel         string
	RetraceInterval time.Duration
	Creator         []byte
	Signer          interface{}
}

type retracer struct {
	RetraceConf
	syncStatus                 map[string]uint64
	syncMu                     sync.Mutex
	cache                      chan *[]ChaincodeEventInfo
	RegisteredTxEvent          map[string]chan int
	RegisteredEventByEventName map[string]chan ChaincodeEventInfo
}

func InitRetracer(config RetraceConf) (tracer *retracer, err error) {
	tracer = &retracer{}
	tracer.RetraceConf = config
	tracer.syncStatus = make(map[string]uint64)
	tracer.RegisteredTxEvent = make(map[string]chan int)
	tracer.RegisteredEventByEventName = make(map[string]chan ChaincodeEventInfo)

	_, err = os.Stat(RetraceFileName)
	if os.IsNotExist(err) {
		logger.Infof("[consortium sdk retracer] Can't find retrace file [%s], init event checker for the first time", RetraceFileName)
		err = ioutil.WriteFile(RetraceFileName, []byte("{}"), 0644)
		if err != nil {
			err = fmt.Errorf("[consortium sdk retracer] write retrace file [%s] failed: %s", RetraceFileName, err)
			logger.Error(err)
			return
		}
		tracer.syncStatus[config.Channel] = 0
	} else {
		serialization, err := ioutil.ReadFile(RetraceFileName)
		if err != nil {
			err = fmt.Errorf("[consortium sdk retracer] initialize retrace history from [%s] failed: %s", RetraceFileName, err)
			logger.Error(err)
			return nil, err
		}
		err = json.Unmarshal(serialization, &tracer.syncStatus)
		if err != nil {
			err = fmt.Errorf("[consortium sdk retracer] unmarshal retrace history failed: %s", err)
			logger.Error(err)
			return nil, err
		}
	}
	logger.Debugf("[consortium sdk retracer] retrace sync status: %#v", tracer.syncStatus)
	return
}

func (rt *retracer) Process() {
	for {
		time.Sleep(rt.RetraceInterval)
		height, err := GetHeight(rt.PeerClient, rt.Channel, rt.Creator, rt.Signer.(*ecdsa.PrivateKey))
		if err != nil {
			logger.Errorf("[consortium sdk retracer] get height failed: %s", err)
			continue
		}
		logger.Debugf("[consortium sdk retracer] block height: %d", height)
		syncHeight := rt.getSyncHeight()
		logger.Debugf("[consortium sdk retracer] sync height: %d", syncHeight)
		if height <= syncHeight {
			continue
		}

		events, err := rt.seekEventsInBlocks(syncHeight, height)
		if err != nil {
			logger.Errorf("[consortium sdk retracer] seek events in blocks failed: %s", err)
			continue
		}
		if events != nil {
			for _, single := range events {
				if log, ok := rt.RegisteredEventByEventName[single.EventName]; ok {
					log <- single
				} else {
					logger.Warningf("[consortium sdk retracer] unRegistered event [%s]", single.EventName)
				}
			}
		}
		rt.updateStatus(height)
		rt.SyncToFile()
	}
}

func (rt *retracer) updateStatus(height uint64) {
	rt.syncMu.Lock()
	defer rt.syncMu.Unlock()
	rt.syncStatus[rt.Channel] = height
}

func (rt *retracer) SyncToFile() error {
	rt.syncMu.Lock()
	snapShot, err := json.Marshal(rt.syncStatus)
	if len(rt.syncStatus) == 0 || len(snapShot) == 0 {
		snapShot = []byte("{}")
	}
	rt.syncMu.Unlock()
	if err != nil {
		err = fmt.Errorf("[consortium sdk retracer] marshal sync status failed: %s", err)
		logger.Error(err)
		return err
	}

	f, err := ioutil.TempFile(filepath.Dir(RetraceFileName), fmt.Sprintf(".%s.tmp", filepath.Base(RetraceFileName)))
	if err != nil {
		err = fmt.Errorf("[consortium sdk retracer] create retrace temp file failed: %s", err)
		logger.Error(err)
		return err
	}
	defer f.Close()

	if _, err := f.Write(snapShot); err != nil {
		err = fmt.Errorf("[consortium sdk retracer] write to temp file failed: %s", err)
		logger.Error(err)
		err = os.Remove(f.Name())
		if err != nil {
			err = fmt.Errorf("[consortium sdk retracer] remove file failed: %s", err)
			logger.Error(err)
		}
		return err
	}
	err = os.Rename(f.Name(), RetraceFileName)
	if err != nil {
		err = fmt.Errorf("[consortium sdk retracer] rename temp file to retrace file failed: %s", err)
		logger.Error(err)
		return err
	}
	logger.Debugf("[consortium sdk retracer] persist data to retrace file: %s", string(snapShot))
	return nil
}

func (rt *retracer) getSyncHeight() uint64 {
	rt.syncMu.Lock()
	defer rt.syncMu.Unlock()
	return rt.syncStatus[rt.Channel]
}

// seekEventsInBlocks seek logs in block num [from,to)
//
//	c=1 重新发送 mytx
//	c=2 mytx 成功
//	c=3 所有节点发送成功，整体事件成功
//	c=4 回滚
//	c=5 回滚成功
func (rt *retracer) seekEventsInBlocks(from uint64, to uint64) (result []ChaincodeEventInfo, err error) {
	for i := from; i < to; i++ {
		block, err := GetBlockByNumber(rt.PeerClient, rt.Channel, i, rt.Creator, rt.Signer.(*ecdsa.PrivateKey))
		if err != nil {
			err = fmt.Errorf("[consortium sdk retracer] get block by number failed: %s", err)
			logger.Error(err)
			return nil, err
		}

		txsFltr := utils.TxValidationFlags(block.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER])
		for j, tdata := range block.Data.Data {
			env, err := utils.GetEnvelopeFromBlock(tdata)
			if err != nil {
				logger.Errorf("[consortium sdk retracer] extracting Envelope from block failed: %s\n", err)
				continue
			}
			payload, err := utils.GetPayload(env)
			if err != nil {
				logger.Errorf("[consortium sdk retracer] extracting Payload from envelope failed: %s\n", err)
				continue
			}
			channelHeaderBytes := payload.Header.ChannelHeader
			channelHeader := &common.ChannelHeader{}
			err = proto.Unmarshal(channelHeaderBytes, channelHeader)
			if err != nil {
				logger.Errorf("[consortium sdk retracer] extracting ChannelHeader from payload failed: %s\n", err)
				continue
			}

			tx := payload
			if tx != nil {
				chdr, err := utils.UnmarshalChannelHeader(tx.Header.ChannelHeader)
				if err != nil {
					logger.Errorf("[consortium sdk retracer] unmarshal ChannelHeader failed: %s\n", err)
					continue
				}
				c, ok := rt.RegisteredTxEvent[chdr.TxId]
				if ok {
					if txsFltr.IsInvalid(j) {
						logger.Debugf("[consortium sdk retracer] transaction txid [%s] invalid", chdr.TxId)
						c <- 1
						rt.UnRegisterTxId(chdr.TxId)
						continue
					} else {
						logger.Debugf("[consortium sdk retracer] transaction txid [%s] ok", chdr.TxId)
						c <- 2
						rt.UnRegisterTxId(chdr.TxId)
					}
				}
			}

			if ccEvent, channelID, err := getChaincodeEvent(payload.Data, channelHeader); err != nil {
				err = fmt.Errorf("[consortium sdk retracer] get chaincode event failed: %s", err)
				logger.Error(err)
			} else if ccEvent != nil && ccEvent.Payload != nil {
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
	return
}

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
