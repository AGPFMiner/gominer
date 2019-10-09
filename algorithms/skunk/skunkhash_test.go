package skunk

import (
	"encoding/hex"
	"log"
	"strings"
	"testing"
	"time"
)

/*
0000000487FA4893F60F62928021D4E78B411B0D660F780C0235996C00001860000000002898DB0E67330354F0071E1F43721C07775A2DB18F0B4309B540A0EEBBBBB4255CC828B31B00FD6C00000000
, nonce: 000000000083B2CE
*/
func TestSkunkSingle(t *testing.T) {
	// header := "04000000EA9C403960428CC6BC886631703A218E461970FC97DBFF72347B0000000000008472ACD58EB351228A7667306C8E7EA10BC72F50CEF439F4B424CB13DBACA5933E55E15C8AEE001B7472C349"
	header := "04000000EA9C403960428CC6BC886631703A218E461970FC97DBFF72347B0000000000008472ACD58EB351228A7667306C8E7EA10BC72F50CEF439F4B424CB13DBACA5933E55E15C8AEE001BF581DC49"
	headerBytes, _ := hex.DecodeString(header)
	hash := Skunkhash(headerBytes)
	log.Printf("%02X\n", hash)
	expectedHash := "02FFE74CB834511E48035E6C4D06F125A98D2E399CDC28A1C73C000000000000"
	if expectedHash != strings.ToUpper(hex.EncodeToString(hash)) {
		t.Fatal("Wrong Hash.")
	}
}

func TestSkunkSingle2(t *testing.T) {
	header := "0000000487FA4893F60F62928021D4E78B411B0D660F780C0235996C00001860000000002898DB0E67330354F0071E1F43721C07775A2DB18F0B4309B540A0EEBBBBB4255CC828B31B00FD6CCEB28300"
	headerBytes, _ := hex.DecodeString(header)
	// headerBytesRev := stratum.RevHash(headerBytes)
	hash := Skunkhash(headerBytes)
	log.Printf("%02X\n", hash)
	expectedHash := "02FFE74CB834511E48035E6C4D06F125A98D2E399CDC28A1C73C000000000000"
	if expectedHash != strings.ToUpper(hex.EncodeToString(hash)) {
		t.Fatal("Wrong Hash.")
	}
}

func TestSkunkRegen(t *testing.T) {
	header := "0400000005B207392B60E8A0930069BC92A9A079ABE37D0F3411012EC8AE000000000000F573D2B4950EB9046D7C457BBA4DBE327276849B69BF9537309C52E1DDB44538FD3EC85C11D6001B00000000002450D1"
	headerBytes, _ := hex.DecodeString(header)
	hash := RegenHash(headerBytes)
	log.Printf("%02X\n", hash)
	expectedHash := "02FFE74CB834511E48035E6C4D06F125A98D2E399CDC28A1C73C000000000000"
	if expectedHash != strings.ToUpper(hex.EncodeToString(hash)) {
		t.Fatal("Wrong Hash.")
	}
}

func TestSkunkMidstate(t *testing.T) {
	header := "040000000aec9672c093c9290bb2b0193646c5f208c59f332d9410f6d1460000000000006ef86955d69b5a6d6439e34b27d419707c717136ec89ab0b649fc8b6c084de9724eff45bc897001b7ce44802"
	headerBytes, _ := hex.DecodeString(header)
	hash := SkunkMidstate(headerBytes)
	log.Printf("%02X\n", hash)
	expectedHash := "581A6E07C63738D3A2BCFEFD65E6663E30EDBD70AF4BD62AF4FF42E7F955C79A703414D6C7E00571F90F4BA51256A0B784D5BE8F9082845C80A1938A6516150EC084DE9724EFF45BC897001B7CE44802"
	if expectedHash != strings.ToUpper(hex.EncodeToString(hash)) {
		t.Fatal("Wrong Hash.")
	}
}

func TestGroestl100Times(t *testing.T) {
	header := "0000002092c6971a93ba3973f403daae40f8d53d4bc72a72ec0765c5118a3d000000000000ddf7262382434b8c2932438340b72bbba1ad89f21a4a92fe94ee41f8c07bc3fdb60b5a620e471b95dcdf01"
	headerBytes, _ := hex.DecodeString(header)
	var hash []byte
	start := time.Now()
	repeatTimes := 100000
	for i := 0; i < repeatTimes; i++ {
		hash = Skunkhash(headerBytes)
	}
	rate := float64(repeatTimes) / time.Since(start).Seconds() / 1000
	log.Printf("hashrate: %f KH/s\n", rate)
	log.Printf("%02X\n", hash)
	expectedHash := "5F5CB5924BD63DBAB46C4D2D8491F64F76BFA005C198080B864F210000000000"
	if expectedHash != strings.ToUpper(hex.EncodeToString(hash)) {
		t.Fatal("Wrong Hash.")
	}
}
