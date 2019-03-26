package txmanager

import (
	"fmt"
	"github.com/vntchain/go-vnt/accounts"
	"github.com/vntchain/go-vnt/accounts/keystore"
	pubcom "github.com/vntchain/go-vnt/common"
	"github.com/vntchain/go-vnt/core/types"
	"github.com/vntchain/kepler/conf"
	"github.com/vntchain/kepler/core/public/sdk"
	sdkutils "github.com/vntchain/kepler/core/public/sdk/common"
	pubevent "github.com/vntchain/kepler/event/public"
	"math/big"
	"strings"
	"time"
)

const (
	SyncInterval    = 5 * time.Second
	RetraceInterval = 3 * time.Second
	TxAmount        = 0
	TxPrice         = 21
	TxGasLimit      = 1000000
)

type NodeManager struct {
	nodeName string
	ethSDK   *sdk.EthSDK
}

func newNodeManager(nodeName string, nodeConfig conf.NodeConf, ks *keystore.KeyStore) (nm *NodeManager, err error) {
	sdkConfig := sdk.EthConfig{HttpUrl: nodeConfig.HttpUrl, WsPath: nodeConfig.WsPath}
	ethSDK, err := sdk.NewEthSDK(sdkConfig, ks)
	if err != nil {
		err = fmt.Errorf("[public nodeManager] new public SDK failed: %s", err)
		logger.Error(err)
		return nil, err
	}

	nm = &NodeManager{
		nodeName: nodeName,
		ethSDK:   ethSDK,
	}
	return nm, nil
}

func (nm *NodeManager) GetEthSDK() (*sdk.EthSDK, error) {
	if nm.ethSDK == nil {
		err := fmt.Errorf("[public nodeManager] sdk is nil")
		logger.Error(err)
		return nil, err
	}

	return nm.ethSDK, nil
}

func (nm *NodeManager) GetTransactionReceipt(txHash string) (map[string]interface{}, error) {
	return nm.ethSDK.GetTransactionReceipt(pubcom.HexToHash(txHash))
}

func (nm *NodeManager) ListenWsEvent(contractManager *ContractManager, userToCChan chan *pubevent.LogUserToC) {
	logger.Debugf("[public nodeManager] start listening event")

	contractAddress := contractManager.GetContractAddress()
	log := make(chan types.Log, 1)
	err := nm.ethSDK.RetraceLog(sdkutils.RetraceConf{
		HttpClient:      nm.ethSDK.HttpClient,
		ContractAddress: contractAddress,
		SyncInterval:    SyncInterval,
		RetraceInterval: RetraceInterval,
	}, log)
	if err != nil {
		logger.Panicf("[public nodeManager] retrace log failed: %s", err)
	}

	for {
		l := <-log
		data := l.Data
		topics := l.Topics
		blockNumber := l.BlockNumber
		txHash := l.TxHash
		txIndex := l.TxIndex
		removed := l.Removed
		blockHash := l.BlockHash

		if len(topics) == 0 {
			continue
		}

		eventName := topics[1].Hex()
		logger.Debugf("[public nodeManager] read event [%s]", eventName)

		if strings.Contains(eventName, conf.ThePublicConf.Contract.EventUserToC) {
			userToC, err := contractManager.ReadUserToCEvent(data, topics, blockNumber, blockHash, txHash, txIndex, removed)
			if err != nil {
				logger.Errorf("[public nodeManager] ReadUserToCEvent failed: %s", err)
				continue
			}
			logger.Debugf("[public nodeManager] UserToC Event: %#v", userToC)
			userToCChan <- userToC
		} else if strings.Contains(eventName, conf.ThePublicConf.Contract.EventRollback) {
			continue
		} else if strings.Contains(eventName, conf.ThePublicConf.Contract.EventCToUser) {
			continue
		}
	}
}

func (nm *NodeManager) SendRawTransaction(passwd string, chainId int, contractAdd pubcom.Address, data []byte, isWait bool) (txid string, err error) {
	var nonce uint64
	var acct accounts.Account

	if len(nm.ethSDK.Keystore.Accounts()) == 0 {
		acct, err = nm.ethSDK.Keystore.NewAccount(passwd)
		if err != nil {
			err = fmt.Errorf("[public nodeManager] cannot find account, new account failed: %s", err)
			logger.Error(err)
			return
		}
	} else {
		acct = nm.ethSDK.Keystore.Accounts()[0]
	}

	if nonce, err = nm.ethSDK.GetNonce(acct.Address); err != nil {
		err = fmt.Errorf("[public nodeManager] getNonce of [%s] failed: %s", acct.Address.Hex(), err)
		logger.Error(err)
		return
	}

	chainIdBig := big.NewInt(int64(chainId))
	amount := big.NewInt(TxAmount)
	price := big.NewInt(TxPrice)
	// FIXME
	// gasLimit, _ := nm.ethSDK.EstimateGas(acc.Address, contractAdd, data, amount, price)
	gasLimit := uint64(TxGasLimit)

	if err = nm.ethSDK.Keystore.Unlock(acct, passwd); err != nil {
		err = fmt.Errorf("[public nodeManager] unlock account [%s] failed: %s", acct.Address.Hex(), err)
		logger.Error(err)
		return
	}
	signedData, err := nm.ethSDK.FormSignedTransaction(&acct, chainIdBig, nonce, contractAdd, amount, gasLimit, price, data)
	if err != nil {
		err = fmt.Errorf("[public nodeManager] form signed transaction failed: %s", err)
		logger.Error(err)
		return
	}
	result, err := nm.ethSDK.SendRawTransaction(signedData, isWait)
	if err != nil {
		err = fmt.Errorf("[public nodeManager] send raw transaction failed: %s", err)
		logger.Error(err)
		return
	}
	txid = result.Hex()
	return
}
