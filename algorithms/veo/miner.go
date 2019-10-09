package veo

import (
	"crypto/sha256"
	"gominer/driver"
	"log"
	"time"

	"gominer/mining"

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
	if len(input) < 56 {
		res := sha256.Sum256([]byte{0})
		output = res[:]
		return
	}
	shasum := sha256.New()
	shasum.Write(input[:48])
	shasum.Write(input[49:56])
	output = shasum.Sum(nil)
	return
}

func veoHash2Int(hash []byte) int {
	var h []uint32
	for b := range hash {
		h = append(h, uint32(hash[b]))
	}
	var x uint32
	var z uint32
	for i := 0; i < 31; i++ {
		if h[i] == 0 {
			x += 8
			continue
		} else if h[i] < 2 {
			x += 7
			z = h[i+1]
		} else if h[i] < 4 {
			x += 6
			z = (h[i+1] / 2) + ((h[i] % 2) * 128)
		} else if h[i] < 8 {
			x += 5
			z = (h[i+1] / 4) + ((h[i] % 4) * 64)
		} else if h[i] < 16 {
			x += 4
			z = (h[i+1] / 8) + ((h[i] % 8) * 32)
		} else if h[i] < 32 {
			x += 3
			z = (h[i+1] / 16) + ((h[i] % 16) * 16)
		} else if h[i] < 64 {
			x += 2
			z = (h[i+1] / 32) + ((h[i] % 32) * 8)
		} else if h[i] < 128 {
			x++
			z = (h[i+1] / 64) + ((h[i] % 64) * 4)
		} else {
			z = (h[i+1] / 128) + ((h[i] % 128) * 2)
		}
		break
	}
	var y [2]uint32
	y[0] = x
	y[1] = z
	return int((256 * y[0]) + y[1])
}

func DiffChecker(hash []byte, work driver.MiningWork) bool {
	hashInt, diff := veoHash2Int(hash), int(work.Difficulty)
	// log.Printf("DevDiff: %d, PoolDiff: %d\n", hashInt, diff)
	return hashInt >= diff
}

//Halt stops all miners
func (m *Miner) Halt() {
	// for _, v := range m.DeviceList {
	// 	v.halt()
	// }
}

const (
	addrBlockW00 = iota + 0x18
	addrBlockW01
	addrBlockW02
	addrBlockW03
	addrBlockW04
	addrBlockW05
	addrBlockW06
	addrBlockW07
	addrBlockW08
	addrBlockW09
	addrBlockW10
	addrBlockW11
	addrBlockW12
	addrBlockW13
	addrBlockW14
	addrBlockW15
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
)

const nonceLen = 7

func ConstructHeaderPackets(header []byte, boardJobID uint8) (fpgaPacket []byte) {
	if len(header) != 55-nonceLen {
		log.Panic("Unable to ConstructHeaderPackets")
	}
	header = append(header, []byte{0, 0, 0, 0, 0, 0, 0}...)
	header = append(header[:55], []byte{0}...)

	addrbase := addrBlockW15
	for cursor := 0; cursor < 48; cursor += 4 {
		var temp []byte
		temp = append(temp, writeCtrl, byte(addrbase))
		data := append(temp, header[cursor:cursor+4]...)
		fpgaPacket = append(fpgaPacket, data...)
		addrbase--
	}

	midstate := Sha256Midstate12(header)
	addrbase = addrMidstate7
	for cursor := 0; cursor < 32; cursor += 4 {
		var temp []byte
		temp = append(temp, writeCtrl, byte(addrbase))
		data := append(temp, midstate[cursor:cursor+4]...)
		fpgaPacket = append(fpgaPacket, data...)
		addrbase--
	}

	var temp []byte
	// nonceCnt, _ := hex.DecodeString("062800000000062900000000")
	// temp = append(temp, nonceCnt...)
	temp = append(temp, writeCtrl, addrJobID)
	data := append(temp, []byte{0x89, 0xab, 0xcd, byte(boardJobID)}...)
	fpgaPacket = append(fpgaPacket, data...)
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
