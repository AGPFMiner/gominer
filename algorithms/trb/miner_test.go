package trb

import (
	"encoding/hex"
	"log"
	"testing"
)

func TestRegenHash(t *testing.T) {
	header, _ := hex.DecodeString("294AD2F9740D395A805DC406D19661F4D3A49D5752A9B5D23A68445A51B9BE830000000000000000000000000000005800000000276994AD")
	hash := RegenHash(header)
	log.Printf("%02X\n", hash)
}
