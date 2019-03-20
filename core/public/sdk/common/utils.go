package common

import (
	"fmt"
	"github.com/vntchain/go-vnt/accounts/abi"
	"io/ioutil"
	"strings"
)

func PackMethodAndArgs(abiPath string, method string, args ...interface{}) ([]byte, error) {
	abiJson, err := ioutil.ReadFile(abiPath)
	if err != nil {
		err = fmt.Errorf("[public sdk] readfile [%s] failed: %s", abiPath, err)
		return nil, err
	}
	abiInstance, err := abi.JSON(strings.NewReader(string(abiJson)))
	if err != nil {
		err = fmt.Errorf("[public sdk] [%s] form abi json failed: %s", string(abiJson), err)
		return nil, err
	}
	res, err := abiInstance.Pack(method, args...)
	if err != nil {
		err = fmt.Errorf("[public sdk] abi pack [method: %s, args: %#v] failed: %s", method, args, err)
	}
	return res, err
}
