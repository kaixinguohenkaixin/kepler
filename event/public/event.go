package public

import (
	ethcom "github.com/vntchain/go-vnt/common"
	"math/big"
)

type LogUserToCData struct {
	Value *big.Int
}

type LogUserToC struct {
	Ac_address  []byte
	A_address   []byte
	CTxId       []byte
	Value       *big.Int
	BlockNumber uint64
	BlockHash   ethcom.Hash
	TxHash      ethcom.Hash
	TxIndex     uint
	Removed     bool
}

// Height represents the height of a transaction in blockchain
type Height struct {
	BlockNum uint64
	TxNum    uint64
}

// KVAppendValue captures a read/write (read/update/delete) operation performed during transaction simulation
type KVAppendValue struct {
	Version  *Height `protobuf:"bytes,1,opt,name=version" json:"version,omitempty"`
	IsDelete bool    `protobuf:"varint,2,opt,name=is_delete,json=isDelete" json:"is_delete,omitempty"`
	Value    []byte  `protobuf:"bytes,3,opt,name=value,proto3" json:"value,omitempty"`
}
