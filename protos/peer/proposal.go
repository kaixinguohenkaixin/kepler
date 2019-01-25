/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0

VNT consortium chain depend on github.com/hyperledger/fabric project,
proto files and the generated pb.go files is copied from 
github.com/hyperledger/fabric/protos. However, to lower the dependence
of the fabric project, VNT protos just modified the go_package and 
java_package. Replace the github.com/hyperledger/fabric/protos to 
github.com/hyperledger/kepler/protos. 
*/

package peer

import (
	"fmt"

	"github.com/golang/protobuf/proto"
)

func (cpp *ChaincodeProposalPayload) StaticallyOpaqueFields() []string {
	return []string{"input"}
}

func (cpp *ChaincodeProposalPayload) StaticallyOpaqueFieldProto(name string) (proto.Message, error) {
	if name != cpp.StaticallyOpaqueFields()[0] {
		return nil, fmt.Errorf("not a marshaled field: %s", name)
	}
	return &ChaincodeInvocationSpec{}, nil
}
