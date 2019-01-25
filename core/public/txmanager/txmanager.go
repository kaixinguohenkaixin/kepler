package txmanager

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"github.com/vntchain/kepler/core/consortium/endorser"
	cTxManager "github.com/vntchain/kepler/core/consortium/txmanager"
	comsdk "github.com/vntchain/kepler/core/public/sdk/common"
	pcommon "github.com/vntchain/kepler/core/public/sdk/common"
	cevent "github.com/vntchain/kepler/event/consortium"
	pubevent "github.com/vntchain/kepler/event/public"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
	"github.com/vntchain/go-vnt/accounts/keystore"
	ethcom "github.com/vntchain/go-vnt/common"
	"math/big"
	"math/rand"
	"time"
)

var logger = logging.MustGetLogger("ptxmanager")

const (
	DeltaConfirmations = 3
	WaitTime           = 10 * 60 * time.Second
	ConfirmedCount     = 10
)

type TxManager struct {
	keyDir  string
	chainId int
	// account accounts.Account
	nm []*NodeManager // TODO 目前先按照一个处理

	cm *ContractManager
}

func (tm *TxManager) Init(ks *keystore.KeyStore) error {
	err := tm.readConfig(ks)
	if err != nil {
		return err
	}

	return nil
}

func (tm *TxManager) readConfig(ks *keystore.KeyStore) error {
	tm.keyDir = viper.GetString("public.keyDir")
	tm.chainId = viper.GetInt("public.chainId")
	nodes := viper.GetStringMap("public.nodes")

	tm.cm = &ContractManager{}
	err := tm.cm.Init()
	if err != nil {
		return err
	}

	for key, node := range nodes {
		tnm := &NodeManager{}

		err = tnm.Init(key, node, ks)
		if err != nil {
			return err
		}

		tm.nm = append(tm.nm, tnm)
	}

	return nil
}

func (tm *TxManager) ListenEvent(userToCChan chan *pubevent.LogUserToC) {
	for _, n := range tm.nm {
		go n.ListenWsEvent(tm.cm, userToCChan)
	}
}

func (tm *TxManager) PickNodeManager() (*NodeManager, error) {
	if len(tm.nm) == 0 {
		return nil, fmt.Errorf("node config is empty")
	}
	i := rand.Intn(len(tm.nm))
	return tm.nm[i], nil
}

func (tm *TxManager) WaitUntilDeltaConfirmations(userToC *pubevent.LogUserToC, consortiumManager *cTxManager.TxManager) (bool, uint64, error) {
	userToCBlockNumber := userToC.BlockNumber

	nm, err := tm.PickNodeManager()
	confirmedBlockNumber := uint64(0)

	if err != nil {
		return false, confirmedBlockNumber, err
	}

	ethSDK, err := nm.GetEthSDK()
	if err != nil {
		return false, confirmedBlockNumber, err
	}

	//总体等待时间不超过10分钟
	startTime := time.Now()

	for {
		blockNumberBigInt, err := ethSDK.GetBlockNumber()
		blockNumber := blockNumberBigInt.Uint64()
		confirmedBlockNumber = blockNumber
		if err != nil || blockNumber-userToCBlockNumber <= DeltaConfirmations {
			endTime := time.Now()
			duration := endTime.Sub(startTime)

			logger.Debugf("waiting for confirmed userToCBlockNumber:%d....now blockNumber:%d", userToCBlockNumber, blockNumber)
			if duration >= WaitTime {
				return false, confirmedBlockNumber, fmt.Errorf("wait 10 minutes and the blockNumber is not confirmed")
			}
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}

	//判断userToC.BlockHash是否在某个userToCBlockNumber的哈希一致
	blockHash, err := nm.ethSDK.GetBlockByNumber(userToCBlockNumber)
	if err != nil {
		return false, confirmedBlockNumber, err
	}

	confirmedBlockHash := ethcom.HexToHash(blockHash)
	if userToC.BlockHash != confirmedBlockHash {
		return false, confirmedBlockNumber, fmt.Errorf("the blockHash is not same")
	}
	logger.Debugf("blockHash:%s is confirmed ", blockHash)
	return true, confirmedBlockNumber, nil

}

func (tm *TxManager) HandleRollbackToC(_txid []byte, _value *big.Int) error {
	attempt := 0

	nm, err := tm.PickNodeManager()
	if err != nil {
		return err
	}

	ethSDK, err := nm.GetEthSDK()
	if err != nil {
		return err
	}

	account := ethSDK.GetAccounts()[0]
	nonce, err := ethSDK.GetNonce(account.Address)
	chainId := big.NewInt(int64(viper.GetInt("public.chainId")))
	gasLimit := uint64(2000000)
	price := big.NewInt(10)
	value := big.NewInt(0)
	txid := string(_txid)
	logger.Debugf("HandleRollbackToC Revert txid: %s", txid)

	CRollback := viper.GetString("public.CRollback")
	input, err := comsdk.PackMethodAndArgs(viper.GetString("public.contract.abi"), CRollback, txid)
	if err != nil {
		return err
	}
	logger.Debugf("HandleRollbackToC Input: %#v", string(input))

	to := ethcom.HexToAddress(viper.GetString("public.contract.address"))

	err = ethSDK.Keystore.Unlock(account, viper.GetString("public.keypass"))
	if err != nil {
		return err
	}

	txBytes, err := ethSDK.FormSignedTransaction(&account, chainId, nonce, to, value, gasLimit, price, input)
	if err != nil {
		return err
	}

	for {

		_, err := ethSDK.SendRawTransaction(txBytes, true)
		if err != nil {
			attempt += 1

		} else {
			return nil
		}

		if attempt <= ConfirmedCount {
			//继续重新尝试n次，若还不行，则放弃之
			time.Sleep(time.Second)
			continue
		} else {
			return err
		}

	}

	return nil
}

func (tm *TxManager) HandleUserToCEvent(userToC *pubevent.LogUserToC, consortiumManager *cTxManager.TxManager) {
	attempt := 0
	confirmed := false
	var err error

	for {
		if attempt >= ConfirmedCount {
			//继续重新尝试n次，若还不行，则放弃之
			break
		}

		tobeConfirmed, confirmedBlockNumber, err := tm.WaitUntilDeltaConfirmations(userToC, consortiumManager)
		confirmed = tobeConfirmed

		logger.Debugf("the confirmed is %v--->err:%v", confirmed, err)
		if confirmed == true && err == nil {
			break
		}

		if confirmedBlockNumber >= userToC.BlockNumber+DeltaConfirmations {
			break
		}
	}

	if err != nil || confirmed == false {
		return
	}

	//开始向联盟链发送交易，并监听，若发送失败，则回退该交易
	orgName := viper.GetString("consortium.mspId")
	//txid := userToC.TxHash.Hex()
	txid := string(userToC.CTxId)
	value := userToC.Value.Uint64()
	agreedOrgs := make(map[string]*cevent.LogCToUser)
	channelName := viper.GetString("consortium.channelName")
	chaincodeName := viper.GetString("consortium.chaincodeName")
	version := viper.GetString("consortium.version")
	queryCTransfer := viper.GetString("consortium.queryCTransfer")
	cTransfer := viper.GetString("consortium.cTransfer")
	queryCApprove := viper.GetString("consortium.queryCApprove")
	cApprove := viper.GetString("consortium.cApprove")

	accNameAccount := string(userToC.Ac_address)
	go consortiumManager.WaitUntilTransactionSuccess(func(data interface{}) bool {
		logCToUser := data.(*cevent.LogCToUser)
		logger.Debugf("callback received an agreed org %v", logCToUser)
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
	}, channelName, chaincodeName, version, cApprove, txid, txid, fmt.Sprintf("%d", value), accNameAccount, orgName)
}

func (tm *TxManager) revert(_th *endorser.TransactionHandler, _tm *cTxManager.TxManager, _txid []byte, _value *big.Int) {
	logger.Debugf("revert _txid is %s", ethcom.Bytes2Hex(_txid))
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

	if err := tm.HandleRollbackToC(_txid, _value); err != nil {
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

	return n.SendRawTransaction(tm.chainId, tm.cm.GetContractAddress(), data, isWait)
}

func (tm *TxManager) RandomNode() *NodeManager {
	nmLen := len(tm.nm)
	if nmLen <= 0 {
		logger.Errorf("Invalid length of NodeManager: %d", nmLen)
		return nil
	}
	return tm.nm[rand.Intn(len(tm.nm))]
}
