package endorser

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

func TestInitRetracer(t *testing.T) {
	rc := RetraceConf{
		PeerClient:      &PeerClient{},
		Channel:         "mychannel",
		RetraceInterval: time.Second,
		Creator:         []byte{},
		Signer:          "signer",
	}

	r, err := InitRetracer(rc)
	assert.Equal(t, nil, err, "InitRetracer error: %s", err)
	assert.NotEqual(t, nil, r, "InitRetracer return nil")
	t.Logf("Retracer: %#v", r)

	_, err = os.Stat(RetraceFileName)
	assert.Equal(t, nil, err, "file [%s] not generate: %s", RetraceFileName, err)

	time.Sleep(2 * time.Second)

	// 检查retracefile存在的情况下，是否可以成功初始化
	r, err = InitRetracer(rc)
	assert.Equal(t, nil, err, "InitRetracer error: %s", err)
	assert.NotEqual(t, nil, r, "InitRetracer return nil")
	t.Logf("Retracer: %#v", r)

	err = os.Remove(RetraceFileName)
	if err != nil {
		panic("remove retrace file failed, please remove it.")
	}
}

func TestSyncToFile(t *testing.T) {
	rc := RetraceConf{
		PeerClient:      &PeerClient{},
		Channel:         "mychannel",
		RetraceInterval: time.Second,
		Creator:         []byte{},
		Signer:          "signer",
	}

	r, err := InitRetracer(rc)
	assert.Equal(t, nil, err, "InitRetracer error: %s", err)

	err = r.syncToFile()
	assert.Equal(t, nil, err, "syncToFile error: %s", err)

	_, err = os.Stat(RetraceFileName)
	assert.Equal(t, nil, err, "file [%s] not generate: %s", RetraceFileName, err)

	err = os.Remove(RetraceFileName)
	if err != nil {
		panic("remove retrace file failed, please remove it.")
	}
}
