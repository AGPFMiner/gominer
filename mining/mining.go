package mining

import (
	"time"

	"github.com/dynm/gominer/clients"

	"go.uber.org/zap"
)

// "log"

//HashRateReport is sent from the mining routines for giving combined information as output
type HashRateReport struct {
	MinerID            int
	HashRate           [3]float64
	ShareCounter       uint64
	GoldenNonceCounter uint64
	Difficulty         float64
	NonceStats         map[int]uint64
}

//CreateEmptyBuffer calls CreateEmptyBuffer on the supplied context and logs and panics if an error occurred
// func CreateEmptyBuffer(ctx *cl.Context, flags cl.MemFlag, size int) (buffer *cl.MemObject) {
// 	buffer, err := ctx.CreateEmptyBuffer(flags, size)
// 	if err != nil {
// 		log.Panicln(err)
// 	}
// 	return
// }

type MinerArgs struct {
	FPGADevice           string
	Client               *clients.Client
	MuxNums              int
	PollDelay            time.Duration
	AutoProgramBit       bool
	NonceTraverseTimeout time.Duration
	Logger               *zap.Logger
}

//Miner declares the common 'Mine' method
type Miner interface {
	Init(MinerArgs)
	Halt()
}
