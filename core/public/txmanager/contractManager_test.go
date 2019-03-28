package txmanager

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/vntchain/kepler/conf"
	"os"
	"path"
	"testing"
)

const (
	ConfigPath = "src/github.com/vntchain/kepler/config/"
)

func setEnv() {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		panic("Not Set GOPATH")
	}
	err := os.Setenv("KEPLER_CFG_PATH", path.Join(gopath, ConfigPath))
	if err != nil {
		errMsg := fmt.Sprintf("Set KEPLER_CFG_PATH failed: %s", err)
		panic(errMsg)
	}
}

func createContractManager() (cm *ContractManager, err error) {
	setEnv()
	conf.InitConfig()
	conf.ThePublicConf.Contract.AbiPath = path.Join(os.Getenv("KEPLER_CFG_PATH"), "public/abi/abi.json")
	cm, err = newContractManager()
	return
}

func TestNewContractManager(t *testing.T) {
	cm, err := createContractManager()
	assert.Equal(t, nil, err, "newContractManager error: %s", err)
	assert.NotEqual(t, nil, cm, "newContractManager return nil")
	t.Logf("ContranctManager: %#v\n", cm)
}