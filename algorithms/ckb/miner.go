package ckb

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
	"math/big"
	"time"

	"github.com/AGPFMiner/gominer/clients/stratum"
	"github.com/AGPFMiner/gominer/driver"

	"github.com/AGPFMiner/gominer/mining"

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

func RegenHash(input []byte) (output []byte) {
	if len(input) < 52 {
		res := sha256.Sum256([]byte{0}) //meaningless
		output = res[:]
		return
	}
	fullHeader := make([]byte, 44)
	copy(fullHeader, input[:44])
	fullHeader = append(fullHeader, stratum.RevBytes(input[48:52])...)
	output = EaglesongHash(fullHeader)
	log.Printf("fullheader: %02x, regen hash: %02x\n", fullHeader, output)

	return
}

func DiffChecker(hash []byte, work driver.MiningWork) bool {
	targetInt := big.NewInt(0).SetBytes(work.Target)
	hashInt := big.NewInt(0).SetBytes(hash)
	log.Printf("target: %02x, hash: %02x\n", targetInt.Bytes(), hashInt.Bytes())
	return hashInt.Cmp(targetInt) < 0
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

const nonceLen = 4

func ConstructHeaderPackets(header []byte, boardJobID uint8) (fpgaPacket []byte) {
	if len(header) != 48-nonceLen {
		log.Panicf("Unable to ConstructHeaderPackets, Header: %02X\n", header)
	}
	// [0 .. 32] [32 .. 48]
	// [0 1 2 3 4 5 6 7] [8 9 a b]
	// a 9 8
	headerBak := make([]byte, 0, 48)
	headerBak = append(headerBak, header...)

	tail := headerBak[32 : 32+16-4]

	addrbase := addrBlockW8
	for cursor := 0; cursor < 12; cursor += 4 {
		var temp []byte
		temp = append(temp, writeCtrl, byte(addrbase))
		data := append(temp, tail[cursor:cursor+4]...)
		fpgaPacket = append(fpgaPacket, data...)
		addrbase++
	}

	midstate := stratum.ReverseByteSlice(EaglesongMidstate(headerBak[:32]))
	addrbase = addrMidstate0
	for cursor := 0; cursor < 64; cursor += 4 {
		var temp []byte
		temp = append(temp, writeCtrl, byte(addrbase))
		data := append(temp, midstate[cursor:cursor+4]...)
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
