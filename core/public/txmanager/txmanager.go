package txmanager

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
	"github.com/vntchain/go-vnt/accounts/keystore"
	pubcom "github.com/vntchain/go-vnt/common"
	"github.com/vntchain/kepler/core/consortium/endorser"
	cTxManager "github.com/vntchain/kepler/core/consortium/txmanager"
	comsdk "github.com/vntchain/kepler/core/public/sdk/common"
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
	CheckInterval	   = 3 * time.Second
	ConfirmedCount     = 10
	GasLimit		   = 2000000
	Price			   = 10
	RetryInterval	   = 1 * time.Second
)

type TxManager struct {
	keyDir  string
	chainId int
	nm      []*NodeManager
	cm      *ContractManager
}

func NewPublicTxManager(ks *keystore.KeyStore, keyDir string, chainId int, nodes map[string]interface{}, abiPath string, contractAddress string) (tm *TxManager, err error) {
	var cm *ContractManager
	cm, err = newContractManager(abiPath, contractAddress)
	if err != nil {
		err = fmt.Errorf("[public txManager] create contract manager failed: %s", err)
		logger.Error(err)
		return
	}
	tm = &TxManager{
		keyDir: keyDir,
		chainId: chainId,
		cm: cm,
		nm: make([]*NodeManager, 0),
	}

	for key, node := range nodes {
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

func (tm *TxManager) ListenEvent(userToCChan chan *pubevent.LogUserToC, EventRollback string, EventUserToC string, EventCToUser string, logUserToC string) {
	for _, n := range tm.nm {
		go n.ListenWsEvent(EventRollback, EventUserToC, EventCToUser, logUserToC, tm.cm, userToCChan)
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

func (tm *TxManager) SendPublicRollback(txidBytes []byte, chainIdInt int, CRollback string, abiPath string, passwd string) error {
	nm, err := tm.PickNodeManager()
	if err != nil {
		err = fmt.Errorf("[public txManager] pick node manger failed: %s", err)
		logger.Error(err)
		return err
	}
	ethSDK, err := nm.GetEthSDK()
	if err != nil {
		err = fmt.Errorf("[public txManager] get public SDK failed: %s", err)
		logger.Error(err)
		return err
	}

	txid := string(txidBytes)
	input, err := comsdk.PackMethodAndArgs(abiPath, CRollback, txid)
	if err != nil {
		err = fmt.Errorf("[public txManager] pack method and args failed: %s", err)
		logger.Error(err)
		return err
	}
	logger.Debugf("[public txManager] CRollback txid [%s] input: %#v", txid, string(input))

	account := ethSDK.GetAccounts()[0]
	err = ethSDK.Keystore.Unlock(account, passwd)
	if err != nil {
		err = fmt.Errorf("[public txManager] unlock account [%s] failed: %s", account.Address.String(), err)
		logger.Error(err)
		return err
	}

	nonce, err := ethSDK.GetNonce(account.Address)
	if err != nil {
		err = fmt.Errorf("[public txManager] get nounce of [%s] failed: %s", account.Address.String(), err)
		logger.Error(err)
		return err
	}
	chainId := big.NewInt(int64(chainIdInt))
	gasLimit := uint64(GasLimit)
	price := big.NewInt(Price)
	value := big.NewInt(0)
	to := pubcom.HexToAddress(abiPath)
	txBytes, err := ethSDK.FormSignedTransaction(&account, chainId, nonce, to, value, gasLimit, price, input)
	if err != nil {
		err = fmt.Errorf("[public txManager] form signed transaction failed: %s", err)
		logger.Error(err)
		return err
	}

	attempt := 0
	for ; attempt <= ConfirmedCount; attempt++ {
		_, err = ethSDK.SendRawTransaction(txBytes, true)
		if err == nil {
			return nil
		}
		err = fmt.Errorf("[public txManager] send transaction failed: %s", err)
		logger.Error(err)
		time.Sleep(RetryInterval)
	}
	if attempt == ConfirmedCount && err != nil {
		return err
	}

	return nil
}

func (tm *TxManager) HandleUserToCEvent(userToC *pubevent.LogUserToC, consortiumManager *cTxManager.TxManager, orgName string, channelName string, chaincodeName string, version string, queryCTransfer string, cTransfer string, queryCApprove string, cApprove string) {
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
	go consortiumManager.WaitUntilTransactionSuccess(func(data interface{}) bool {
		logCToUser := data.(*cevent.LogCToUser)
		logger.Debugf("[public txManager] callback received an agreed org %v", logCToUser)
		if logCToUser.TxId != txid {
			return false
		}
		if _, ok := agreedOrgs[logCToUser.AgreedOrg]; !ok {
			agreedOrgs[logCToUser.AgreedOrg] = logCToUser
		}
		logger.Debugf("agreed orgs length is %d", len(agreedOrgs))
		if len(agreedOrgs) >= viper.GetInt("consortium.agreedCount") {
			//发送cTransfer交易，并监听
			go consortiumManager.WaitUntilTransactionSuccess(func(cdata interface{}) bool {
				return true
			}, func(args ...interface{}) bool {
				if len(args) != 3 {
					return false
				}
				th := args[0].(*endorser.TransactionHandler)
				tm := args[1].(*cTxManager.TxManager)
				txid := args[2].(string)

				prop, txid, err := th.CreateProposal(channelName, chaincodeName, version, queryCTransfer, tm.GetCreator(), txid)
				if err != nil {
					logger.Errorf("create proposal in query err %v", err)
					return false
				}

				proposalResponse, err := th.ProcessProposal(tm.GetSigner().(*ecdsa.PrivateKey), prop)
				logger.Infof("callback queryCTransfer proposal response is result:%s err:%v", proposalResponse, err)
				if proposalResponse.Response != nil && err == nil {
					if proposalResponse.Response.Payload != nil {
						return true
					}

				}
				return false
			}, func(args ...interface{}) {
				//revert the eth in public chain
				_th := args[0].(*endorser.TransactionHandler)
				_tm := args[1].(*cTxManager.TxManager)
				tm.revert(_th, _tm, userToC.CTxId, userToC.Value)
			}, channelName, chaincodeName, version, cTransfer, cTransfer+txid, txid)

			return true
		}
		return false
	}, func(args ...interface{}) bool {
		if len(args) != 3 {
			return false
		}
		th := args[0].(*endorser.TransactionHandler)
		tm := args[1].(*cTxManager.TxManager)
		txid := args[2].(string)

		prop, txid, err := th.CreateProposal(channelName, chaincodeName, version, queryCApprove, tm.GetCreator(), txid)
		if err != nil {
			logger.Errorf("create proposal in query err %v", err)
			return false
		}

		proposalResponse, err := th.ProcessProposal(tm.GetSigner().(*ecdsa.PrivateKey), prop)
		logger.Infof("callback queryCApprove proposal response is result:%s err:%v", proposalResponse, err)
		if proposalResponse.Response != nil {
			var logCToUsers map[string]pubevent.KVAppendValue
			if proposalResponse.Response.Payload != nil {
				err = json.Unmarshal(proposalResponse.Response.Payload, &logCToUsers)
				if err != nil {
					logger.Errorf("the query result is err:%v", err)
					return false
				}
				if _, ok := logCToUsers[orgName]; ok {
					return true
				}
			}

		}
		return false

	}, func(args ...interface{}) {
		_th := args[0].(*endorser.TransactionHandler)
		_tm := args[1].(*cTxManager.TxManager)
		//revert the eth in public chain
		tm.revert(_th, _tm, userToC.CTxId, userToC.Value)
	}, channelName, chaincodeName, version, cApprove, txid, txid, fmt.Sprintf("%d", value), acctName, orgName)
}

func (tm *TxManager) revert(_th *endorser.TransactionHandler, _tm *cTxManager.TxManager, _txid []byte, _value *big.Int) {
	logger.Debugf("revert _txid is %s", pubcom.Bytes2Hex(_txid))
	//TODO 目前此处简单粗暴的直接查询联盟链节点transfer或revert是否成功，但是存在以下场景
	/*
		A、B、C三个节点同时发送CTransfer交易，但是在100s内都没有监听到该事件，同时发送revert请求，此时公链的revert执行
		但是联盟链接收到CTransfer交易成功，则此处不能简单的query，而是发送一个revert事件，只有监听到revert事件才能真正的
		revert成功。

		有一个简单的想法，可以学tcp的三次握手协议，但是改动太大，此处暂时先用query检查代替，后续得调整整体的框架
	*/

	txid := string(_txid)
	logger.Debugf("revert: the txid is %s", txid)
	channelName := viper.GetString("consortium.channelName")
	chaincodeName := viper.GetString("consortium.chaincodeName")
	version := viper.GetString("consortium.version")
	queryCTransfer := viper.GetString("consortium.queryCTransfer")
	queryCRevert := viper.GetString("consortium.queryCRevert")
	cRevert := viper.GetString("consortium.cRevert")

	prop, transId, err := _th.CreateProposal(channelName, chaincodeName, version, queryCTransfer, _tm.GetCreator(), txid)
	if err != nil {
		logger.Panicf("create proposal in query err %v", err)
	}

	proposalResponse, err := _th.ProcessProposal(_tm.GetSigner().(*ecdsa.PrivateKey), prop)
	logger.Infof("revert and try to queryCTranfer, proposal response is result:%s err:%v", proposalResponse, err)
	if proposalResponse.Response != nil {
		//logger.Infof("the proposal response payload is %s ",proposalResponse.Response.Payload)
		var logTransfered cevent.LogTransfered
		if proposalResponse.Response.Payload != nil {
			err = json.Unmarshal(proposalResponse.Response.Payload, &logTransfered)
			if err != nil {
				logger.Errorf("queryCTransfer result is err:%v", err)
			}
			logger.Warningf("the revert query txid exist and will return %#v", logTransfered)
			return
		}
	}

	// 查询该交易是否已经被回退，如果已经回退，则没必要再次回退
	prop, transId, err = _th.CreateProposal(channelName, chaincodeName, version, queryCRevert, _tm.GetCreator(), txid)
	if err != nil {
		logger.Panicf("create proposal in query err %v", err)
	}

	proposalResponse, err = _th.ProcessProposal(_tm.GetSigner().(*ecdsa.PrivateKey), prop)
	logger.Infof("revert and try to queryCRevert, proposal response is result:%s err:%v", proposalResponse, err)
	if proposalResponse.Response != nil {
		var orgName string
		if proposalResponse.Response.Payload != nil {
			err = json.Unmarshal(proposalResponse.Response.Payload, &orgName)
			if err != nil {
				logger.Errorf("queryCRevert result is err:%v", err)
			}
			logger.Warningf("cRevert query txid[%s] exist and will return %s", txid, orgName)
			return
		}
	}

	// 记录该交易的回退信息
	c := make(chan int, 1)
	cc := make(chan interface{}, 1)
	transId, _, err = _tm.SendTransaction(_th, c, cc, channelName, chaincodeName, version, cRevert, cRevert+txid, txid)
	if err != nil {
		logger.Errorf("Record cRevert failed, err: %s", err)
	}
	logger.Debugf("Record cRevert, revert txid: %s, transaction txid: %s", txid, transId)

	chainId := viper.GetInt("public.chainId")
	CRollback := viper.GetString("public.CRollback")
	abiPath := viper.GetString("public.contract.abi")
	passwd := viper.GetString("public.keypass")
	if err := tm.SendPublicRollback(_txid, chainId, CRollback, abiPath, passwd); err != nil {
		logger.Errorf("Revert RollbackToC failed: %s", err)
	}
}
func (tm *TxManager) SendRawTransaction(isWait bool, methodName string, args ...interface{}) (string, error) {
	n := tm.RandomNode()
	if n == nil {
		return "", fmt.Errorf("Failed to get NodeManager!")
	}

	abiPath := tm.cm.GetABIPath()
	data, err := pcommon.PackMethodAndArgs(abiPath, methodName, args...)
	if err != nil {
		logger.Errorf("Failed to PackMethodAndArgs with %s", err.Error())
		return "", err
	}
	pw := viper.GetString("public.keypass")

	return n.SendRawTransaction(pw, tm.chainId, tm.cm.GetContractAddress(), data, isWait)
}

func (tm *TxManager) RandomNode() *NodeManager {
	nmLen := len(tm.nm)
	if nmLen <= 0 {
		logger.Errorf("Invalid length of NodeManager: %d", nmLen)
		return nil
	}
	return tm.nm[rand.Intn(len(tm.nm))]
}
