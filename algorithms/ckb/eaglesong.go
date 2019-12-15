package ckb

/*
#include <stdlib.h>
#include <eaglesong.h>
*/
import "C"
import (
	"unsafe"
)

//EaglesongMidstate calculates eaglesong midstate
func EaglesongMidstate(input []byte) (output []byte) {
	in := C.CString(string(input))
	inp := unsafe.Pointer(in)
	defer C.free(inp)
	output = make([]byte, 64, 64)
	out := unsafe.Pointer(&output[0])
	C.EaglesongMidstate(inp, out)
	return
}

//EaglesongHash calculates eaglesong hash
func EaglesongHash(input []byte) (output []byte) {
	in := C.CString(string(input))
	inp := unsafe.Pointer(in)
	defer C.free(inp)
	output = make([]byte, 32, 32)
	out := unsafe.Pointer(&output[0])
	C.EaglesongHash(inp, out)
	return
}
