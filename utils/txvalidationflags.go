package utils

import (
	"github.com/vntchain/kepler/protos/peer"
)

// TxValidationFlags is array of transaction validation codes. It is used when committer validates block.
type TxValidationFlags []uint8

// NewTxValidationFlags Create new object-array of validation codes with target size. Default values: valid.
func NewTxValidationFlags(size int) TxValidationFlags {
	inst := make(TxValidationFlags, size)
	return inst
}

// SetFlag assigns validation code to specified transaction
func (obj TxValidationFlags) SetFlag(txIndex int, flag peer.TxValidationCode) {
	obj[txIndex] = uint8(flag)
}

// Flag returns validation code at specified transaction
func (obj TxValidationFlags) Flag(txIndex int) peer.TxValidationCode {
	return peer.TxValidationCode(obj[txIndex])
}

// IsValid checks if specified transaction is valid
func (obj TxValidationFlags) IsValid(txIndex int) bool {
	return obj.IsSetTo(txIndex, peer.TxValidationCode_VALID)
}

// IsInvalid checks if specified transaction is invalid
func (obj TxValidationFlags) IsInvalid(txIndex int) bool {
	return !obj.IsValid(txIndex)
}

// IsSetTo returns true if the specified transaction equals flag; false otherwise.
func (obj TxValidationFlags) IsSetTo(txIndex int, flag peer.TxValidationCode) bool {
	return obj.Flag(txIndex) == flag
}
