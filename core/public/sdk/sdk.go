package sdk

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/big"
	"strings"
	"time"

	"github.com/vntchain/kepler/core/public/sdk/common"
	"github.com/vntchain/go-vnt/accounts"
	"github.com/vntchain/go-vnt/accounts/abi"
	"github.com/vntchain/go-vnt/accounts/keystore"
	ethcom "github.com/vntchain/go-vnt/common"
	"github.com/vntchain/go-vnt/common/hexutil"
	"github.com/vntchain/go-vnt/core/types"
	"github.com/vntchain/go-vnt/rlp"
	"github.com/vntchain/go-vnt/rpc"
)

type EthSDK struct {
	HttpClient *rpc.Client
	Keystore   *keystore.KeyStore
	IpcClient  *rpc.Client
	WsClient   *rpc.Client
}

type EthConfig struct {
	HttpUrl string
	KeyDir  string
	WsPath  string
}

func NewKeyStore(keydir string) (*keystore.KeyStore, error) {

	ks := keystore.NewKeyStore(keydir, keystore.StandardScryptN, keystore.StandardScryptP)

	return ks, nil
}

func NewEthSDK(config EthConfig, ks *keystore.KeyStore) (*EthSDK, error) {
	var err error

	ethSDK := new(EthSDK)
	if config.HttpUrl != "" {
		ethSDK.HttpClient, err = rpc.Dial(config.HttpUrl)
		if err != nil {
			return nil, fmt.Errorf("[NewEthSDK]Failed to new HttpClient with %s", err.Error())
		}
	}

	ethSDK.Keystore = ks

	if config.WsPath != "" {
		ethSDK.WsClient, err = rpc.Dial(config.WsPath)
		if err != nil {
			return nil, fmt.Errorf("[NewEthSDK]Failed to new WsClient with %s", err.Error())
		}
	}
	return ethSDK, nil
}

func (es *EthSDK) GetAccounts() []accounts.Account {
	return es.Keystore.Accounts()
}

func (es *EthSDK) NewAccount(pass string) (string, error) {
	var result interface{}
	err := es.HttpClient.Call(&result, "personal_newAccount", pass)
	if err != nil {
		return "", fmt.Errorf("[NewAccount]Failed to new account with %s", err.Error())
	}

	return result.(string), nil
}

func (es *EthSDK) GetBalanceByHex(hex string) (*big.Int, error) {
	var balance hexutil.Big
	err := es.HttpClient.Call(&balance, "core_getBalance", hex, "latest")
	if err != nil {
		return big.NewInt(0), fmt.Errorf("[GetBalanceByHex]Failed to get %s's balance with %s", hex, err.Error())
	}
	fmt.Printf("Balance of %s is %#v\n", hex, balance.ToInt())
	return balance.ToInt(), nil
}

func (es *EthSDK) GetBlockNumber() (*big.Int, error) {
	var blockNumber hexutil.Big
	err := es.HttpClient.Call(&blockNumber, "core_blockNumber")
	if err != nil {
		return big.NewInt(0), fmt.Errorf("[GetBlockNumber]Failed to get blocknumber with err:%s", err.Error())
	}
	return blockNumber.ToInt(), nil
}

func (es *EthSDK) GetTransactionCount(account ethcom.Address) (uint64, error) {
	var result interface{}
	err := es.HttpClient.Call(&result, "core_getTransactionCount", account, "latest")
	if err != nil {
		return 0, fmt.Errorf("[GetTransactionCount]Failed to getTransactionCount with %s", err.Error())
	}

	count, err := hexutil.DecodeUint64(result.(string))
	return count, err
}

func (es *EthSDK) SendRawTransaction(data []byte, isWait bool) (ethcom.Hash, error) {
	var sendRes ethcom.Hash
	err := es.HttpClient.Call(&sendRes, "core_sendRawTransaction", ethcom.ToHex(data))
	if err != nil {
		return sendRes, fmt.Errorf("Failed to transact with %s", err.Error())
	}
	if isWait {
		for {
			trans, err := es.GetTransactionByHash(sendRes)
			if err != nil {
				return sendRes, err
			}
			if trans.(map[string]interface{})["blockNumber"] == nil {
				time.Sleep(3 * time.Second)
				continue
			} else {
				break
			}
		}
	}
	return sendRes, err
}

func (es *EthSDK) GetTransactionByHash(data ethcom.Hash) (interface{}, error) {
	var res interface{}
	err := es.HttpClient.Call(&res, "core_getTransactionByHash", data)
	if err != nil {
		return nil, fmt.Errorf("Failed to getTransactionByHash with %s", err.Error())
	}
	fmt.Printf("%s's Transaction:\n %#v \n", data.String(), res)
	return res, nil
}

func (es *EthSDK) GetTransactionReceipt(data ethcom.Hash) (map[string]interface{}, error) {
	var res map[string]interface{}
	err := es.HttpClient.Call(&res, "core_getTransactionReceipt", data)
	if err != nil {
		return nil, fmt.Errorf("Failed to getTransactionReceipt of %s with %s", data.String(), err.Error())
	}
	fmt.Printf("%s's TransactionReceipt:\n %#v \n", data.String(), res)
	return res, nil
}

func (es *EthSDK) GetBlockByNumber(blockNumber uint64) (string, error) {
	var block map[string]interface{}
	blockNumberStr := hexutil.EncodeUint64(blockNumber)
	err := es.HttpClient.Call(&block, "core_getBlockByNumber", blockNumberStr, false)
	if err != nil {
		return "", fmt.Errorf("Failed to getBlockByNumber of %d with err:(%s)", blockNumber, err.Error())
	}
	if _, ok := block["hash"]; !ok {
		return "", fmt.Errorf("Failed to getBlockByNumber of %d without blockhash", blockNumber)
	}
	return block["hash"].(string), nil
}

func (es *EthSDK) Deploy(
	acc *accounts.Account,
	chainId *big.Int,
	nonce uint64,
	amount *big.Int,
	gasLimit uint64,
	price *big.Int,
	contractPath string,
) (string, error) {
	codeBytes, err := ioutil.ReadFile(contractPath)
	if err != nil {
		return "", fmt.Errorf("Failed to read contract %s", contractPath)
	}
	codeStr := string(codeBytes)
	code := ethcom.Hex2Bytes(codeStr)

	tx := types.NewContractCreation(nonce, amount, gasLimit, price, code)

	signed, err := es.Keystore.SignTx(*acc, tx, chainId)
	data, err := rlp.EncodeToBytes(signed)
	if err != nil {
		return "", fmt.Errorf("Failed to EncodeToBytes with %s", err.Error())
	}
	sendRes, err := es.SendRawTransaction(data, true)
	if err != nil {
		return "", err
	}

	tranReceipt, err := es.GetTransactionReceipt(sendRes)
	if err != nil {
		return "", err
	}
	contractAddress := tranReceipt["contractAddress"]
	if contractAddress != nil {
		return contractAddress.(string), nil
	} else {
		return "", fmt.Errorf("Can't find contract address!")
	}
}

func (es *EthSDK) FormSignedTransaction(
	acc *accounts.Account,
	chainId *big.Int,
	nonce uint64,
	to ethcom.Address,
	amount *big.Int,
	gasLimit uint64,
	price *big.Int,
	input []byte,
) ([]byte, error) {
	tx := types.NewTransaction(nonce, to, amount, gasLimit, price, input)
	fmt.Printf("here is send tx: %#v\n", *tx)
	signed, err := es.Keystore.SignTx(*acc, tx, chainId)
	fmt.Printf("here is signed send tx: %#v\n", *signed)
	if err != nil {
		return nil, fmt.Errorf("Failed to signTx with %s", err.Error())
	}
	fmt.Printf("Transaction signer: %s, url: %s\n", acc.Address.Hex(), acc.URL.Path)
	data, err := rlp.EncodeToBytes(signed)
	if err != nil {
		return nil, fmt.Errorf("Failed to encode data with %s", err)
	}
	return data, nil
}

func (es *EthSDK) EthCall(
	blockNumber uint64,
	to ethcom.Address,
	data []byte,
	abi_instance abi.ABI,
	method string,
	result_struct interface{},
) (interface{}, error) {

	hexData := hexutil.Bytes(data)
	args := map[string]interface{}{
		"to":   to,
		"data": hexData,
	}
	var blockNumberStr string
	if blockNumber < 0 {
		blockNumberStr = "latest"
	} else {
		blockNumberStr = hexutil.EncodeUint64(blockNumber)
	}

	var result hexutil.Bytes
	err := es.HttpClient.Call(&result, "core_call", args, blockNumberStr)
	if err != nil {
		return nil, fmt.Errorf("Failed to core_call with %s", err.Error())
	}

	x := result.String()
	//fmt.Println("the result is ", x[2:])
	err = abi_instance.Unpack(result_struct, method, ethcom.Hex2Bytes(x[2:]))
	if err != nil {
		return nil, fmt.Errorf("Failed to upack with %s", err.Error())
	}

	return result_struct, nil
}

func (es *EthSDK) ListenAndPreProcessEvent(abiPath string, events []string, to []ethcom.Address, handledRecv chan map[string]interface{}) {
	result := make(map[string]interface{})
	abi_json, _ := ioutil.ReadFile(abiPath)
	abi_instance, _ := abi.JSON(strings.NewReader(string(abi_json)))
	eventsInfo, _ := common.GetNonAnnoymousEvent(abi_instance)
	logevent := make(chan types.Log, 1)
	go func() {
		for {
			eveArgs := common.FormSubscribeArgs(1, -1, to, [][]ethcom.Hash{})
			// es.LinstenEvent(eveArgs, logevent)
			es.ListenWsEvent(eveArgs, logevent)
			log := <-logevent
			topics := log.Topics
			data := log.Data
			eventInfo, ok := eventsInfo[topics[0].Hex()]
			if ok {
				fmt.Printf("Receive Event %s.\n", eventInfo.Name)
				result["_event_name_"] = eventInfo.Name
				if len(eventInfo.IndexedArgs)+1 == len(topics) {
					for i := 0; i < len(eventInfo.IndexedArgs); i++ {
						result[eventInfo.IndexedArgs[i]] = topics[i+1].Hex()
					}
				} else {
					fmt.Printf("Event Topic Length %d does not match indexedLenth %d\n", len(topics), len(eventInfo.IndexedArgs))
				}
				eventData, _ := abi_instance.Events[eventInfo.Name].Inputs.UnpackValues(data)
				nonIndexedLength := abi_instance.Events[eventInfo.Name].Inputs.LengthNonIndexed()
				if nonIndexedLength == len(eventData) {
					for i := 0; i < nonIndexedLength; i++ {
						result[eventInfo.NonIndexedArgs[i]] = eventData[i]
					}
				} else {
					fmt.Printf("Event Data Length %d does not match nonIndexedLength %d\n", len(eventData), nonIndexedLength)
				}
				//fmt.Printf("Event %s Detail:\n%#v\n", eventInfo.Name, result)
			} else {
				fmt.Printf("Receive Anonymous Event.\n")
				result["_event_name_"] = "_anonymony_"
				for i := 0; i < len(topics); i++ {
					result[fmt.Sprintf("topic_%d", i)] = topics[i].Hex()
				}
				result["_data_"] = ethcom.Bytes2Hex(data)
			}
			handledRecv <- result
		}
	}()
}

func (es *EthSDK) GetLogs(fromBlock string, toBlock string, address []ethcom.Address, topics [][]ethcom.Hash) ([]types.Log, error) {
	if fromBlock == "" {
		fromBlock = "0x0"
	}
	if toBlock == "" {
		toBlock = "latest"
	}
	var result []types.Log
	arg := map[string]interface{}{
		"fromBlock": fromBlock,
		"toBlock":   toBlock,
		"address":   address,
		"topics":    topics,
	}
	err := es.HttpClient.Call(&result, "core_getLogs", arg)
	return result, err
}

func (es *EthSDK) GetNonce(address ethcom.Address) (uint64, error) {
	var res interface{}
	err := es.HttpClient.Call(&res, "core_getTransactionCount", address.Hex(), "pending")
	if err != nil {
		return uint64(0), fmt.Errorf("Failed to core_getTransactionCount of %v with %s", address, err.Error())
	}
	return hexutil.DecodeUint64(res.(string))
}

func (es *EthSDK) EstimateGas(from, to ethcom.Address, data []byte, value, gasPrice *big.Int) (uint64, error) {
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
	err := es.HttpClient.Call(&hex, "core_estimateGas", arg)
	if err != nil {
		return 0, err
	}
	return uint64(hex), nil
}

func (es *EthSDK) ListenWsEvent(arg map[string]interface{}, result chan types.Log) {
	logs := make(chan types.Log, 128)

	ctx := context.TODO()
	cs, err := es.WsClient.VntSubscribe(ctx, logs, "logs", arg)
	if err != nil {
		fmt.Println("Failed to listenWsEvent with", err)
	}
	defer cs.Unsubscribe()
	for {
		logevent := <-logs
		result <- logevent
	}

}

func (es *EthSDK) RetraceLog(config common.RetraceConf, log chan types.Log) {
	retracer := common.InitRetracer(config)
	go retracer.Process(log)
	// go retracer.SyncToFile()
}
