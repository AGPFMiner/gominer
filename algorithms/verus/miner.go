package verus

import (
	"github.com/dynm/gominer/clients/stratum"
	"github.com/dynm/gominer/driver"
	"time"

	"github.com/dynm/gominer/mining"

	"go.uber.org/zap"
)

//miningWork is sent to the mining routines and defines what ranges should be searched for a matching nonce
type miningWork struct {
	Header     []byte
	Offset     int
	Target     []byte
	Difficulty float64
	Job        stratumJob
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

func DiffChecker(hash []byte, work driver.MiningWork) bool {
	return true
}

func RegenHash(input []byte) (output []byte) {
	return []byte{0x00, 0x00, 0x00, 0x00}
}

//Halt stops all miners
func (m *Miner) Halt() {
	// for _, v := range m.DeviceList {
	// 	v.halt()
	// }
}

const (
	addrMidstate0 = 0x40
	addrMidstate7 = 0x47
	addrKey0x100  = 0x100
	addrKey0x9a0  = 0x9a0
	writeCtrl     = byte(0x06)
	addrJobID     = byte(0x30)
)

func ConstructHeaderPackets(header []byte, boardJobID uint8) (fpgaPacket []byte) {
	curBuf := genCurBuf(header)
	key := genKey(curBuf)

	for addr := addrMidstate0; addr < addrMidstate7+1; addr++ {
		cursor := addr - addrMidstate0
		var temp []byte
		temp = append(temp, writeCtrl, byte(addr))
		revMidstate := stratum.ReverseByteSlice(curBuf[cursor*4 : cursor*4+4])
		data := append(temp, revMidstate...)
		fpgaPacket = append(fpgaPacket, data...)
	}

	for addr := addrKey0x100; addr < addrKey0x9a0; addr++ {
		cursor := addr - addrKey0x100
		var temp []byte
		temp = append(temp, writeCtrl, byte(addr))
		revMidstate := stratum.ReverseByteSlice(key[cursor*4 : cursor*4+4])
		data := append(temp, revMidstate...)
		fpgaPacket = append(fpgaPacket, data...)
	}

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
