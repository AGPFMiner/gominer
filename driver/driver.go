package driver

import (
	"github.com/dynm/gominer/clients"
	"github.com/dynm/gominer/types"
)

const (
	pullLow           = "00000000"
	pullHigh          = "ffffffff"
	nonceReadCtrlAddr = "060b"
	startMineCtrlAddr = "0608"
	initCnt0          = "0628"
	initCnt1          = "0629"
)

type MiningWork struct {
	Header     []byte
	Offset     int
	Target     []byte
	Difficulty float64
	// Job        stratumJob
	Job interface{}
}

type MiningFuncs interface {
	RegenHash(input []byte) (output []byte)
	DiffChecker(hash []byte, work MiningWork) bool
	ConstructHeaderPackets(header []byte, boardJobID uint8) (fpgaPacket []byte)
}

type Driver interface {
	Start()
	Stop()
	GetDriverStats() types.DriverStates
	RegisterMiningFuncs(string, MiningFuncs)
	Init(interface{})
	ProgramBitstream(bitstreamPath string) (err error)
	SetClient(clients.Client)
}
