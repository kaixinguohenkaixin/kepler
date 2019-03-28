package endorser

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/vntchain/kepler/conf"
	"github.com/vntchain/kepler/utils"
	"os"
	"path"
	"testing"
)

const (
	CertPath       = "src/github.com/vntchain/kepler/config/consortium/cert"
	ConfigFullPath = "src/github.com/vntchain/kepler/config/"
)

func getEnv() (gopath string) {
	gopath = os.Getenv("GOPATH")
	if gopath == "" {
		panic("Not Set GOPATH")
	}
	return
}

func setEnv() {
	err := os.Setenv("KEPLER_CFG_PATH", path.Join(getEnv(), ConfigFullPath))
	if err != nil {
		errMsg := fmt.Sprintf("Set KEPLER_CFG_PATH failed: %s", err)
		panic(errMsg)
	}
}

func initConfig(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("InitConfig failed")
			t.Fail()
		}
	}()
	setEnv()
	conf.InitConfig()
	return
}

func getCreator(t *testing.T) ([]byte, error) {
	certPath := path.Join(getEnv(), CertPath)
	cert, err := utils.ReadCertFile(certPath)
	if err != nil {
		t.Errorf("read cert file from path [%s] failed: %s", certPath, err)
		return nil, err
	}
	creator, err := utils.SerializeCert(cert)
	if err != nil {
		t.Errorf("serialize cert failed: %s", err)
		return nil, err
	}
	return creator, nil
}

func TestTransactionHandler_CreateProposal(t *testing.T) {
	initConfig(t)
	creator, err := getCreator(t)
	assert.Equal(t, nil, err, "getCreator error: %s", err)

	th := &TransactionHandler{}
	prop, txid, err := th.CreateProposal(
		"mychannel",
		"kscc",
		"1.0",
		"addOrg",
		creator)
	assert.NotEqual(t, nil, prop, "CreateProposal error: proposal is nil")
	assert.NotEqual(t, "", txid, "CreateProposal error: txid is nil")
	assert.Equal(t, nil, err, "CreateProposal error: %s", err)
}

func TestTransactionHandler_CreateProposalWithTxGenerator(t *testing.T) {
	initConfig(t)
	creator, err := getCreator(t)
	assert.Equal(t, nil, err, "getCreator error: %s", err)

	th := &TransactionHandler{}
	generator := []byte("public chain txid")
	generator = append(generator, ([]byte("public tx public key ...."))...)
	prop, txid, err := th.CreateProposalWithTxGenerator(
		"mychannel",
		"kscc",
		"1.0",
		"addOrg",
		creator,
		generator)
	assert.NotEqual(t, nil, prop, "CreateProposal error: proposal is nil")
	assert.NotEqual(t, "", txid, "CreateProposal error: txid is nil")
	assert.Equal(t, nil, err, "CreateProposal error: %s", err)
}
