package verus

import (
	"github.com/bmkessler/haraka"
)

const HEADER_LEN = 1487

var buf1 [64]byte
var buf2 [64]byte
var outBuffer [32]byte

func genCurBuf(header []byte) [32]byte {
	pos := 0
	curPos := 0
	len := HEADER_LEN
	curBuf := &buf1
	result := &buf2
	for pos < len {
		room := 32 - curPos
		if len-pos >= room {
			copy(curBuf[32+curPos:64], header[pos:])
			haraka.Haraka512(&outBuffer, curBuf)
			copy(result[:], outBuffer[:])
			result, curBuf = curBuf, result
			pos += room
			curPos = 0
		} else {
			copy(curBuf[32+curPos:64], header[pos:])
			curPos += len - pos
			pos = len
		}
	}
	copy(outBuffer[:], curBuf[:32])
	return outBuffer
}

func genKey(curBuf [32]byte) (key [8832]byte) {
	var inBuf [32]byte
	copy(inBuf[:], curBuf[:])
	keySeed := &inBuf

	for i := 0; i < 276; i++ {
		haraka.Haraka256(&outBuffer, keySeed)
		copy(key[i*32:i*32+32], outBuffer[:])
		copy(inBuf[:], outBuffer[:])
	}
	return
}

//VerusMidstate calculates skunk midstate
func VerusMidstate(input []byte) (output []byte) {
	// haraka.Haraka256()
	return
}
