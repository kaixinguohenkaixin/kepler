package sdk

import (
	"fmt"
	"github.com/op/go-logging"
	"github.com/vntchain/go-vnt/accounts"
	"github.com/vntchain/go-vnt/accounts/keystore"
	pubcom "github.com/vntchain/go-vnt/common"
	"github.com/vntchain/go-vnt/common/hexutil"
	"github.com/vntchain/go-vnt/core/types"
	"github.com/vntchain/go-vnt/rlp"
	"github.com/vntchain/go-vnt/rpc"
	"github.com/vntchain/kepler/core/public/sdk/common"
	"math/big"
	"time"
)

type EthSDK struct {
	Keystore   *keystore.KeyStore
	HttpClient *rpc.Client
	WsClient   *rpc.Client
}

type EthConfig struct {
	HttpUrl string
	KeyDir  string
	WsPath  string
}

const (
	CheckBlkInterval = 3 * time.Second
)

var logger = logging.MustGetLogger("public_sdk")

func NewKeyStore(keydir string) *keystore.KeyStore {
	return keystore.NewKeyStore(keydir, keystore.StandardScryptN, keystore.StandardScryptP)
}

func NewEthSDK(config EthConfig, ks *keystore.KeyStore) (ethSDK *EthSDK, err error) {
	var httpClient *rpc.Client
	var wsClient *rpc.Client

	if config.HttpUrl != "" {
		httpClient, err = rpc.Dial(config.HttpUrl)
		if err != nil {
			err = fmt.Errorf("[public sdk] create new Http Client failed: %s", err)
			logger.Error(err)
			return
		}
	} else {
		err = fmt.Errorf("[public sdk] Http Client not set in config file")
		logger.Error(err)
		return
	}

	if config.WsPath != "" {
		wsClient, err = rpc.Dial(config.WsPath)
		if err != nil {
			err = fmt.Errorf("[public sdk] create new WebSocket Client failed: %s", err)
			logger.Error(err)
			return
		}
	} else {
		err = fmt.Errorf("[public sdk] WebSocket Client not set in config file")
		logger.Error(err)
		return
	}

	ethSDK = &EthSDK{
		Keystore:   ks,
		HttpClient: httpClient,
		WsClient:   wsClient,
	}
	return
}

func (es *EthSDK) GetAccounts() []accounts.Account {
	return es.Keystore.Accounts()
}

func (es *EthSDK) GetBlockNumber() (*big.Int, error) {
	var blockNumber hexutil.Big
	err := es.HttpClient.Call(&blockNumber, "core_blockNumber")
	if err != nil {
		err = fmt.Errorf("[public sdk] get blocknumber failed: %s", err)
		logger.Error(err)
		return nil, err
	}
	return blockNumber.ToInt(), nil
}

func (es *EthSDK) SendRawTransaction(data []byte, isWait bool) (resp pubcom.Hash, err error) {
	err = es.HttpClient.Call(&resp, "core_sendRawTransaction", pubcom.ToHex(data))
	if err != nil {
		err = fmt.Errorf("[public sdk] send raw transaction failed: %s", err)
		logger.Error(err)
		return
	}
	if isWait {
		for {
			var tx interface{}
			tx, err = es.GetTransactionByHash(resp)
			if err != nil {
				err = fmt.Errorf("[public sdk] get transaction failed: %s", err)
				logger.Error(err)
				return
			}
			if tx.(map[string]interface{})["blockNumber"] == nil {
				time.Sleep(CheckBlkInterval)
				continue
			} else {
				break
			}
		}
	}
	return
}

func (es *EthSDK) GetTransactionByHash(data pubcom.Hash) (res interface{}, err error) {
	err = es.HttpClient.Call(&res, "core_getTransactionByHash", data)
	if err != nil {
		err = fmt.Errorf("[public sdk] get transaction by hash failed: %s", err)
		logger.Error(err)
		return
	}
	return
}

func (es *EthSDK) GetTransactionReceipt(data pubcom.Hash) (res map[string]interface{}, err error) {
	err = es.HttpClient.Call(&res, "core_getTransactionReceipt", data)
	if err != nil {
		err = fmt.Errorf("[public sdk] get transaction receipt of [%s] failed: %s", data.String(), err)
		logger.Error(err)
		return
	}
	return
}

func (es *EthSDK) GetBlockByNumber(blockNumber uint64) (string, error) {
	var block map[string]interface{}
	blockNumberStr := hexutil.EncodeUint64(blockNumber)
	err := es.HttpClient.Call(&block, "core_getBlockByNumber", blockNumberStr, false)
	if err != nil {
		err = fmt.Errorf("[public sdk] get block by number [%d] failed: %s", blockNumber, err)
		logger.Error(err)
		return "", err
	}
	if _, ok := block["hash"]; !ok {
		err = fmt.Errorf("[public sdk] get block by number [%d] without blockhash", blockNumber)
		logger.Error(err)
		return "", err
	}
	return block["hash"].(string), nil
}

func (es *EthSDK) FormSignedTransaction(
	acct *accounts.Account,
	chainId *big.Int,
	nonce uint64,
	to pubcom.Address,
	amount *big.Int,
	gasLimit uint64,
	price *big.Int,
	input []byte,
) ([]byte, error) {
	tx := types.NewTransaction(nonce, to, amount, gasLimit, price, input)
	logger.Debugf("[public sdk] new transaction: %#v", *tx)
	signedTx, err := es.Keystore.SignTx(*acct, tx, chainId)
	if err != nil {
		err = fmt.Errorf("[public sdk] sign transaction failed: %s", err)
		logger.Error(err)
		return nil, err
	}
	logger.Debugf("[public sdk] account [%s] signed transaction: %#v", acct.Address.Hex(), *signedTx)

	data, err := rlp.EncodeToBytes(signedTx)
	if err != nil {
		err = fmt.Errorf("[public sdk] encode transaction failed: %s", err)
		logger.Error(err)
		return nil, err
	}
	return data, nil
}

func (es *EthSDK) GetNonce(address pubcom.Address) (n uint64, err error) {
	var res interface{}
	err = es.HttpClient.Call(&res, "core_getTransactionCount", address.Hex(), "pending")
	if err != nil {
		err = fmt.Errorf("[public sdk] get nonce of [%s] failed: %s", address.String(), err)
		logger.Error(err)
		return
	}
	n, err = hexutil.DecodeUint64(res.(string))
	if err != nil {
		err = fmt.Errorf("[public sdk] decode [%s] failed: %s", res.(string), err)
		logger.Error(err)
	}
	return
}

func (es *EthSDK) EstimateGas(from, to pubcom.Address, data []byte, value, gasPrice *big.Int) (n uint64, err error) {
	var hex hexutil.Uint64
	arg := map[string]interface{}{
		"from": from,
		"to":   to,
		"data": hexutil.Bytes(data),
	}
	if value != nil {
		arg["value"] = (*hexutil.Big)(value)
	}
	if gasPrice != nil {
		arg["gasPrice"] = (*hexutil.Big)(gasPrice)
	}
	err = es.HttpClient.Call(&hex, "core_estimateGas", arg)
	if err != nil {
		err = fmt.Errorf("[public sdk] estimate gas failed: %s", err)
		logger.Error(err)
		return 0, err
	}

	n = uint64(hex)
	return
}

func (es *EthSDK) RetraceLog(config common.RetraceConf, log chan types.Log) error {
	retracer, err := common.InitRetracer(config)
	if err != nil {
		err = fmt.Errorf("[public sdk] init retracer failed: %s", err)
		logger.Error(err)
		return err
	}
	go retracer.Process(log)
	return nil
}
