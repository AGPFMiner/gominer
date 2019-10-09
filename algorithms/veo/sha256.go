package veo

/*
#include <stdlib.h>
#include <sha256.h>
*/
import "C"
import (
	"unsafe"
)

//Sha256Midstate12 calculates sha256midstate
func Sha256Midstate12(input []byte) (output []byte) {
	in := C.CString(string(input))
	inp := unsafe.Pointer(in)
	defer C.free(inp)
	output = make([]byte, 32, 32)
	out := unsafe.Pointer(&output[0])
	C.sha256_midstate12(inp, out)
	return
}
