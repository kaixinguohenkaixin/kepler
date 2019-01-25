package common

import (
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/vntchain/go-vnt/accounts/abi"
	"github.com/vntchain/go-vnt/common"
	"github.com/vntchain/go-vnt/common/hexutil"
	"github.com/vntchain/go-vnt/core/types"
	"github.com/vntchain/go-vnt/rpc"
)

type EventInfo struct {
	Name           string
	IndexedArgs    []string
	NonIndexedArgs []string
}

func PackMethodAndArgs(abiPath string, method_name string, args ...interface{}) ([]byte, error) {
	abi_json, err := ioutil.ReadFile(abiPath)
	if err != nil {
		return []byte{}, err
	}
	abi_instance, err := abi.JSON(strings.NewReader(string(abi_json)))
	if err != nil {
		return []byte{}, fmt.Errorf("Failed to form abi.JSON with %s", err.Error())
	}
	return abi_instance.Pack(method_name, args...)
}

func FormSubscribeArgs(
	fromBlock int64,
	toBlock int64,
	addresses []common.Address,
	topics [][]common.Hash,
) map[string]interface{} {
	var toBlockStr string

	if toBlock >= 0 {
		toBlockStr = hexutil.EncodeBig(big.NewInt(toBlock))
	} else {
		toBlockStr = "latest"
	}

	return map[string]interface{}{
		"fromBlock": hexutil.EncodeBig(big.NewInt(fromBlock)),
		"toBlock":   toBlockStr,
		"address":   addresses,
		"topics":    topics,
	}
}

func GetAccountFile(path string, suffix string) string {
	filename := ""
	err := filepath.Walk(path, func(path string, f os.FileInfo, err error) error {
		if f == nil {
			return err
		}
		if f.IsDir() {
			return nil
		}
		name := f.Name()
		if strings.Contains(string(name), strings.ToLower(suffix)) {
			filename = name
		}
		return nil
	})
	if err != nil {
		logger.Errorf("filepath.Walk() returned %v\n", err)
	}
	return filename
}

func GetNonAnnoymousEvent(abiInstatnce abi.ABI) (map[string]EventInfo, error) {
	anonymous := 0
	result := make(map[string]EventInfo)

	for _, event := range abiInstatnce.Events {
		if event.Anonymous {
			anonymous += 1
		} else {
			var indexedArgs, nonIndexedArgs []string
			for _, input := range event.Inputs {
				if input.Indexed {
					indexedArgs = append(indexedArgs, input.Name)
				} else {
					nonIndexedArgs = append(nonIndexedArgs, input.Name)
				}
			}
			result[event.Id().Hex()] = EventInfo{
				Name:           event.Name,
				IndexedArgs:    indexedArgs,
				NonIndexedArgs: nonIndexedArgs,
			}
		}
	}
	logger.Debugf("There are %d events in total, include %d anonymous events.\n", len(abiInstatnce.Events), anonymous)
	return result, nil
}

func GetLogs(httpClient *rpc.Client, fromBlock uint64, toBlock uint64, address []common.Address, topics [][]common.Hash) ([]types.Log, error) {
	fromBlockHex := hexutil.EncodeUint64(fromBlock)
	toBlockHex := hexutil.EncodeUint64(toBlock)
	var result []types.Log
	arg := map[string]interface{}{
		"fromBlock": fromBlockHex,
		"toBlock":   toBlockHex,
		"address":   address,
		"topics":    topics,
	}
	logger.Debugf("public GetLogs start Args:%v, Address: %s", arg, address[0].Hex())
	err := httpClient.Call(&result, "core_getLogs", arg)
	logger.Debugf("public GetLogs result: %#v", result)
	return result, err
}

func GetHeight(httpClient *rpc.Client) (uint64, error) {
	var result interface{}
	//ctx := context.TODO()
	err := httpClient.Call(&result, "core_blockNumber", "latest", true)
	if err != nil {
		logger.Errorf("GetHeight error:%s", err.Error())
		return uint64(0), err
	}
	heightStr := result

	height, err := hexutil.DecodeUint64(heightStr.(string))
	return height, err
}
