package txmanager

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"github.com/op/go-logging"
	"github.com/vntchain/kepler/conf"
	"github.com/vntchain/kepler/core/consortium/endorser"
	cTxManager "github.com/vntchain/kepler/core/consortium/txmanager"
	"github.com/vntchain/kepler/core/public/sdk"
	pcommon "github.com/vntchain/kepler/core/public/sdk/common"
	cevent "github.com/vntchain/kepler/event/consortium"
	pubevent "github.com/vntchain/kepler/event/public"
	"math/big"
	"math/rand"
	"time"
)

var logger = logging.MustGetLogger("public_txmanager")

const (
	DeltaConfirmations = 3
	WaitTime           = 10 * time.Minute
	CheckInterval      = 3 * time.Second
	ConfirmedCount     = 10
	GasLimit           = 2000000
	Price              = 10
	RetryInterval      = 1 * time.Second
)

type TxManager struct {
	keyDir  string
	chainId int
	nm      []*NodeManager
	cm      *ContractManager
}

func NewPublicTxManager() (tm *TxManager, err error) {
	var cm *ContractManager
	cm, err = newContractManager()
	if err != nil {
		err = fmt.Errorf("[public txManager] create contract manager failed: %s", err)
		logger.Error(err)
		return
	}
	tm = &TxManager{
		keyDir:  conf.ThePublicConf.KeyPath,
		chainId: conf.ThePublicConf.ChainId,
		cm:      cm,
		nm:      make([]*NodeManager, 0),
	}

	ks := sdk.NewKeyStore(conf.ThePublicConf.KeyPath)
	for key, node := range conf.ThePublicConf.Nodes {
		nm, err := newNodeManager(key, node, ks)
		if err != nil {
			err = fmt.Errorf("[public txManager] create node [%s] manager failed: %s", key, err)
			logger.Error(err)
			return nil, err
		}
		tm.nm = append(tm.nm, nm)
	}
	return
}

func (tm *TxManager) ListenEvent(userToCChan chan *pubevent.LogUserToC) {
	for _, n := range tm.nm {
		go n.ListenWsEvent(tm.cm, userToCChan)
	}
}

func (tm *TxManager) PickNodeManager() (*NodeManager, error) {
	if len(tm.nm) == 0 {
		err := fmt.Errorf("[public txManager] node manager list is nil")
		logger.Error(err)
		return nil, err
	}
	i := rand.Intn(len(tm.nm))
	return tm.nm[i], nil
}

func (tm *TxManager) WaitUntilDeltaConfirmations(userToC *pubevent.LogUserToC) (err error) {
	nm, err := tm.PickNodeManager()
	if err != nil {
		err = fmt.Errorf("[public txManager] pick node manger failed: %s", err)
		logger.Error(err)
		return
	}

	ethSDK, err := nm.GetEthSDK()
	if err != nil {
		err = fmt.Errorf("[public txManager] get public SDK failed: %s", err)
		logger.Error(err)
		return
	}

	userToCBlockNumber := userToC.BlockNumber
	startTime := time.Now()
	for {
		var blockNumberBigInt *big.Int
		blockNumberBigInt, err = ethSDK.GetBlockNumber()
		if err != nil {
			endTime := time.Now()
			duration := endTime.Sub(startTime)
			err = fmt.Errorf("[public txManager] get block number failed: %s", err)
			logger.Error(err)
			if duration >= WaitTime {
				err = fmt.Errorf("[public txManager] wait [%v]", WaitTime)
				logger.Error(err)
				return
			}
			time.Sleep(CheckInterval)
			continue
		}

		blockNumber := blockNumberBigInt.Uint64()
		if blockNumber-userToCBlockNumber <= DeltaConfirmations {
			endTime := time.Now()
			duration := endTime.Sub(startTime)
			logger.Debugf("[public txManager] waiting for confirmed userToC, userToC block number [%d] and current blockNumber [%d]", userToCBlockNumber, blockNumber)
			if duration >= WaitTime {
				err = fmt.Errorf("[public txManager] wait [%v] and block number delta not meet %d", WaitTime, DeltaConfirmations)
				logger.Error(err)
				return
			}
			time.Sleep(CheckInterval)
			continue
		}
		break
	}
	return nil
}

func (tm *TxManager) SendPublicRollback(txidBytes []byte) (err error) {
	attempt := 0
	for ; attempt <= ConfirmedCount; attempt++ {
		_, err = tm.SendRawTransaction(true, conf.ThePublicConf.Contract.CRollback, string(txidBytes))
		if err == nil {
			logger.Debug("[Debug] [public txManager] send public rollback transaction succeed")
			return nil
		}
		err = fmt.Errorf("[public txManager] send public rollback transaction failed: %s", err)
		logger.Error(err)
		time.Sleep(RetryInterval)
	}
	if attempt == ConfirmedCount && err != nil {
		return
	}
	return
}

func (tm *TxManager) HandleUserToCEvent(userToC *pubevent.LogUserToC, consortiumManager *cTxManager.TxManager) {
	err := tm.WaitUntilDeltaConfirmations(userToC)
	if err != nil {
		err = fmt.Errorf("[public txManager] wait public UerToC event failed: %s", err)
		logger.Error(err)
		return
	}

	txid := string(userToC.CTxId)
	value := userToC.Value.Uint64()
	acctName := string(userToC.Ac_address)
	agreedOrgs := make(map[string]*cevent.LogCToUser)

	callback := func(data interface{}) bool {
		logCToUser := data.(*cevent.LogCToUser)
		if logCToUser.TxId != txid {
			return false
		}
		logger.Debugf("[public txManager] recieve consortium LogCToUser event: %#v", *logCToUser)

		if _, ok := agreedOrgs[logCToUser.AgreedOrg]; !ok {
			agreedOrgs[logCToUser.AgreedOrg] = logCToUser
		}
		if len(agreedOrgs) >= conf.TheConsortiumConf.AgreedCount {
			cCallback := func(cdata interface{}) bool {
				return true
			}
			cQuery := func(args ...interface{}) bool {
				if len(args) != 3 {
					return false
				}
				th := args[0].(*endorser.TransactionHandler)
				tm := args[1].(*cTxManager.TxManager)
				txid := args[2].(string)

				prop, txid, err := th.CreateProposal(conf.TheConsortiumConf.ChannelName, conf.TheConsortiumConf.Chaincode.Name, conf.TheConsortiumConf.Chaincode.Version, conf.TheConsortiumConf.Chaincode.QueryCTransfer, tm.GetCreator(), txid)
				if err != nil {
					err = fmt.Errorf("[public txManager] create consortium query proposal failed: %s", err)
					logger.Error(err)
					return false
				}
				proposalResponse, err := th.ProcessProposal(tm.GetSigner().(*ecdsa.PrivateKey), prop)
				logger.Debugf("[public txManager] consortium [queryCTransfer] proposal response: %s error: %#v", proposalResponse, err)
				if proposalResponse.Response != nil && err == nil {
					if proposalResponse.Response.Payload != nil {
						return true
					}
				}
				return false
			}
			cRevert := func(args ...interface{}) {
				cTh := args[0].(*endorser.TransactionHandler)
				cTm := args[1].(*cTxManager.TxManager)
				tm.revert(cTh, cTm, userToC.CTxId)
			}

			go consortiumManager.WaitUntilTransactionSuccess(
				cCallback,
				cQuery,
				cRevert,
				conf.TheConsortiumConf.Chaincode.CTransfer,
				conf.TheConsortiumConf.Chaincode.CTransfer+txid,
				txid)
			return true
		}
		return false
	}

	query := func(args ...interface{}) bool {
		if len(args) != 3 {
			return false
		}
		th := args[0].(*endorser.TransactionHandler)
		tm := args[1].(*cTxManager.TxManager)
		txid := args[2].(string)

		prop, txid, err := th.CreateProposal(
			conf.TheConsortiumConf.ChannelName,
			conf.TheConsortiumConf.Chaincode.Name,
			conf.TheConsortiumConf.Chaincode.Version,
			conf.TheConsortiumConf.Chaincode.QueryCApprove,
			tm.GetCreator(),
			txid)
		if err != nil {
			err = fmt.Errorf("[public txManager] create consortium query proposal failed: %s", err)
			logger.Error(err)
			return false
		}

		proposalResponse, err := th.ProcessProposal(tm.GetSigner().(*ecdsa.PrivateKey), prop)
		logger.Debugf("[public txManager] consortium [queryCApprove] proposal response: %s error: %#v", proposalResponse, err)
		if proposalResponse.Response != nil {
			var logCToUsers map[string]pubevent.KVAppendValue
			if proposalResponse.Response.Payload != nil {
				err = json.Unmarshal(proposalResponse.Response.Payload, &logCToUsers)
				if err != nil {
					err = fmt.Errorf("[public txManager] [queryCApprove] proposal response unmarshal failed: %s", err)
					logger.Error(err)
					return false
				}
				if _, ok := logCToUsers[conf.TheConsortiumConf.MspId]; ok {
					return true
				}
			}
		}
		return false
	}

	revert := func(args ...interface{}) {
		cTh := args[0].(*endorser.TransactionHandler)
		cTm := args[1].(*cTxManager.TxManager)
		tm.revert(cTh, cTm, userToC.CTxId)
	}

	go consortiumManager.WaitUntilTransactionSuccess(
		callback,
		query,
		revert,
		conf.TheConsortiumConf.Chaincode.CApprove,
		txid,
		txid,
		fmt.Sprintf("%d", value),
		acctName,
		conf.TheConsortiumConf.MspId)
}

func (tm *TxManager) revert(cTh *endorser.TransactionHandler, cTm *cTxManager.TxManager, txidBytes []byte) {
	logger.Debugf("[public txManager] revert txid: %s", string(txidBytes))
	//FIXME: 目前此处简单粗暴的直接查询联盟链节点transfer或revert是否成功，但是存在以下场景
	/*
		A、B、C三个节点同时发送CTransfer交易，但是在100s内都没有监听到该事件，同时发送revert请求，此时公链的revert执行
		但是联盟链接收到CTransfer交易成功，则此处不能简单的query，而是发送一个revert事件，只有监听到revert事件才能真正的
		revert成功。

		有一个简单的想法，可以学tcp的三次握手协议，但是改动太大，此处暂时先用query检查代替，后续得调整整体的框架
	*/

	txid := string(txidBytes)
	logger.Debugf("[public txManager] revert txid: %s", txid)

	// 查询该交易是否已经成功，如果成功则不回退
	prop, transId, err := cTh.CreateProposal(
		conf.TheConsortiumConf.ChannelName,
		conf.TheConsortiumConf.Chaincode.Name,
		conf.TheConsortiumConf.Chaincode.Version,
		conf.TheConsortiumConf.Chaincode.QueryCTransfer,
		cTm.GetCreator(),
		txid)
	if err != nil {
		err = fmt.Errorf("[public txManager] create consortium [queryCTransfer] proposal failed: %s", err)
		// FIXME: 有没有更好的办法？
		logger.Panic(err)
	}
	proposalResponse, err := cTh.ProcessProposal(cTm.GetSigner().(*ecdsa.PrivateKey), prop)
	logger.Debugf("[public txManager] consortium [queryCTransfer] proposal response: %s error: %#v", proposalResponse, err)
	if proposalResponse.Response != nil {
		var logTransfered cevent.LogTransfered
		if proposalResponse.Response.Payload != nil {
			err = json.Unmarshal(proposalResponse.Response.Payload, &logTransfered)
			if err != nil {
				err = fmt.Errorf("[public txManager] [queryCTransfer] proposal response unmarshal failed: %s", err)
				logger.Error(err)
			}
			logger.Warningf("[public txManager] revert [transfered] transaction exist: %#v", logTransfered)
			return
		}
	}

	// 查询该交易是否已经被回退，如果已经回退，则没必要再次回退
	prop, transId, err = cTh.CreateProposal(
		conf.TheConsortiumConf.ChannelName,
		conf.TheConsortiumConf.Chaincode.Name,
		conf.TheConsortiumConf.Chaincode.Version,
		conf.TheConsortiumConf.Chaincode.QueryCRevert,
		cTm.GetCreator(),
		txid)
	if err != nil {
		err = fmt.Errorf("[public txManager] create consortium [queryCRevert] proposal failed: %s", err)
		// FIXME: 有没有更好的办法？
		logger.Panic(err)
	}
	proposalResponse, err = cTh.ProcessProposal(cTm.GetSigner().(*ecdsa.PrivateKey), prop)
	logger.Debugf("[public txManager] consortium [queryCRevert] proposal response: %s error: %#v", proposalResponse, err)
	if proposalResponse.Response != nil {
		var orgName string
		if proposalResponse.Response.Payload != nil {
			err = json.Unmarshal(proposalResponse.Response.Payload, &orgName)
			if err != nil {
				err = fmt.Errorf("[public txManager] [queryCRevert] proposal response unmarshal failed: %s", err)
				logger.Error(err)
			}
			logger.Warningf("[public txManager] transaction [%s] revert by [%s]: %#v", txid, orgName)
			return
		}
	}

	// 记录该交易的回退信息
	c := make(chan int, 1)
	cc := make(chan interface{}, 1)
	transId, _, err = cTm.SendTransaction(
		cTh,
		c,
		cc,
		conf.TheConsortiumConf.ChannelName,
		conf.TheConsortiumConf.Chaincode.Name,
		conf.TheConsortiumConf.Chaincode.Version,
		conf.TheConsortiumConf.Chaincode.CRevert,
		conf.TheConsortiumConf.Chaincode.CRevert+txid,
		txid)
	if err != nil {
		err = fmt.Errorf("[public txManager] send [cRevert] transaction failed: %s", err)
		logger.Error(err)
		return
	}
	logger.Debugf("[public txManager] record cRevert [txid: %s, transaction txid: %s]", txid, transId)

	if err := tm.SendPublicRollback(txidBytes); err != nil {
		err = fmt.Errorf("[public txManager] send public [Rollback] transaction failed: %s", err)
		logger.Error(err)
	}
	return
}

func (tm *TxManager) SendRawTransaction(isWait bool, methodName string, args ...interface{}) (result string, err error) {
	logger.Debugf("[Debug] [public txManager] send raw transaction, method: %s, args: %#v", methodName, args)
	n, err := tm.PickNodeManager()
	if err != nil {
		err = fmt.Errorf("[public txManager] get NodeManager failed: %s", err)
		logger.Error(err)
		return
	}

	abiPath := tm.cm.GetABIPath()
	data, err := pcommon.PackMethodAndArgs(abiPath, methodName, args...)
	if err != nil {
		err = fmt.Errorf("[public txManager] pack method and args failed: %s", err)
		logger.Error(err)
		return
	}

	result, err = n.SendRawTransaction(conf.ThePublicConf.Password, tm.chainId, tm.cm.GetContractAddress(), data, isWait)
	if err != nil {
		err = fmt.Errorf("[public txManager] send public transaction failed: %s", err)
		logger.Error(err)
	}
	return
}
