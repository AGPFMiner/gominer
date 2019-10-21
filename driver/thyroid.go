package driver

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"sync/atomic"

	"github.com/dynm/gominer/boardman"
	"github.com/dynm/gominer/clients"
	"github.com/dynm/gominer/clients/stratum"
	"github.com/dynm/gominer/mining"
	"github.com/dynm/gominer/statistics"
	"github.com/dynm/gominer/types"

	"github.com/jacobsa/go-serial/serial"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type SingleNonce struct {
	jobid uint8
	nonce [8]byte
}

type Thyroid struct {
	shareCounter       uint64
	goldennonceCounter uint64
	wronghashCounter   uint64

	driverQuit        chan struct{}
	FPGADevice        string
	HashRateReports   chan *mining.HashRateReport
	MiningFuncs       map[string]MiningFuncs
	miningWorkChannel chan *MiningWork
	cleanJobChannel   chan bool
	muxNums           int
	blockTimeField    []byte

	Client                          clients.Client
	PollDelay, NonceTraverseTimeout time.Duration
	logger                          *zap.Logger
	port                            io.ReadWriteCloser
	nonceChan                       chan SingleNonce

	writeReadNonceMutex *sync.Mutex
	workCacheLock       *sync.RWMutex
	readNoncePacket     []byte
	boardID             int
	boardJobID          uint8
	chanSlot            map[int]chan bool
	workCache           map[uint8]MiningWork
	nonceStats          map[int]uint64
	prevEpochEnd        time.Time
	prevEpochNonceNum   uint64
	hr                  *statistics.HashRate
	stats               types.HardwareStats
	feedDog             chan bool
}

func NewThyroid(args mining.MinerArgs) (drv Driver) {
	drv = &Thyroid{}
	drv.Init(args)
	return drv
}

const (
	Kilo     = 1000
	Mega     = 1000 * Kilo
	Giga     = 1000 * Mega
	FourGiga = 4 * Giga
)

func (thy *Thyroid) GetDriverStats() (stats types.DriverStates) {
	stats.DriverName = "Thyroid"
	stats.Status = thy.stats
	// oneMin := float64(4096*thy.hr.RecentNSum(60)) / float64(60)
	// fiveMin := float64(4096*thy.hr.RecentNSum(300)) / float64(300)
	// oneHour := float64(4096*thy.hr.RecentNSum(3600)) / float64(3600)

	oneMin := thy.hr.RecentNSum(60)
	fiveMin := thy.hr.RecentNSum(300)
	oneHour := thy.hr.RecentNSum(3600)

	stats.NonceNum[0], stats.NonceNum[1], stats.NonceNum[2] = oneMin, fiveMin, oneHour
	stats.Hashrate[0], stats.Hashrate[1], stats.Hashrate[2] = oneMin*FourGiga/60, fiveMin*FourGiga/300, oneHour*FourGiga/3600
	stats.NonceStats = &thy.nonceStats
	stats.Algo = thy.Client.AlgoName()
	// stats.Temperature =
	if thy.stats != types.Programming {
		stats.Temperature, stats.Voltage, _ = getTempeVolt()
	} else {
		stats.Temperature, stats.Voltage = "-273.15", "25K"
	}
	// thy.HashRateReports <- &mining.HashRateReport{
	// 	HashRate:           [3]float64{oneMin, fiveMin, oneHour},
	// 	ShareCounter:       thy.shareCounter,
	// 	GoldenNonceCounter: thy.goldennonceCounter,
	// 	Difficulty:         0.0,
	// 	NonceStats:         thy.nonceStats,
	// }
	return
}

func (thy *Thyroid) RegisterMiningFuncs(algo string, mf MiningFuncs) {
	thy.MiningFuncs[algo] = mf
}

func (thy *Thyroid) SetClient(client clients.Client) {
	thy.Client = client
}

func (thy *Thyroid) Init(args interface{}) {
	thy.feedDog = make(chan bool, 1)
	thy.MiningFuncs = make(map[string]MiningFuncs)

	argsn := args.(mining.MinerArgs)
	thy.FPGADevice = argsn.FPGADevice
	thy.HashRateReports = argsn.HashRateReports
	thy.PollDelay = argsn.PollDelay
	thy.NonceTraverseTimeout = argsn.NonceTraverseTimeout
	thy.cleanJobChannel = make(chan bool)
	thy.muxNums = argsn.MuxNums
	thy.logger = argsn.Logger
	thy.readNoncePacket, _ = hex.DecodeString(nonceReadCtrlAddr + pullLow + nonceReadCtrlAddr + pullHigh)

	thy.shareCounter = 0
	thy.goldennonceCounter = 0
	thy.wronghashCounter = 0
	thy.boardID = 0
	thy.boardJobID = 0
	thy.writeReadNonceMutex = &sync.Mutex{}
	thy.workCacheLock = &sync.RWMutex{}
	thy.chanSlot = make(map[int]chan bool)
	thy.nonceChan = make(chan SingleNonce, 100)
	thy.workCache = make(map[uint8]MiningWork)
	thy.nonceStats = make(map[int]uint64)
	thy.prevEpochEnd = time.Now()
	thy.prevEpochNonceNum = 0
	thy.hr = &statistics.HashRate{}
}

const (
	rpiInterfacePath = "/usr/share/openocd/scripts/interface/raspberrypi-native.cfg"
	xc7CfgPath       = "/usr/share/openocd/scripts/cpld/xilinx-xc7.cfg"
	xdacPath         = "/usr/share/openocd/scripts/fpga/xilinx-xadc.cfg"
	adapterInit      = "adapter_khz 3000; init;"
)

func programBit(bitstreamPath string) error {

	pldLoadCmd := fmt.Sprintf("%s xc7_program xc7.tap; pld load 0 %s; exit", adapterInit, bitstreamPath)
	openocdCmd := fmt.Sprintf("sudo openocd -f %s -f %s -c '%s'", rpiInterfacePath, xc7CfgPath, pldLoadCmd)
	// openocdCmd := fmt.Sprintf("'adapter_khz 3000; init; xc7_program xc7.tap; pld load 0 %s; exit'", bitstreamPath)
	cmd := exec.Command("/bin/sh", "-c", openocdCmd)
	return cmd.Run()
}

func getTempeVolt() (temp, voltage string, err error) {

	readInfo := fmt.Sprintf("%s xadc_report xc7.tap; exit", adapterInit)
	openocdCmd := fmt.Sprintf("sudo openocd -f %s -f %s -f %s -c '%s'", rpiInterfacePath, xc7CfgPath, xdacPath, readInfo)
	cmd := exec.Command("/bin/sh", "-c", openocdCmd)
	out, err := cmd.CombinedOutput()
	r := bytes.NewBuffer(out)
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanLines)
	/*
		TEMP 79.05 C
		VCCINT 0.983 V
	*/
	for scanner.Scan() {
		t := scanner.Text()
		if strings.HasPrefix(t, "TEMP") {
			temp = strings.Split(t, " ")[1]
		}
		if strings.HasPrefix(t, "VCCINT") {
			voltage = strings.Split(t, " ")[1]
		}
	}
	return
}

const (
	BitStreamDir = "/opt/scripta/bitstreams"
)

func (thy *Thyroid) ProgramBitstream(bitstreamFilePath string) (err error) {
	var bitstreamName string

	if bitstreamFilePath == "" {
		algo := thy.Client.AlgoName()
		switch algo {
		case "odocrypt":
			gotBlockTs := false
			for i := 0; i < 10; i++ {
				if len(thy.blockTimeField) == 4 {
					//[]byte{0xE3,0x17,0x96,0x5D}
					ts := binary.LittleEndian.Uint32(thy.blockTimeField)
					ts = ts - ts%(10*24*60*60)
					thy.logger.Info("driver", zap.String("timestamp source", "blocktime"),
						zap.Uint32("timestamp", ts))
					bitstreamName = fmt.Sprintf("%s-%d.bit", algo, ts)
					gotBlockTs = true
					break
				}
				time.Sleep(time.Second * 1)
			}
			if !gotBlockTs {
				ts := time.Now().Unix()
				ts = ts - ts%(10*24*60*60)
				thy.logger.Warn("driver", zap.String("timestamp source", "miner's local time"),
					zap.Int64("timestamp", ts))
				bitstreamName = fmt.Sprintf("%s-%d.bit", algo, ts)
			}
		default:
			bitstreamName = fmt.Sprintf("%s.bit", algo)
		}
	} else {
		bitstreamName = bitstreamFilePath
	}

	thy.stats = types.Programming
	log.Print("bit path:", bitstreamName)
	err = programBit(path.Join(BitStreamDir, bitstreamName))
	thy.stats = types.Running
	return
}

//Start spawns a seperate miner for each device defined in the FPGADevices and feeds it with work
func (thy *Thyroid) Start() {
	thy.driverQuit = make(chan struct{})

	go thy.nonceStatistic()
	// go thy.statsReporter()
	log.Println("Starting thyroid driver")
	thy.miningWorkChannel = make(chan *MiningWork, 1)
	go thy.createWork()

	thy.initPort()
	time.Sleep(618 * time.Millisecond)
	// thy.readVersion()
	go thy.minePollVer()
	switch thy.Client.AlgoName() {
	case "odocrypt":
		go thy.readNonceOdo()
	default:
		go thy.readNonce()
	}
	go thy.writeReadNonceRepeatly()
	go thy.processNonce()
	go thy.watchDog()
}

func (thy *Thyroid) Stop() {
	close(thy.driverQuit)
	thy.port.Close()
}

func (thy *Thyroid) selectBoard(board int) {
	thy.boardID = board
	// cmd := exec.Command("/home/pi/towc/PI", "console", strconv.Itoa(board+1))
	// cmd.Run()
	if thy.muxNums == 1 {
		return
	}
	boardman.SelectConsole(uint8(board + 1))
}

func (thy *Thyroid) createWork() {
	//Register a function to clear the generated work if a job gets deprecated.
	// It does not matter if we clear too many, it is worse to work on a stale job.
	thy.Client.SetDeprecatedJobCall(func(jobid string) {
		// log.Println("createWork: Force cleanning job.")
		numberOfWorkItemsToRemove := len(thy.miningWorkChannel) * 1
		for i := 0; i <= numberOfWorkItemsToRemove; i++ {
			<-thy.miningWorkChannel
		}
	})

	thy.Client.SetCleanJobEventCall(func() {
		thy.cleanJobChannel <- true
	})

	var target, header []byte
	var difficulty float64
	var job interface{}
	var err error

	var odoTs uint32
	for {
		select {
		case <-thy.driverQuit:
			return
		default:
			target, difficulty, header, _, job, err = thy.Client.GetHeaderForWork()
		}
		if err != nil {
			thy.logger.Warn("ERROR fetching work", zap.Error(err))
			time.Sleep(1000 * time.Millisecond)
			continue
		}

		switch thy.Client.AlgoName() {
		case "odocrypt":
			thy.blockTimeField = header[68:72]
			ts := binary.LittleEndian.Uint32(thy.blockTimeField)
			ts = ts - ts%(10*24*60*60)
			if ts != odoTs {
				odoTs = ts
				thy.ProgramBitstream("")
			}
		default:
		}

		thy.miningWorkChannel <- &MiningWork{header, 0, target, difficulty, job}
	}
}

func (thy *Thyroid) writeInitCnt() {
	startMine, _ := hex.DecodeString(initCnt0 + pullLow + initCnt1 + pullLow)
	thy.port.Write(startMine)
}

func (thy *Thyroid) write1cSomeJunks() {
	junkChunk, _ := hex.DecodeString("061c" + "aabbccdd")
	thy.port.Write(junkChunk)
}

func (thy *Thyroid) writeStartMine() {
	startMine, _ := hex.DecodeString(startMineCtrlAddr + pullHigh)
	thy.port.Write(startMine)
}

type Nonce struct {
	empty  [8]byte
	len    uint8
	nonces []SingleNonce
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (thy *Thyroid) getNonceSum() (sum uint64) {
	sum = 0
	for _, v := range thy.nonceStats {
		sum += v
	}
	return
}

func (thy *Thyroid) readNonce() {
	thy.logger.Debug("start read nonce")
	scanner := bufio.NewScanner(thy.port)
	nonceStatsMutex := &sync.Mutex{}
	split := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		thy.logger.Debug("UART Data", zap.String("Buffer", fmt.Sprintf("%02X", data)))
		if len(data) < 9 {
			return 0, nil, nil
		}
		index := 0
		for index = 0; index < len(data)-8; index++ {
			if bytes.Equal(data[index:index+8], []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}) {
				nonceNum := int(data[index+8])
				nonceLen := nonceNum * 9

				if len(data) < index+9+nonceLen { // waiting for more data
					return 0, nil, nil
				}

				if nonceNum > 0 {
					if int(data[index+9]) == 0 { // jobid will never be zero
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
		for i := 0; i < noncesLen; i += 9 {
			if nonces[i] == byte(0) {
				continue
			}
			var singleNonce SingleNonce
			singleNonce.jobid = uint8(nonces[i])
			for j := 0; j < 8; j++ {
				singleNonce.nonce[j] = nonces[i+1+j]
			}

			go func(board int) {
				nonceStatsMutex.Lock()
				thy.nonceStats[board]++
				nonceStatsMutex.Unlock()
			}(thy.boardID)
			thy.logger.Debug("Parsed Nonce", zap.Int("BoardID", thy.boardID), zap.String("SingleNonce", fmt.Sprintf("%02X", singleNonce.nonce)), zap.Uint8("JobID", singleNonce.jobid))

			thy.nonceChan <- singleNonce
		}
	}
	thy.logger.Debug("Scanner exited.")

	return
}

func (thy *Thyroid) readNonceOdo() {
	log.Print("start read odo nonce")
	scanner := bufio.NewScanner(thy.port)
	nonceStatsMutex := &sync.Mutex{}
	split := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		thy.logger.Debug("UART Data", zap.String("Buffer", fmt.Sprintf("%02X", data)))
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
		for i := 0; i < noncesLen; i += 9 {
			var singleNonce SingleNonce
			singleNonce.jobid = uint8(nonces[i+1+3])
			copy(singleNonce.nonce[4:], stratum.ReverseByteSlice(nonces[i+1+4:i+1+4+4]))

			go func(board int) {
				nonceStatsMutex.Lock()
				thy.nonceStats[board]++
				nonceStatsMutex.Unlock()
			}(thy.boardID)
			thy.logger.Debug("Parsed Nonce", zap.Int("BoardID", thy.boardID), zap.String("SingleNonce", fmt.Sprintf("%02X", singleNonce.nonce)), zap.Uint8("JobID", singleNonce.jobid))

			thy.nonceChan <- singleNonce
		}
	}
	thy.logger.Debug("Scanner exited.")

	return
}

func (thy *Thyroid) nonceStatistic() {
	for {
		select {
		case <-thy.driverQuit:
			return
		case <-time.After(time.Second * 1):
			var diffMultiplier float64
			switch thy.Client.AlgoName() {
			case "odocrypt":
				stats := thy.Client.GetPoolStats()
				diffMultiplier = stats.Diff
			case "veo":
				diffMultiplier = 1.0
			case "skunk":
				diffMultiplier = 1.0 / 256
			case "xdag":
				diffMultiplier = 1.0
			}
			periodNonceCnt := thy.goldennonceCounter - thy.prevEpochNonceNum
			nonceCntWithWeight := float64(periodNonceCnt) * diffMultiplier
			thy.hr.Add(nonceCntWithWeight)
			thy.prevEpochNonceNum = thy.goldennonceCounter
		}
	}
}

func (thy *Thyroid) watchDog() {
	timeout := time.Second * 30
	for {
		select {
		case <-thy.driverQuit:
			thy.stats = types.Stopped
			return
		case <-time.After(timeout):
			if thy.stats != types.Programming {
				thy.stats = types.NoResponse
			}
		case <-thy.feedDog:
			thy.stats = types.Running
		}
	}
}

func (thy *Thyroid) processNonce() {
	for {
		select {
		case <-thy.driverQuit:
			return
		case nNonce := <-thy.nonceChan:
			thy.workCacheLock.RLock()
			cachedWork := thy.workCache[nNonce.jobid]
			thy.workCacheLock.RUnlock()
			thy.feedDog <- true
			thy.goldennonceCounter++
			if cachedWork.Header != nil {
				go thy.checkAndSubmitJob(nNonce, cachedWork)
			}
		}
	}
}

// type WorkMap map[string]miningWork

func (thy *Thyroid) writeReadNonceRepeatly() {
	for {
		select {
		case <-time.After(time.Millisecond * 10):
			thy.writeReadNonceMutex.Lock()
			thy.port.Write(thy.readNoncePacket)
			thy.writeReadNonceMutex.Unlock()
		}
	}
}

func (thy *Thyroid) singleMinerOnce(boardID int, cleanJob, timeout bool) {
	var work *MiningWork
	var continueMining bool
	thy.selectBoard(boardID)
	time.Sleep(time.Millisecond * thy.PollDelay)

	if cleanJob || timeout {
		select {
		case work, continueMining = <-thy.miningWorkChannel:
		default:
			thy.logger.Debug("Work", zap.String("Stat", "No work ready, continuing"))
			return
		}
		if !continueMining {
			thy.logger.Debug("Work",
				zap.String("Stat", "Halting miner"),
			)
		}

		thy.boardJobID++
		if thy.boardJobID == 0 {
			thy.boardJobID = 1
		}
		var backupWork MiningWork
		copier.Copy(&backupWork, work)
		thy.workCacheLock.Lock()
		if cleanJob {
			thy.workCache = make(map[uint8]MiningWork)
		}
		thy.workCache[thy.boardJobID] = backupWork // cache valid works
		thy.workCacheLock.Unlock()
		headerPacket := thy.MiningFuncs[thy.Client.AlgoName()].ConstructHeaderPackets(work.Header, thy.boardJobID)

		thy.logger.Debug("Write Packet",
			zap.Int("BoardID", boardID),
			zap.Uint8("jobID", thy.boardJobID),
			zap.Bool("CleanJob", cleanJob),
			zap.Bool("Timeout", timeout),
			zap.String("Header", fmt.Sprintf("%02X", work.Header)))

		// log.Printf("packet: \n")
		// for i := 0; i < len(headerPacket); i += 6 {
		// 	log.Printf("%02X\n", headerPacket[i:i+6])
		// }
		// log.Printf("Trying to write packet to board: %d (cleanJob:%v, timeout:%v), header: %02X\n", boardID, cleanJob, timeout, work.Header)
		thy.writeReadNonceMutex.Lock()
		_, err := thy.port.Write(headerPacket)
		if err != nil {
			thy.logger.Error("port.Write")
		}
		thy.writeStartMine()
		thy.writeReadNonceMutex.Unlock()
	}
}

func (thy *Thyroid) checkAndSubmitJob(nNonce SingleNonce, work MiningWork) (goodNonce bool) {
	goodNonce = false
	nonce := nNonce.nonce[:]
	jobid := nNonce.jobid
	workHeader := append(work.Header, nonce...)
	fmt.Printf("workHeader: %02X\n", workHeader)
	blockhash := thy.MiningFuncs[thy.Client.AlgoName()].RegenHash(workHeader)
	thy.logger.Debug("SubmitJob",
		zap.String("BlockHash", fmt.Sprintf("%02X", blockhash)),
		zap.String("Target", fmt.Sprintf("%02X", work.Target)),
		zap.Float64("Difficulty", work.Difficulty),
	)
	if bytes.Equal(blockhash[0:3], []byte{0x00, 0x00, 0x00}) {
		goodNonce = true
		// thy.logger.Debug("SubmitJob", zap.String("Stat", "Golden nonce found!"))
		// if stratum.CheckDifficultyReal(blockhash, work.Target) {
		if thy.MiningFuncs[thy.Client.AlgoName()].DiffChecker(blockhash, work) {
			thy.logger.Debug("SubmitJob", zap.String("Stat", "Share found!"))

			var e error
			if thy.Client.AlgoName() == "veo" {
				nonce = workHeader
			}
			e = thy.Client.SubmitHeader(nonce, work.Job)
			// }
			if e != nil {
				thy.logger.Info("SubmitJob",
					zap.String("Stat", "Error submitting solution"),
					zap.Uint8("jobID", jobid),
					zap.Error(e),
				)
			} else {
				thy.logger.Info("SubmitJob",
					zap.String("Stat", "Accepted!"),
					zap.Uint8("jobID", jobid),
				)
				atomic.AddUint64(&thy.shareCounter, 1)
				//thy.shareCounter++
			}
			// }()
		} else {
			// log.Println(miner.MinerID, "Correct hash but not satisfied with pool diff")
		}
		atomic.StoreUint64(&thy.wronghashCounter, 0)
		// thy.wronghashCounter = 0
	} else {
		thy.logger.Debug("SubmitJob",
			zap.String("WorkHeader", fmt.Sprintf("%02X", workHeader)),
			zap.String("BlockHash", fmt.Sprintf("%02X", blockhash)),
		)
		thy.logger.Info("SubmitJob", zap.String("Stat", "Wrong Hash"))
		atomic.AddUint64(&thy.wronghashCounter, 1)
		// thy.wronghashCounter++
	}
	return
}

func (thy *Thyroid) initPort() {
	options := serial.OpenOptions{
		PortName:        thy.FPGADevice,
		BaudRate:        115200,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 4,
		// InterCharacterTimeout: 100,
	}

	// Open the port.
	port, err := serial.Open(options)
	if err != nil {
		thy.logger.Fatal("Port", zap.Error(err))
	}

	// Make sure to close it later.
	thy.port = port
}

func (thy *Thyroid) minePollVer() {
	var cleanJob bool
	var timeout bool
	for {
		select {
		case <-thy.driverQuit:
			return
		case <-thy.cleanJobChannel:
			cleanJob, timeout = true, false
			for boardID := 0; boardID < thy.muxNums; boardID++ {
				thy.singleMinerOnce(boardID, cleanJob, timeout)
			}
		case <-time.After(time.Millisecond * thy.NonceTraverseTimeout):
			cleanJob, timeout = false, true
			for boardID := 0; boardID < thy.muxNums; boardID++ {
				thy.singleMinerOnce(boardID, cleanJob, timeout)
			}
		}
	}
}

func (thy *Thyroid) readVersion() {
	data := make([]byte, 1024)
	// blackHole := make([]byte, 8192)
	// thy.port.Read(blackHole)
	for i := 0; i < thy.muxNums; i++ {
		thy.selectBoard(i)
		log.Println("Now using board:", i)
		time.Sleep(time.Millisecond * 200)
		readVersion, _ := hex.DecodeString("050200000000")
		thy.port.Write(readVersion)
		len, _ := thy.port.Read(data)
		thy.logger.Info("Bitstream", zap.String("Version", fmt.Sprintf("%02X", data[:len])))
		data = []byte{}
	}
}
