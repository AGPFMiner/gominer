package stratum

//Some functions and types commonly used by stratum implementations are grouped here

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"
)

//HexStringToBytes converts a hex encoded string (but as go type interface{}) to a byteslice
// If v is no valid string or the string contains invalid characters, an error is returned
func HexStringToBytes(v interface{}) (result []byte, err error) {
	var ok bool
	var stringValue string
	if stringValue, ok = v.(string); !ok {
		return nil, errors.New("Not a valid string")
	}
	if result, err = hex.DecodeString(stringValue); err != nil {
		return nil, errors.New("Not a valid hexadecimal value")
	}
	return
}

//RevBytes reverse a slice.
func RevBytes(input []byte) (result []byte) {
	inlen := len(input)
	// if inlen == 0 {
	// 	result = input
	// 	return
	// }
	result = make([]byte, inlen)

	for i := range input {
		result[i] = input[inlen-1-i]
	}
	return
}

func RevHash(input []byte) (result []byte) {
	for i := 0; i < len(input)/4; i++ {
		result = append(result, RevBytes(input[i*4:i*4+4])...)
	}
	return
}

func ReverseByteSlice(input []byte) []byte {
	for left, right := 0, len(input)-1; left < right; left, right = left+1, right-1 {
		input[left], input[right] = input[right], input[left]
	}
	return input
}

//Hash256 Return a Sha256 Hash of given data
func Hash256(data []byte) []byte {
	hash := sha256.New()
	hash.Write(data)
	res := hash.Sum(nil)
	return res
}

//SHA256d Returns a string representation of doubled-hashed block header
func SHA256d(data []byte) []byte {
	hash := Hash256(Hash256(data))
	return hash
}

//ExtraNonce2 is the nonce modified by the miner
type ExtraNonce2 struct {
	Value uint64
	Size  uint
}

//Bytes is a bigendian representation of the extranonce2
func (en *ExtraNonce2) Bytes() (b []byte) {
	b = make([]byte, en.Size, en.Size)
	for i := uint(0); i < en.Size; i++ {
		b[(en.Size-1)-i] = byte(en.Value >> (i * 8))
	}
	return
}

//Increment increases the nonce with 1, an error is returned if the resulting is value is bigger than possible given the size
func (en *ExtraNonce2) Increment() (err error) {
	en.Value++
	//TODO: check if does not overflow compared to the allowed size
	return
}

func CheckDifficultyReal(blockhash, target []byte) (isSatisfied bool) {
	blockhashint := new(big.Int)
	blockhashint.SetBytes(blockhash)
	targetint := new(big.Int)
	targetint.SetBytes(target)
	isSatisfied = (blockhashint.Cmp(targetint) < 1)
	return
}
