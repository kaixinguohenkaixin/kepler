/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

VNT consortium chain depend on github.com/hyperledger/fabric project,
proto files and the generated pb.go files is copied from 
github.com/hyperledger/fabric/protos. However, to lower the dependence
of the fabric project, VNT protos just modified the go_package and 
java_package. Replace the github.com/hyperledger/fabric/protos to 
github.com/hyperledger/kepler/protos. 
*/

package peer

import (
	"encoding/json"
)

type strArgs struct {
	Function string
	Args     []string
}

func ToChaincodeArgs(args ...string) [][]byte {
	bargs := make([][]byte, len(args))
	for i, arg := range args {
		bargs[i] = []byte(arg)
	}
	return bargs
}

// UnmarshalJSON converts the string-based REST/JSON input to
// the []byte-based current ChaincodeInput structure.
func (c *ChaincodeInput) UnmarshalJSON(b []byte) error {
	sa := &strArgs{}
	err := json.Unmarshal(b, sa)
	if err != nil {
		return err
	}
	allArgs := sa.Args
	if sa.Function != "" {
		allArgs = append([]string{sa.Function}, sa.Args...)
	}
	c.Args = ToChaincodeArgs(allArgs...)
	return nil
}
