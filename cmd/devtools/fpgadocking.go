package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/dynm/gominer/algorithms/odocrypt"
	"github.com/dynm/gominer/clients"
	"github.com/dynm/gominer/types"
	"io"
	"log"
	"sync"
	"time"

	"github.com/jacobsa/go-serial/serial"
	// _ "net/http/pprof"
)

type SingleNonce struct {
	jobid uint8
	nonce [4]byte
}

type Docking struct {
	FPGADevice  string
	port        io.ReadWriter
	nonceChan   chan SingleNonce
	Client      clients.Client
	blockheader []byte
	mutex       sync.Mutex // protects following
}

func (d *Docking) initPort() {
	options := serial.OpenOptions{
		PortName:        d.FPGADevice,
		BaudRate:        115200,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 4,
		// InterCharacterTimeout: 100,
	}

	// Open the port.
	port, err := serial.Open(options)
	if err != nil {
		log.Panic("Port", err)
	}
	d.port = port
}

func (d *Docking) writeReadInst() {
	readNoncePacket, _ := hex.DecodeString(nonceReadCtrlAddr + pullLow + nonceReadCtrlAddr + pullHigh)
	for {
		select {
		case <-time.After(time.Second * 1):
			{
				d.mutex.Lock()
				log.Println("Write read nonce instr")
				d.port.Write(readNoncePacket)
				d.mutex.Unlock()
			}
		}
	}
}

func (d *Docking) readNonce() {
	log.Print("start read nonce")
	scanner := bufio.NewScanner(d.port)
	split := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		log.Printf("data: %02X\n", data)
		if len(data) < 9 {
			return 0, nil, nil
		}
		index := 0
		for index = 0; index < len(data)-9; index++ {
			if bytes.Equal(data[index:index+7], []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) {
				nonceNum := int(data[index+7])*16 + int(data[index+8])
				nonceLen := nonceNum * 9

				if len(data) < index+9+nonceLen { // waiting for more data
					return 0, nil, nil
				}

				if nonceNum > 0 {
					if int(data[index+9]) != 0 { // should always zero
						return 1, nil, nil
					}
				}
				return index + 9 + nonceLen, data[index+9 : index+9+nonceLen], nil
			}
		}

		if atEOF {
			return 0, nil, io.EOF
		}
		return 0, nil, nil
	}
	scanner.Split(split)
	for scanner.Scan() {
		nonces := scanner.Bytes()
		noncesLen := len(nonces)
		// log.Printf("nonces: %02X\n", nonces)
		for i := 0; i < noncesLen; i += 9 {
			var singleNonce SingleNonce
			singleNonce.jobid = uint8(nonces[i+1+3])
			copy(singleNonce.nonce[:], nonces[i+1+4:i+1+4+4])

			log.Print("Parsed Nonce", "SingleNonce", fmt.Sprintf("%02X", singleNonce.nonce), "JobID", singleNonce.jobid)

			d.nonceChan <- singleNonce
		}
	}
	log.Panic("Scanner exited.")

	return
}

var odoPool = &types.Pool{
	URL:  "stratum+tcp://dgb-odocrypt.f2pool.com:11115",
	User: "DEesW1UoEAUtM8mrwGHjfz1gdwPwqqRPzJ",
	Pass: "x",
	Algo: "odocrypt",
}

func (d *Docking) initPool() {
	d.Client = odocrypt.NewClient(odoPool)
	go d.Client.Start()
}

const (
	pullLow           = "00000000"
	pullHigh          = "ffffffff"
	nonceReadCtrlAddr = "060b"
	startMineCtrlAddr = "0608"
	initCnt0          = "0628"
	initCnt1          = "0629"
)

func (d *Docking) writeRepeatly() {
	jobid := uint8(0)
	for {
		_, _, header, _, _, _ := d.Client.GetHeaderForWork()
		// headerStr := "020E0020304762D062B9A693EE881F5CDE771BEA84268B1CF382583F01000000000000005B0B40CD2F1FAA2FF5D860B57BFEC804E4925A80431F456EE276A7C0871C12F5F621905D542E471A00000000"
		// header, _ := stratum.HexStringToBytes(headerStr)
		d.mutex.Lock()
		for i := 0; i < 32; i++ {
			if i < 4 {
				header[80+i] = 0x00
			} else {
				header[80+i] = 0xff
			}
		}
		d.blockheader = header
		d.mutex.Unlock()
		packet := odocrypt.ConstructHeaderPackets(header, jobid)
		jobid++
		select {
		case <-time.After(time.Second * 1):
			{
				d.mutex.Lock()
				log.Print("Write the same packet every 10s")
				n, err := d.port.Write(packet)
				for i, v := range packet {
					if i%6 == 0 {
						fmt.Printf("\n")
					}
					fmt.Printf("%02X", v)
				}
				log.Printf("written %d bytes, err: %v\n", n, err)
				// rd1 := make([]byte, 4)
				// rd2 := make([]byte, 4)
				// rand.Read(rd1)
				// rand.Read(rd2)
				// rd1Str := fmt.Sprintf("%02x", rd1)
				// rd2Str := fmt.Sprintf("%02x", rd2)
				// cnt, _ := hex.DecodeString(initCnt0 + rd1Str + initCnt1 + rd2Str)
				// log.Printf("Cnt :%02X\n", cnt)
				// d.port.Write(cnt)
				startMine, _ := hex.DecodeString(startMineCtrlAddr + pullLow + startMineCtrlAddr + pullHigh)
				d.port.Write(startMine)
				d.mutex.Unlock()
			}
		}
	}
}

func main() {
	docking := &Docking{FPGADevice: "/dev/ttyAMA0"}
	docking.initPort()
	docking.initPool()
	docking.nonceChan = make(chan SingleNonce, 100)

	go docking.writeReadInst()
	go docking.readNonce()
	// var blockheader []byte
	for {
		_, _, header, _, _, err := docking.Client.GetHeaderForWork()
		log.Printf("header:%02X, err: %v\n", header, err)
		if len(header) < 100 {
			continue
		}
		if header[0] == byte(0x02) {
			// blockheader = header
			break
		}
	}
	// log.Printf("header: %02X\n", blockheader)
	go docking.writeRepeatly()
	for {
		select {
		case nNonce := <-docking.nonceChan:
			docking.mutex.Lock()
			log.Printf("Header: %02X\nTarget: %02X\n", docking.blockheader[:80], docking.blockheader[80:])
			log.Printf("Got Nonce: %02X\n", nNonce.nonce[:])
			log.Printf("Full Header: %02X\n", append(docking.blockheader[:80], nNonce.nonce[:]...))
			docking.mutex.Unlock()
		}
	}
}
