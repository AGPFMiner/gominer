package veo

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

func TestHash2Int(t *testing.T) {
	hash, _ := hex.DecodeString("000000000ffff68A71E3473341DF5C45DA627BF80CE87FB4B3F2CD30D4B737D4")
	diff := veoHash2Int(hash)
	log.Printf("Diff: %d\n", diff)
}
