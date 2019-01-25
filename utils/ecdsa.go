package utils

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/spf13/viper"
	"golang.org/x/crypto/sha3"
	"hash"
	"io/ioutil"
	"math/big"

	proto "github.com/golang/protobuf/proto"
)

var (
	// curveHalfOrders contains the precomputed curve group orders halved.
	// It is used to ensure that signature' S value is lower or equal to the
	// curve group order halved. We accept only low-S signatures.
	// They are precomputed for efficiency reasons.
	curveHalfOrders = map[elliptic.Curve]*big.Int{
		elliptic.P224(): new(big.Int).Rsh(elliptic.P224().Params().N, 1),
		elliptic.P256(): new(big.Int).Rsh(elliptic.P256().Params().N, 1),
		elliptic.P384(): new(big.Int).Rsh(elliptic.P384().Params().N, 1),
		elliptic.P521(): new(big.Int).Rsh(elliptic.P521().Params().N, 1),
	}
)

func GetCurveHalfOrdersAt(c elliptic.Curve) *big.Int {
	return big.NewInt(0).Set(curveHalfOrders[c])
}

type ECDSASignature struct {
	R, S *big.Int
}

func MarshalECDSASignature(r, s *big.Int) ([]byte, error) {
	return asn1.Marshal(ECDSASignature{r, s})
}

func UnmarshalECDSASignature(raw []byte) (*big.Int, *big.Int, error) {
	// Unmarshal
	sig := new(ECDSASignature)
	_, err := asn1.Unmarshal(raw, sig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed unmashalling signature [%s]", err)
	}

	// Validate sig
	if sig.R == nil {
		return nil, nil, errors.New("invalid signature, R must be different from nil")
	}
	if sig.S == nil {
		return nil, nil, errors.New("invalid signature, S must be different from nil")
	}

	if sig.R.Sign() != 1 {
		return nil, nil, errors.New("invalid signature, R must be larger than zero")
	}
	if sig.S.Sign() != 1 {
		return nil, nil, errors.New("invalid signature, S must be larger than zero")
	}

	return sig.R, sig.S, nil
}

// IsLow checks that s is a low-S
func IsLowS(k *ecdsa.PublicKey, s *big.Int) (bool, error) {
	halfOrder, ok := curveHalfOrders[k.Curve]
	if !ok {
		return false, fmt.Errorf("curve not recognized [%s]", k.Curve)
	}

	return s.Cmp(halfOrder) != 1, nil

}

func ToLowS(k *ecdsa.PublicKey, s *big.Int) (*big.Int, bool, error) {
	lowS, err := IsLowS(k, s)
	if err != nil {
		return nil, false, err
	}

	if !lowS {
		// Set s to N - s that will be then in the lower part of signature space
		// less or equal to half order
		s.Sub(k.Params().N, s)

		return s, true, nil
	}

	return s, false, nil
}

func GetHash() hash.Hash {
	hashFunctionName := viper.GetString("consortium.crypto.hash")
	security := viper.GetInt("consortium.crypto.security")
	if hashFunctionName == "sha256" || hashFunctionName == "sha2" {
		return sha256.New()
	} else if hashFunctionName == "sha512" {
		return sha512.New384()
	} else if hashFunctionName == "sha3" {
		if security == 256 {
			return sha3.New256()
		} else {
			return sha3.New384()
		}

	} else {
		return nil
	}
}

func Sign(k *ecdsa.PrivateKey, message []byte) (signature []byte, err error) {
	hf := GetHash()
	if hf == nil {
		return []byte{}, errors.New("hash function in config file err")
	}
	hf.Write(message)
	digest := hf.Sum(nil)
	r, s, err := ecdsa.Sign(rand.Reader, k, digest)
	if err != nil {
		return nil, err
	}

	s, _, err = ToLowS(&k.PublicKey, s)
	if err != nil {
		return nil, err
	}

	return MarshalECDSASignature(r, s)
}

func Verify(k *ecdsa.PublicKey, signature, message []byte) (valid bool, err error) {
	r, s, err := UnmarshalECDSASignature(signature)
	if err != nil {
		return false, fmt.Errorf("Failed unmashalling signature [%s]", err)
	}

	lowS, err := IsLowS(k, s)
	if err != nil {
		return false, err
	}

	if !lowS {
		return false, fmt.Errorf("Invalid S. Must be smaller than half the order [%s][%s].", s, GetCurveHalfOrdersAt(k.Curve))
	}

	hf := GetHash()
	if hf == nil {
		return false, errors.New("hash function in config file err")
	}
	hf.Write(message)
	digest := hf.Sum(nil)

	return ecdsa.Verify(k, digest, r, s), nil
}

// DERToPrivateKey unmarshals a der to private key
func DERToPrivateKey(der []byte) (key interface{}, err error) {

	if key, err = x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}

	if key, err = x509.ParsePKCS8PrivateKey(der); err == nil {
		switch key.(type) {
		case *ecdsa.PrivateKey:
			return
		default:
			return nil, errors.New("Found unknown private key type in PKCS#8 wrapping")
		}
	}

	if key, err = x509.ParseECPrivateKey(der); err == nil {
		return
	}

	return nil, errors.New("Invalid key type. The DER must contain an rsa.PrivareKey or ecdsa.PrivateKey")
}

// PEMtoPrivateKey unmarshals a pem to private key
func PEMtoPrivateKey(raw []byte, pwd []byte) (interface{}, error) {
	if len(raw) == 0 {
		return nil, errors.New("Invalid PEM. It must be different from nil.")
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("Failed decoding PEM. Block must be different from nil. [% x]", raw)
	}

	// TODO: derive from header the type of the key

	if x509.IsEncryptedPEMBlock(block) {
		if len(pwd) == 0 {
			return nil, errors.New("Encrypted Key. Need a password")
		}

		decrypted, err := x509.DecryptPEMBlock(block, pwd)
		if err != nil {
			return nil, fmt.Errorf("Failed PEM decryption [%s]", err)
		}

		key, err := DERToPrivateKey(decrypted)
		if err != nil {
			return nil, err
		}
		return key, err
	}

	cert, err := DERToPrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return cert, err
}

func ReadPrivateKey(filename string, pwd []byte) (interface{}, error) {
	raw, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	key, err := PEMtoPrivateKey(raw, pwd)
	return key, err
}

// This struct represents an Identity
// (with its MSP identifier) to be used
// to serialize it and deserialize it
type SerializedIdentity struct {
	// The identifier of the associated membership service provider
	Mspid string `protobuf:"bytes,1,opt,name=mspid" json:"mspid,omitempty"`
	// the Identity, serialized according to the rules of its MPS
	IdBytes []byte `protobuf:"bytes,2,opt,name=id_bytes,json=idBytes,proto3" json:"id_bytes,omitempty"`
}

func (m *SerializedIdentity) Reset()         { *m = SerializedIdentity{} }
func (m *SerializedIdentity) String() string { return proto.CompactTextString(m) }
func (*SerializedIdentity) ProtoMessage()    {}
func (m *SerializedIdentity) GetMspid() string {
	if m != nil {
		return m.Mspid
	}
	return ""
}

func (m *SerializedIdentity) GetIdBytes() []byte {
	if m != nil {
		return m.IdBytes
	}
	return nil
}

// Serialize returns a byte array representation of this identity
func SerializeCert(certPEM []byte) ([]byte, error) {
	// mspIdentityLogger.Infof("Serializing identity %s", id.id)

	pb, _ := pem.Decode(certPEM)
	if pb == nil {
		return nil, errors.New("decoding the cert pem err")
	}

	pemBytes := pem.EncodeToMemory(pb)
	if pemBytes == nil {
		return nil, errors.New("encoding of identity failed")
	}

	// We serialize identities by prepending the MSPID and appending the ASN.1 DER content of the cert
	sId := &SerializedIdentity{Mspid: viper.GetString("consortium.mspId"), IdBytes: pemBytes}
	idBytes, err := proto.Marshal(sId)
	if err != nil {
		return nil, err
	}

	return idBytes, nil
}

func ReadCertFile(filepath string) ([]byte, error) {
	raw, err := ioutil.ReadFile(filepath)
	return raw, err
}
