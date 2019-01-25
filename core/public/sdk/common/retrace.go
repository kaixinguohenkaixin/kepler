package common

import (
	"encoding/json"
	"fmt"
	"github.com/op/go-logging"
	"github.com/vntchain/go-vnt/common"
	"github.com/vntchain/go-vnt/core/types"
	"github.com/vntchain/go-vnt/rpc"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var logger = logging.MustGetLogger("core/public/sdk/common")

const RETRACE_FILE_NAME = "./log-retrace.json"

type RetraceConf struct {
	HttpClient      *rpc.Client
	ContractAddress common.Address
	SyncInterval    time.Duration
	RetraceInterval time.Duration
}

type retracer struct {
	httpClient      *rpc.Client
	syncStatus      map[string]uint64
	syncMu          sync.Mutex
	cache           chan *[]types.Log
	contractAddress common.Address
	syncInterval    time.Duration
	retraceInterval time.Duration
}

func InitRetracer(config RetraceConf) *retracer {
	logger.Debugf("Init Retracer")
	var tracer retracer
	tracer.httpClient = config.HttpClient
	tracer.contractAddress = config.ContractAddress
	tracer.syncInterval = config.SyncInterval
	tracer.retraceInterval = config.RetraceInterval
	tracer.syncStatus = make(map[string]uint64)

	tracer.cache = make(chan *[]types.Log, 32)
	_, err := os.Stat(RETRACE_FILE_NAME)
	if os.IsNotExist(err) {
		logger.Debugf(`Can't find serialization file "%s",init event checker for the first time!`, RETRACE_FILE_NAME)
		ioutil.WriteFile(RETRACE_FILE_NAME, []byte("{}"), 0644)
		tracer.syncStatus[config.ContractAddress.Hex()] = 0
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
	logger.Debugf("[public retracer] syncStatus: %#v", tracer.syncStatus)
	return &tracer
}

func (rt *retracer) Process(log chan types.Log) {
	for {
		time.Sleep(rt.retraceInterval)

		height, _ := rt.getHeight()
		logger.Debugf("[public getHeight]:%d", height)
		syncHeight := rt.getSyncHeight()
		logger.Debugf("[public syncHeight]:%d", syncHeight)

		if height <= syncHeight {
			continue
		}

		rt.cache <- rt.seekLogs(syncHeight+1, height+1)
		rt.dispatcher(log)
		rt.syncMu.Lock()
		rt.syncStatus[rt.contractAddress.Hex()] = height
		rt.syncMu.Unlock()
		err := rt.syncToFile()

		if err != nil {
			logger.Critical("save status to file err:(%s)", err)
		}

	}

}

func (rt *retracer) SyncToFile() {
	for {
		time.Sleep(rt.syncInterval)
		rt.syncToFile()
		logger.Debug("Sync Data persisted successful!")
	}
}

func (rt *retracer) syncToFile() error {
	rt.syncMu.Lock()
	snapShot, err := json.Marshal(rt.syncStatus)
	if len(rt.syncStatus) == 0 || len(snapShot) == 0 {
		snapShot = []byte("{}")
	}
	if err != nil {
		logger.Errorf("Serialization error when marshaling channelHandled: %s", err)
		return err
	}

	logger.Debugf("Start persist data to disk,content: %s", string(snapShot))
	f, err := ioutil.TempFile(filepath.Dir(RETRACE_FILE_NAME), fmt.Sprintf(".%s.tmp", filepath.Base(RETRACE_FILE_NAME)))
	if err != nil {
		logger.Errorf("Serialization creating temp file error: %s", err)
		return err
	}
	if _, err := f.Write(snapShot); err != nil {
		f.Close()
		os.Remove(f.Name())
		logger.Errorf("Serialization writing to temp file error: %s", err)
		return err
	}
	f.Close()
	err = os.Rename(f.Name(), RETRACE_FILE_NAME)
	if err != nil {
		logger.Errorf("Rename temp file to file error: %s", err)
		return err
	}
	rt.syncMu.Unlock()

	return nil
}

//seek logs in block num [from,to)
func (rt *retracer) seekLogs(from uint64, to uint64) *[]types.Log {
	result, _ := GetLogs(rt.httpClient, from, to, []common.Address{rt.contractAddress}, [][]common.Hash{})
	return &result
}

func (rt *retracer) getHeight() (uint64, error) {
	return GetHeight(rt.httpClient)
}

func (rt *retracer) dispatcher(log chan types.Log) {
	var single types.Log
	logPtr := <-rt.cache
	for _, single = range *logPtr {
		log <- single
		logger.Debugf("[public retrace] dispatch serve log: %#v", single)
	}
}

func (rt *retracer) getSyncHeight() uint64 {
	rt.syncMu.Lock()
	defer rt.syncMu.Unlock()
	return rt.syncStatus[rt.contractAddress.Hex()]
}
