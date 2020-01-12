package odocrypt

import (
	"encoding/hex"
	"time"

	"github.com/AGPFMiner/gominer/algorithms/generalstratum"
	"github.com/AGPFMiner/gominer/clients/stratum"
	"github.com/AGPFMiner/gominer/driver"
	"github.com/AGPFMiner/gominer/mining"

	"go.uber.org/zap"
)

//miningWork is sent to the mining routines and defines what ranges should be searched for a matching nonce
type miningWork struct {
	Header     []byte
	Offset     int
	Target     []byte
	Difficulty float64
	Job        generalstratum.StratumJob
}

// Miner actually mines :-)
type Miner struct {
	Driver            driver.Driver
	miningWorkChannel chan *miningWork

	PollDelay time.Duration
	logger    *zap.Logger
}

func (m *Miner) Init(args mining.MinerArgs) {
	m.logger = args.Logger
}

//Halt stops all miners
func (m *Miner) Halt() {
	// m.Driver.Stop()
}

const (
	addrHeader00 = 0x18
	addrHeader19 = 0x2b
	addrTarget00 = 0x40
	addrTarget07 = 0x47
	addrTestMode = 0x81
	writeCtrl    = byte(0x06)
	addrJobID    = byte(0x30)
)

func DiffChecker(hash []byte, work driver.MiningWork) bool {
	return true
}

func ConstructHeaderPackets(header []byte, boardJobID uint8) (fpgaPacket []byte) {
	target := header[80:]
	for addr := addrHeader00; addr < addrHeader19+1; addr++ {
		cursor := addr - addrHeader00
		var temp []byte
		temp = append(temp, writeCtrl, byte(addr))
		revHeader := header[cursor*4 : cursor*4+4]
		// revHeader = []byte{0x00, 0x00, 0x00, 0x00}
		data := append(temp, revHeader...)
		fpgaPacket = append(fpgaPacket, data...)
	}

	revTarget := stratum.ReverseByteSlice(target)
	for addr := addrTarget00; addr < addrTarget07+1; addr++ {
		cursor := addr - addrTarget00
		var temp []byte
		temp = append(temp, writeCtrl, byte(addr))
		data := append(temp, revTarget[cursor*4:cursor*4+4]...)
		// log.Printf("%02X\n", data)
		fpgaPacket = append(fpgaPacket, data...)
	}

	testMode, _ := hex.DecodeString("068100000000")
	fpgaPacket = append(fpgaPacket, testMode...)

	var temp []byte

	// nonceCnt, _ := hex.DecodeString("062800000000062900000000")
	// temp = append(temp, nonceCnt...)
	temp = append(temp, writeCtrl, addrJobID)
	data := append(temp, []byte{0x89, 0xab, 0xcd, byte(boardJobID)}...)
	fpgaPacket = append(fpgaPacket, data...)
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
