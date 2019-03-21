package common

import (
	"encoding/json"
	"fmt"
	"github.com/op/go-logging"
	"github.com/vntchain/go-vnt/common"
	"github.com/vntchain/go-vnt/common/hexutil"
	"github.com/vntchain/go-vnt/core/types"
	"github.com/vntchain/go-vnt/rpc"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var logger = logging.MustGetLogger("public_sdk_retracer")

const (
	RetraceFileName = "./log-retrace.json"
	FilePerm        = 0644
)

type RetraceConf struct {
	HttpClient      *rpc.Client
	ContractAddress common.Address
	SyncInterval    time.Duration
	RetraceInterval time.Duration
}

type retracer struct {
	RetraceConf
	syncStatus map[string]uint64
	syncLock   sync.Mutex
}

func InitRetracer(config RetraceConf) (*retracer, error) {
	var tracer retracer
	tracer.RetraceConf = config
	tracer.syncStatus = make(map[string]uint64)

	_, err := os.Stat(RetraceFileName)
	if os.IsNotExist(err) {
		logger.Infof("[public sdk retracer] Can't find retrace file [%s], init event checker for the first time", RetraceFileName)
		err = ioutil.WriteFile(RetraceFileName, []byte("{}"), FilePerm)
		if err != nil {
			err = fmt.Errorf("[public sdk retracer] write retrace file [%s] failed: %s", RetraceFileName, err)
			logger.Error(err)
			return nil, err
		}
		tracer.syncStatus[config.ContractAddress.Hex()] = 0
	} else {
		serialization, err := ioutil.ReadFile(RetraceFileName)
		if err != nil {
			err = fmt.Errorf("[public sdk retracer] initialize retrace history from [%s] failed: %s", RetraceFileName, err)
			logger.Error(err)
			return nil, err
		}
		err = json.Unmarshal(serialization, &tracer.syncStatus)
		if err != nil {
			err = fmt.Errorf("[public sdk retracer] unmarshal retrace history failed: %s", err)
			logger.Error(err)
			return nil, err
		}
	}
	logger.Debugf("[public sdk retracer] retrace sync status: %#v", tracer.syncStatus)
	return &tracer, nil
}

func (rt *retracer) Process(log chan types.Log) {
	for {
		time.Sleep(rt.RetraceInterval)

		height, err := rt.getHeight()
		if err != nil {
			logger.Errorf("[public sdk retracer] get height failed: %s", err)
			continue
		}
		logger.Debugf("[public sdk retracer] block height: %d", height)
		syncHeight := rt.getSyncHeight()
		logger.Debugf("[public sdk retracer] sync height: %d", syncHeight)
		if height <= syncHeight {
			continue
		}

		result, err := rt.seekLogs(syncHeight+1, height+1)
		if err != nil {
			logger.Errorf("[public sdk retracer] seek logs failed: %s", err)
			continue
		}

		for _, single := range result {
			log <- single
			logger.Debugf("[public sdk retracer] dispatch log: %#v", single)
		}

		rt.syncLock.Lock()
		rt.syncStatus[rt.ContractAddress.Hex()] = height
		rt.syncLock.Unlock()

		err = rt.syncToFile()
		if err != nil {
			logger.Errorf("[public sdk retracer] save status to file [%s] failed: %s", RetraceFileName, err)
		}
	}
}

func (rt *retracer) getHeight() (height uint64, err error) {
	var result interface{}
	err = rt.HttpClient.Call(&result, "core_blockNumber", "latest", true)
	if err != nil {
		err = fmt.Errorf("[public sdk retracer] get blocknumber failed: %s", err)
		logger.Error(err)
		return
	}
	height, err = hexutil.DecodeUint64(result.(string))
	if err != nil {
		err = fmt.Errorf("[public sdk retracer] decode blocknumber [%s] failed: %s", result.(string), err)
		logger.Error(err)
	}
	return
}

func (rt *retracer) getSyncHeight() uint64 {
	rt.syncLock.Lock()
	defer rt.syncLock.Unlock()
	return rt.syncStatus[rt.ContractAddress.Hex()]
}

func (rt *retracer) seekLogs(from uint64, to uint64) (result []types.Log, err error) {
	fromBlockHex := hexutil.EncodeUint64(from)
	toBlockHex := hexutil.EncodeUint64(to)
	arg := map[string]interface{}{
		"fromBlock": fromBlockHex,
		"toBlock":   toBlockHex,
		"address":   []common.Address{rt.ContractAddress},
		"topics":    [][]common.Hash{},
	}

	result = make([]types.Log, 0)
	err = rt.HttpClient.Call(&result, "core_getLogs", arg)
	if err != nil {
		err = fmt.Errorf("[public sdk retracer] get logs failed: %s", err)
		logger.Error(err)
		return
	}
	logger.Debugf("[public sdk retracer] get logs result: %#v", result)
	return
}

func (rt *retracer) syncToFile() error {
	rt.syncLock.Lock()
	snapShot, err := json.Marshal(rt.syncStatus)
	if len(rt.syncStatus) == 0 || len(snapShot) == 0 {
		snapShot = []byte("{}")
	}
	rt.syncLock.Unlock()
	if err != nil {
		err = fmt.Errorf("[public sdk retracer] marshal sync status failed: %s", err)
		logger.Error(err)
		return err
	}

	f, err := ioutil.TempFile(filepath.Dir(RetraceFileName), fmt.Sprintf(".%s.tmp", filepath.Base(RetraceFileName)))
	if err != nil {
		err = fmt.Errorf("[public sdk retracer] create retrace temp file failed: %s", err)
		logger.Error(err)
		return err
	}
	defer f.Close()

	if _, err := f.Write(snapShot); err != nil {
		err = fmt.Errorf("[public sdk retracer] write to temp file failed: %s", err)
		logger.Error(err)
		err = os.Remove(f.Name())
		if err != nil {
			err = fmt.Errorf("[public sdk retracer] remove file failed: %s", err)
			logger.Error(err)
		}
		return err
	}
	err = os.Rename(f.Name(), RetraceFileName)
	if err != nil {
		err = fmt.Errorf("[public sdk retracer] rename temp file to retrace file failed: %s", err)
		logger.Error(err)
		return err
	}
	logger.Debugf("[public sdk retracer] persist data to retrace file: %s", string(snapShot))

	return nil
}
