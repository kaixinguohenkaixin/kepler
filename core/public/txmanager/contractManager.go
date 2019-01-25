package txmanager

import (
	pubevent "github.com/vntchain/kepler/event/public"
	"github.com/spf13/viper"
	"github.com/vntchain/go-vnt/accounts/abi"
	ethcom "github.com/vntchain/go-vnt/common"
	"io/ioutil"
	"math/big"
	"strings"
)

type ContractManager struct {
	abiInstance     abi.ABI
	contractAddress ethcom.Address
	abiPath         string
}

func (cm *ContractManager) Init() error {
	cm.abiPath = viper.GetString("public.contract.abi")

	abiJson, err := ioutil.ReadFile(cm.abiPath)
	if err != nil {
		return err
	}

	cm.abiInstance, err = abi.JSON(strings.NewReader(string(abiJson)))
	if err != nil {
		return err
	}

	contractAddressStr := viper.GetString("public.contract.address")
	logger.Errorf("[public contractManager] contract address: %#v", contractAddressStr)
	cm.contractAddress = ethcom.HexToAddress(contractAddressStr)
	return nil
}

func (cm *ContractManager) GetContractAddress() ethcom.Address {
	return cm.contractAddress
}

func (cm *ContractManager) ReadUserToCEvent(data []byte, topics []ethcom.Hash, blockNumber uint64, blockHash ethcom.Hash, txHash ethcom.Hash, txIndex uint, removed bool) (*pubevent.LogUserToC, error) {
	// var userToCData pubevent.LogUserToCData

	type LogUserToC struct {
		Ac_address string
		Txid       string
	}
	var eventUserToC LogUserToC

	logUserToC := viper.GetString("public.LogUserToC")
	err := cm.abiInstance.Unpack(&eventUserToC, logUserToC, data)
	if err != nil {
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
	logger.Debugf("ReadUserToCEvent userToC is %#v", userToC)
	return userToC, nil
}

func (cm *ContractManager) GetABIPath() string {
	return cm.abiPath
}
