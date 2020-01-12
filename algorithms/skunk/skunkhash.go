package skunk

/*
#include <stdlib.h>
#include <skunk.h>
*/
import "C"
import (
	"unsafe"
	"github.com/AGPFMiner/gominer/clients/stratum"
)

//Skunkhash calculates skunk hash
func Skunkhash(input []byte) (output []byte) {
	in := C.CString(string(input))
	inp := unsafe.Pointer(in)
	defer C.free(inp)
	output = make([]byte, 32, 32)
	out := unsafe.Pointer(&output[0])
	C.skunk_hash(inp, out)
	return
}

//RegenHash calculates skunk hash
func RegenHash(input []byte) (output []byte) {
	if len(input)<88{
		output = []byte{0xff,0xff,0xff,0xff,
			0xff,0xff,0xff,0xff,
			0xff,0xff,0xff,0xff,
			0xff,0xff,0xff,0xff,
			0xff,0xff,0xff,0xff,
			0xff,0xff,0xff,0xff,
			0xff,0xff,0xff,0xff,
			0xff,0xff,0xff,0xff}
		return
	}
	header,nonce:=input[0:76],input[116:120]
	// headerRegen:=stratum.RevHash(append(header,nonce...))
	headerRegen := append(header,stratum.ReverseByteSlice(nonce)...)
	in := C.CString(string(headerRegen))
	inp := unsafe.Pointer(in)
	defer C.free(inp)
	output = make([]byte, 32, 32)
	out := unsafe.Pointer(&output[0])
	C.skunk_hash(inp, out)
	output = stratum.ReverseByteSlice(output)
	return
}

//SkunkMidstate calculates skunk midstate
func SkunkMidstate(input []byte) (output []byte) {
	in := C.CString(string(input))
	inp := unsafe.Pointer(in)
	defer C.free(inp)
	output = make([]byte, 80, 80)
	out := unsafe.Pointer(&output[0])
	C.skunk_midstate(inp, out)
	return
}
