package xdag

import (
	"gominer/driver"
	"gominer/mining"
	"math/rand"
	"time"

	"go.uber.org/zap"
)

const (
	pullLow           = "00000000"
	pullHigh          = "ffffffff"
	nonceReadCtrlAddr = "060b"
	startMineCtrlAddr = "0608"
	initCnt0          = "0628"
	initCnt1          = "0629"
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
	return []byte{0x00, 0x00, 0x00, 0x00}
}

func DiffChecker(hash []byte, work driver.MiningWork) bool {
	return true
}

//Halt stops all miners
func (m *Miner) Halt() {
	// for _, v := range m.DeviceList {
	// 	v.halt()
	// }
}

const (
	writeCtrl = byte(0x06)
	addrJobID = byte(0x30)
)

func ConstructHeaderPackets(header []byte, boardJobID uint8) (fpgaPacket []byte) {

	var temp []byte
	temp = append(temp, header...)
	temp = append(temp, writeCtrl, addrJobID)
	data := append(temp, []byte{0x89, 0xab, 0xcd, byte(boardJobID)}...)
	fpgaPacket = append(fpgaPacket, data...)
	rd := make([]byte, 4)
	rand.Read(rd)
	rd1 := append([]byte{0x06, 0x28}, rd...)
	fpgaPacket = append(fpgaPacket, rd1...)
	rand.Read(rd)
	rd2 := append([]byte{0x06, 0x29}, rd...)
	fpgaPacket = append(fpgaPacket, rd2...)
	// spew.Dump(fpgaPacket)
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
