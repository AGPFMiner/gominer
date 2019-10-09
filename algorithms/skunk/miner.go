package skunk

import (
	"github.com/dynm/gominer/algorithms/generalstratum"
	"github.com/dynm/gominer/clients/stratum"
	"github.com/dynm/gominer/driver"
	"math/big"
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

func DiffChecker(hash []byte, work driver.MiningWork) bool {
	// hashInt, diff := veoHash2Int(hash), int(work.Difficulty)
	hashInt := new(big.Int)
	targetInt := new(big.Int)
	hashInt.SetBytes(hash)
	targetInt.SetBytes(work.Target)
	if hashInt.Cmp(targetInt) == -1 {
		return true
	} else {
		return false
	}
}

//Halt stops all miners
func (m *Miner) Halt() {
	// for _, v := range m.DeviceList {
	// 	v.halt()
	// }
}

const (
	addrMidstate0 = 0x40
	addrMidstatef = 0x4f
	addrBlockW01  = 0x19
	addrBlockW04  = 0x1c
	writeCtrl     = byte(0x06)
	addrJobID     = byte(0x30)
)

func ConstructHeaderPackets(header []byte, boardJobID uint8) (fpgaPacket []byte) {
	hash := SkunkMidstate(header)

	midstates := hash[:64]
	p0p1 := hash[64:]

	for addr := addrMidstate0; addr < addrMidstatef+1; addr++ {
		cursor := addr - addrMidstate0
		var temp []byte
		temp = append(temp, writeCtrl, byte(addr))
		revMidstate := stratum.ReverseByteSlice(midstates[cursor*4 : cursor*4+4])
		data := append(temp, revMidstate...)
		fpgaPacket = append(fpgaPacket, data...)
	}

	for addr := addrBlockW01; addr < addrBlockW04+1; addr++ {
		cursor := addr - addrBlockW01
		var temp []byte
		temp = append(temp, writeCtrl, byte(addr))
		revMidstate := stratum.ReverseByteSlice(p0p1[cursor*4 : cursor*4+4])
		data := append(temp, revMidstate...)
		fpgaPacket = append(fpgaPacket, data...)
	}

	var temp []byte
	// nonceCnt, _ := hex.DecodeString("062800000000062900000000")
	// temp = append(temp, nonceCnt...)
	temp = append(temp, writeCtrl, addrJobID)
	data := append(temp, []byte{0x89, 0xab, 0xcd, byte(boardJobID)}...)
	fpgaPacket = append(fpgaPacket, data...)
	junkChunk := []byte{0x06, 0x1c, 0x00, 0x00, 0x00, 0x00}
	fpgaPacket = append(fpgaPacket, junkChunk...)
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
