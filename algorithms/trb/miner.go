package trb

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"math/big"
	"time"

	"github.com/AGPFMiner/gominer/driver"
	"golang.org/x/crypto/ripemd160"

	"github.com/AGPFMiner/gominer/mining"

	solsha3 "github.com/miguelmota/go-solidity-sha3"
	"go.uber.org/zap"
)

// Miner actually mines :-)
type Miner struct {
	Driver            driver.Driver
	miningWorkChannel chan *driver.MiningWork

	PollDelay time.Duration
	logger    *zap.Logger
}

func (m *Miner) Init(args mining.MinerArgs) {
	m.logger = args.Logger
}

func hashFn(data []byte) []byte {
	hash := solsha3.SoliditySHA3(data)

	//Consider moving hasher constructor outside loop and replacing with hasher.Reset()
	hasher := ripemd160.New()

	hasher.Write(hash)
	hash1 := hasher.Sum(nil)
	n := sha256.Sum256(hash1)
	return n[:]
}

func RegenHash(input []byte) (output []byte) {
	if len(input) < 52+16 {
		res := sha256.Sum256([]byte{0}) //meaningless
		output = res[:]
		return
	}
	output = hashFn(input)
	log.Printf("fullheader: %02x, regen hash: %02x\n", input, output)

	return
}

func Hash2BigTarget(hash []byte) *big.Int {
	return new(big.Int).SetBytes(hash[:])
}

func DiffChecker(hash []byte, work driver.MiningWork) bool {
	bInt := Hash2BigTarget(hash)
	x := new(big.Int)
	jDiff := work.Job.(stratumJob).jDiff
	jDiffBig := big.NewInt(jDiff)
	x.Mod(bInt, jDiffBig)
	remainder := x.Int64()
	compareR := jDiff / int64(work.Difficulty)
	if remainder <= compareR {
		return true
	}
	return false
}

//Halt stops all miners
func (m *Miner) Halt() {
	// for _, v := range m.DeviceList {
	// 	v.halt()
	// }
}

const (
	addrBlockW00 = iota + 0x18
	addrBlockW1
	addrBlockW2
	addrBlockW3
	addrBlockW4
	addrBlockW5
	addrBlockW6
	addrBlockW7
	addrBlockW8
	addrBlockW9
	addrBlockWA
	addrBlockWB
	writeCtrl = byte(0x06)
	addrJobID = byte(0x30)
)
const (
	addrMidstate0 = iota + 0x40
	addrMidstate1
	addrMidstate2
	addrMidstate3
	addrMidstate4
	addrMidstate5
	addrMidstate6
	addrMidstate7
	addrMidstate8
	addrMidstate9
	addrMidstateA
	addrMidstateB
	addrMidstateC
	addrMidstateD
	addrMidstateE
	addrMidstateF
)

const nonceLen = 8

func ConstructHeaderPackets(header []byte, boardJobID uint8) (fpgaPacket []byte) {
	//32+20+4+4

	addrbase := addrBlockW00
	for cursor := 0; cursor < 15; cursor += 4 {
		var temp []byte
		temp = append(temp, writeCtrl, byte(addrbase))
		data := append(temp, header[cursor:cursor+4]...)
		fpgaPacket = append(fpgaPacket, data...)
		addrbase++
	}

	var temp []byte
	temp = append(temp, writeCtrl, addrJobID)
	data := append(temp, []byte{0x89, 0xab, 0xcd, byte(boardJobID)}...)
	fpgaPacket = append(fpgaPacket, data...)
	// spew.Dump(fpgaPacket)
	testMode, _ := hex.DecodeString("068100000000")
	fpgaPacket = append(fpgaPacket, testMode...)

	return
}

type MiningFuncs struct{}

func (mf *MiningFuncs) RegenHash(input []byte) (output []byte) {
	return RegenHash(input)
}

func (mf *MiningFuncs) DiffChecker(hash []byte, work driver.MiningWork) bool {
	return DiffChecker(hash, work)
}
func (mf *MiningFuncs) ConstructHeaderPackets(header []byte, boardJobID uint8) (fpgaPacket []byte) {
	return ConstructHeaderPackets(header, boardJobID)
}
