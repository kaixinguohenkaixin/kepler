package common

import (
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"testing"
)

const (
	ABIPath = "src/github.com/vntchain/kepler/config/public/abi/multi_sign.json"
)

func getGOPATH() string {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		panic("Not Set GOPATH")
	}
	return gopath
}

func TestPackMethodAndArgs(t *testing.T) {
	gopath := getGOPATH()
	abiPath := path.Join(gopath, ABIPath)

	method := "CRollback"
	orgAddr := "txid1"

	res, err := PackMethodAndArgs(abiPath, method, orgAddr)
	assert.Equal(t, nil, err, "PackMethodAndArgs error: %s", err)
	assert.NotEqual(t, nil, res, "PackMethodAndArgs return nil")
	t.Logf("Result: %s", string(res))
}
