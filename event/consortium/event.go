package consortium

import (
	"encoding/json"
	"github.com/op/go-logging"
)

var logger = logging.MustGetLogger("consortium_event")

type LogUserToC struct {
	AccountF string
	Value    string
	AccountE string
}

type LogCToUser struct {
	TxId      string
	AgreedOrg string
	AccountF  string
	Value     string
}

type LogTransfered struct {
	TxId       string
	AgreedOrgs []string
	AccountF   string
	Value      string
}

func GetUserToC(payload []byte) *LogUserToC {
	var logUserToC LogUserToC
	if err := json.Unmarshal(payload, &logUserToC); err != nil {
		logger.Errorf("[consortium_event] unmarshal LogUserToC failed: %s", err)
		return nil
	}
	return &logUserToC
}
