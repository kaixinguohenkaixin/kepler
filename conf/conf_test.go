package conf

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"testing"
)

const (
	ConfigFullPath = "src/github.com/vntchain/kepler/config/"
)

func setEnv() {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		panic("Not Set GOPATH")
	}
	err := os.Setenv("KEPLER_CFG_PATH", path.Join(gopath, ConfigFullPath))
	if err != nil {
		errMsg := fmt.Sprintf("Set KEPLER_CFG_PATH failed: %s", err)
		panic(errMsg)
	}
}

func TestInitConfig(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("InitConfig failed")
			t.Fail()
		}
	}()
	setEnv()
	InitConfig()
	return
}

func TestGetLogLevel(t *testing.T) {
	setEnv()
	InitConfig()
	l := GetLogLevel()
	assert.Equal(t, "debug", l, "GetLogLevel error: %s != debug", l)
	return
}
