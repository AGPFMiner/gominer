package veo

import (
	"encoding/hex"
	"log"
	"strings"
	"testing"
)

func TestVeoMidstate(t *testing.T) {
	header := "f86f71c0ae8eb91206c8ed3b98df8357db5eab795550622bfeb16100e75b15fe2bd9fd30000000009de57a6900000000000663000043fd"
	headerBytes, _ := hex.DecodeString(header)
	hash := Sha256Midstate12(headerBytes)
	log.Printf("%02X\n", hash)
	expectedHash := "505F2A794C31049B72DB9F18B6531ACBE74379F07C83D035E54F04B2587E0D11"
	if expectedHash != strings.ToUpper(hex.EncodeToString(hash)) {
		t.Fatal("Wrong Hash.")
	}
}
