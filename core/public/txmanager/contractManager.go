package txmanager

import (
	"fmt"
	"github.com/vntchain/go-vnt/accounts/abi"
	pubcom "github.com/vntchain/go-vnt/common"
	pubevent "github.com/vntchain/kepler/event/public"
	"io/ioutil"
	"math/big"
	"strings"
)

type ContractManager struct {
	abiInstance     abi.ABI
	contractAddress pubcom.Address
	abiPath         string
}

func newContractManager(abiPath string, contractAddrStr string) (*ContractManager, error) {
	abiJson, err := ioutil.ReadFile(abiPath)
	if err != nil {
		err = fmt.Errorf("[public contractManager] read abi file failed: %s", err)
		logger.Error(err)
		return nil, err
	}
	abiInstance, err := abi.JSON(strings.NewReader(string(abiJson)))
	if err != nil {
		err = fmt.Errorf("[public contractManager] create abi json failed: %s", err)
		logger.Error(err)
		return nil, err
	}

	logger.Debugf("[public contractManager] contract address: %s", contractAddrStr)
	contractAddr := pubcom.HexToAddress(contractAddrStr)

	cm := ContractManager{
		abiInstance:     abiInstance,
		contractAddress: contractAddr,
		abiPath:         abiPath,
	}
	return &cm, nil
}

func (cm *ContractManager) GetContractAddress() pubcom.Address {
	return cm.contractAddress
}

func (cm *ContractManager) GetABIPath() string {
	return cm.abiPath
}

func (cm *ContractManager) ReadUserToCEvent(eventName string, data []byte, topics []pubcom.Hash, blockNumber uint64, blockHash pubcom.Hash, txHash pubcom.Hash, txIndex uint, removed bool) (*pubevent.LogUserToC, error) {
	type LogUserToC struct {
		Ac_address string
		Txid       string
	}
	var eventUserToC LogUserToC

	err := cm.abiInstance.Unpack(&eventUserToC, eventName, data)
	if err != nil {
		err = fmt.Errorf("[public contractManager] unpack LogUserToC event failed: %s", err)
		logger.Error(err)
		return nil, err
	}

	value := &big.Int{}
	value.SetBytes(topics[3].Bytes())
	userToC := &pubevent.LogUserToC{
		Ac_address:  []byte(eventUserToC.Ac_address),
		A_address:   topics[2].Bytes(),
		CTxId:       []byte(eventUserToC.Txid),
		Value:       value,
		BlockNumber: blockNumber,
		BlockHash:   blockHash,
		TxHash:      txHash,
		TxIndex:     txIndex,
		Removed:     removed,
	}
	logger.Debugf("[public contractManager] read LogUserToC event: %#v", userToC)
	return userToC, nil
}
