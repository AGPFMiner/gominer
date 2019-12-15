package ckb

import (
	"encoding/hex"
	"log"
	"strings"
	"testing"
)

func TestEaglesongMidstate(t *testing.T) {
	header := "d5a74fba920ad0d35ec5726f26327547cbc82180e356e5ccf6cf2e6bd75f8a6600c904bd000000000000000000114026"
	headerBytes, _ := hex.DecodeString(header)
	hash := EaglesongMidstate(headerBytes)
	log.Printf("%02X\n", hash)
	header = "d5a74fba920ad0d35ec5726f26327547cbc82180e356e5ccf6cf2e6bd75f8a66"
	hash = EaglesongMidstate(headerBytes)
	log.Printf("%02X\n", hash)

	expectedHash := "D1744F377D07446AF254F75C18F213C020ED641588C171852F28AC6C9E25A730E277C1B69D2862A6600ADE078C44DA8C3924D2F21520B8329C6FE3D82BE6C6E9"
	if expectedHash != strings.ToUpper(hex.EncodeToString(hash)) {
		t.Fatal("Wrong Hash.")
	}
}

func TestEaglesongHash(t *testing.T) {
	header := "d5a74fba920ad0d35ec5726f26327547cbc82180e356e5ccf6cf2e6bd75f8a6600c904bd000000000000000000114026"
	headerBytes, _ := hex.DecodeString(header)
	hash := EaglesongHash(headerBytes)
	log.Printf("%02X\n", hash)
	expectedHash := "505F2A794C31049B72DB9F18B6531ACBE74379F07C83D035E54F04B2587E0D11"
	if expectedHash != strings.ToUpper(hex.EncodeToString(hash)) {
		t.Fatal("Wrong Hash.")
	}
}
