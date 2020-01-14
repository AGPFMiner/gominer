package driver

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"sync/atomic"

	"github.com/AGPFMiner/gominer/boardman"
	"github.com/AGPFMiner/gominer/clients"
	"github.com/AGPFMiner/gominer/clients/stratum"
	"github.com/AGPFMiner/gominer/mining"
	"github.com/AGPFMiner/gominer/statistics"
	"github.com/AGPFMiner/gominer/types"
	"github.com/spf13/viper"
	"github.com/stianeikeland/go-rpio"

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
	BaudRate          uint
	MiningFuncs       map[string]MiningFuncs
	miningWorkChannel chan *MiningWork
	cleanJobChannel   chan bool
	muxNums           int
	blockTimeField    []byte
	skippedSlots      map[int]bool

	Client                          clients.Client
	PollDelay, NonceTraverseTimeout time.Duration
	logger                          *zap.Logger
	port                            io.ReadWriteCloser
	nonceChan                       chan SingleNonce

	workCacheLock     *sync.RWMutex
	readNoncePacket   []byte
	boardJobID        uint8
	jobBoardIDMap     map[uint8]int
	chanSlot          map[int]chan bool
	workCache         map[uint8]MiningWork
	nonceStats        map[int]uint64
	prevEpochEnd      time.Time
	prevEpochNonceNum uint64
	hr                *statistics.HashRate
	stats             types.HardwareStats
	feedDog           chan bool
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

	if thy.muxNums > 1 {
		stats.Temperature, stats.Voltage = "WIP", "WIP"
		return
	}

	if thy.stats != types.Programming || !isOpenocdRunning() {
		stats.Temperature, stats.Voltage, _ = getTempeVolt()
	} else {
		stats.Temperature, stats.Voltage = "-273.15", "25K"
	}

	return
}

func (thy *Thyroid) GetDriverStatsMulti() (statsMulti []*types.DriverStates) {
	oneMin := thy.hr.RecentNSum(60)
	fiveMin := thy.hr.RecentNSum(300)
	oneHour := thy.hr.RecentNSum(3600)
	totalNonces := thy.getNonceSum()

	for board := 0; board < thy.muxNums; board++ {
		stats := &types.DriverStates{}

		stats.DriverName = "Thyroid"
		stats.Status = thy.stats

		norm := float64(thy.nonceStats[board]) / float64(totalNonces)
		stats.NonceNum[0], stats.NonceNum[1], stats.NonceNum[2] = oneMin*norm, fiveMin*norm, oneHour*norm
		stats.Hashrate[0], stats.Hashrate[1], stats.Hashrate[2] = oneMin*FourGiga*norm/60, fiveMin*FourGiga*norm/300, oneHour*FourGiga*norm/3600
		stats.NonceStats = &thy.nonceStats
		stats.Algo = thy.Client.AlgoName()

		if thy.stats != types.Programming || !isOpenocdRunning() {
			boardman.SelectJTAG(uint8(board + 1))
			time.Sleep(1 * time.Millisecond)
			stats.Temperature, stats.Voltage, _ = getTempeVolt()
		} else {
			stats.Temperature, stats.Voltage = "-273.15", "25K"
		}
		statsMulti = append(statsMulti, stats)
	}

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
	if thy.MiningFuncs == nil {
		thy.MiningFuncs = make(map[string]MiningFuncs)
	}

	argsn := args.(mining.MinerArgs)
	thy.FPGADevice = argsn.FPGADevice
	thy.BaudRate = argsn.BaudRate
	thy.PollDelay = argsn.PollDelay
	thy.NonceTraverseTimeout = argsn.NonceTraverseTimeout
	thy.muxNums = argsn.MuxNums
	if thy.muxNums > 1 {
		log.Println("Opening GPIO")
		err := rpio.Open()
		if err != nil {
			log.Println("Cannot open GPIO")
		}
	}
	if thy.logger == nil {
		thy.logger = argsn.Logger
	}

	thy.readNoncePacket, _ = hex.DecodeString(nonceReadCtrlAddr + pullHigh)
	thy.cleanJobChannel = make(chan bool)
	thy.shareCounter = 0
	thy.goldennonceCounter = 0
	thy.wronghashCounter = 0
	thy.boardJobID = 0
	thy.jobBoardIDMap = make(map[uint8]int)
	thy.workCacheLock = &sync.RWMutex{}
	thy.chanSlot = make(map[int]chan bool)
	thy.nonceChan = make(chan SingleNonce, 100)
	thy.workCache = make(map[uint8]MiningWork)
	thy.nonceStats = make(map[int]uint64)
	thy.prevEpochEnd = time.Now()
	thy.prevEpochNonceNum = 0
	thy.hr = &statistics.HashRate{}
	thy.blockTimeField = []byte{}
	thy.skippedSlots = make(map[int]bool)
	skipslots := viper.GetIntSlice("skipslots")
	for _, slot := range skipslots {
		thy.skippedSlots[slot] = true
	}

}

const (
	rpiInterfacePath    = "/usr/share/openocd/scripts/interface/raspberrypi-native.cfg"
	xc7CfgPath          = "/usr/share/openocd/scripts/cpld/xilinx-xc7.cfg"
	xdacPath            = "/usr/share/openocd/scripts/fpga/xilinx-xadc.cfg"
	adapterInit         = "adapter_khz 2500; init;"
	BitStreamDir        = "/opt/scripta/bitstreams"
	queryopenocdProcess = "sudo ps -aux | grep openocd | grep -v grep"
)

func isOpenocdRunning() bool {
	cmd := exec.Command("/bin/sh", "-c", queryopenocdProcess)
	ret, _ := cmd.Output()
	if len(ret) > 0 {
		return true
	}
	return false
}

func programBit(bitstreamPath string) error {

	pldLoadCmd := fmt.Sprintf("%s xc7_program xc7.tap; pld load 0 %s; exit", adapterInit, bitstreamPath)
	openocdCmd := fmt.Sprintf("sudo openocd -f %s -f %s -c '%s'", rpiInterfacePath, xc7CfgPath, pldLoadCmd)
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

func (thy *Thyroid) ProgramBitstream(bitstreamFilePath string) (err error) {
	var bitstreamName string
	if isOpenocdRunning() {
		log.Printf("openocd running")
		return nil
	}

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
	if thy.muxNums > 1 {
		for board := 0; board < thy.muxNums; board++ {
			log.Printf("now programming: %d\n", board+1)
			boardman.SelectJTAG(uint8(board + 1))
			time.Sleep(time.Millisecond * 10)
			err = programBit(path.Join(BitStreamDir, bitstreamName))
		}
	}
	if thy.muxNums == 1 {
		err = programBit(path.Join(BitStreamDir, bitstreamName))
	}

	thy.stats = types.Running
	return
}

//Start spawns a seperate miner for each device defined in the FPGADevices and feeds it with work
func (thy *Thyroid) Start() {
	thy.driverQuit = make(chan struct{})

	go thy.nonceStatistic()
	log.Println("Starting thyroid driver")
	thy.miningWorkChannel = make(chan *MiningWork, 1)
	go thy.createWork()

	thy.initPort()
	time.Sleep(618 * time.Millisecond)
	switch thy.Client.AlgoName() {
	case "odocrypt", "ckb":
		go thy.readNonceNewProtocol()
	default:
		go thy.readNonce()
	}
	go thy.processNonce()

	go thy.minePollVer()
	go thy.watchDog()
}

func (thy *Thyroid) Stop() {
	close(thy.driverQuit)
	thy.port.Close()
}

func (thy *Thyroid) selectBoard(board int) {
	if thy.muxNums == 1 {
		return
	}
	// cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("sudo /home/pi/towc/PI console %d", board+1))
	// cmd.Run()
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
	testFetchedHeader := false
	testMode := viper.GetBool("test")

	var odoTs uint32
	for {
		select {
		case <-thy.driverQuit:
			return
		default:
			if !(testMode && testFetchedHeader) {
				thy.logger.Debug("createWork", zap.String("Header Source", "Stratum"))
				target, difficulty, header, _, job, err = thy.Client.GetHeaderForWork()
				if testMode && err == nil {
					testFetchedHeader = true
				}
			} else {
				thy.logger.Debug("createWork", zap.String("Header Source", "Local test mode"))
			}
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

var (
	startMine, _ = hex.DecodeString(startMineCtrlAddr + pullHigh)
	initcnt, _   = hex.DecodeString(initCnt0 + pullLow + initCnt1 + pullLow)
	junkChunk, _ = hex.DecodeString("061c" + "aabbccdd")
)

func (thy *Thyroid) writeInitCnt() {
	thy.port.Write(initcnt)
}

func (thy *Thyroid) write1cSomeJunks() {
	thy.port.Write(junkChunk)
}

func (thy *Thyroid) writeStartMine() {
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
			}(thy.jobBoardIDMap[singleNonce.jobid])
			thy.logger.Debug("Parsed Nonce", zap.Int("BoardID", thy.jobBoardIDMap[singleNonce.jobid]), zap.String("SingleNonce", fmt.Sprintf("%02X", singleNonce.nonce)), zap.Uint8("JobID", singleNonce.jobid))

			thy.nonceChan <- singleNonce
		}
	}
	thy.logger.Debug("Scanner exited.")

	return
}

func (thy *Thyroid) readNonceNewProtocol() {
	log.Print("start read nonce (new protocol)")
	scanner := bufio.NewScanner(thy.port)
	nonceStatsMutex := &sync.Mutex{}
	split := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		thy.logger.Debug("UART Data", zap.String("Buffer", fmt.Sprintf("%02X", data)))
		datalen := len(data)
		if datalen < 8 {
			return 0, nil, nil
		}
		first89abcd := bytes.Index(data, []byte{0x89, 0xab, 0xcd})

		if first89abcd < 0 {
			return 0, nil, nil
		}
		if datalen-first89abcd < 8 {
			return 0, nil, nil
		}

		return first89abcd + 8, data[first89abcd+3 : first89abcd+8], nil
	}
	scanner.Split(split)
	for scanner.Scan() {
		nonce := scanner.Bytes()
		var singleNonce SingleNonce
		singleNonce.jobid = uint8(nonce[0])
		if singleNonce.jobid == 0 {
			continue
		}
		copy(singleNonce.nonce[4:], stratum.ReverseByteSlice(nonce[1:5]))

		go func(board int) {
			nonceStatsMutex.Lock()
			thy.nonceStats[board]++
			nonceStatsMutex.Unlock()
		}(thy.jobBoardIDMap[singleNonce.jobid])
		thy.logger.Debug("Parsed Nonce", zap.Int("BoardID", thy.jobBoardIDMap[singleNonce.jobid]), zap.String("SingleNonce", fmt.Sprintf("%02X", singleNonce.nonce)), zap.Uint8("JobID", singleNonce.jobid))

		thy.nonceChan <- singleNonce
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
			case "ckb":
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
			// thy.workCacheLock.RLock()
			cachedWork := thy.workCache[nNonce.jobid]
			// thy.workCacheLock.RUnlock()
			thy.feedDog <- true
			thy.goldennonceCounter++
			if cachedWork.Header != nil {
				go thy.checkAndSubmitJob(nNonce, cachedWork)
			}
		}
	}
}

var measuredTime time.Time
var polldelayMeasuredTime time.Time

func (thy *Thyroid) singleMinerOnce(boardID int, cleanJob, timeout bool) {
	// cleanJob, timeout = false, false //for debug
	var work *MiningWork
	var continueMining bool
	polldelayMeasuredTime = time.Now()
	if thy.muxNums > 1 {
		measuredTime = time.Now()
		thy.selectBoard(boardID)
		time.Sleep(time.Microsecond * 1)
		thy.logger.Debug("Execution", zap.Duration("selectBoard", time.Since(measuredTime)))
	}
	/*
		{
			1:true //boardid=0
		}
	*/
	if thy.skippedSlots[boardID+1] {
		thy.logger.Debug("Work", zap.Int("SkippedSlot", boardID+1))
		return
	}

	if !cleanJob && !timeout {
		thy.port.Write(thy.readNoncePacket)
		thy.logger.Debug("Work", zap.String("Stat", "Write readnonce only"))
	}

	//this clause consumes about ?ms
	if cleanJob || timeout {
		measuredTime = time.Now()
		constructStart := measuredTime
		select {
		case work, continueMining = <-thy.miningWorkChannel:
		default:
			thy.logger.Debug("Work", zap.String("Stat", "No work ready, continuing"))
			goto DELAY
		}
		if !continueMining {
			thy.logger.Debug("Work",
				zap.String("Stat", "Halting miner"),
			)
		}
		thy.logger.Debug("Execution", zap.Duration("fetchwork", time.Since(measuredTime)))

		thy.boardJobID++
		if thy.boardJobID == 0 {
			thy.boardJobID = 1
		}

		measuredTime = time.Now()
		var backupWork MiningWork
		copier.Copy(&backupWork, work)
		// thy.workCacheLock.Lock()
		thy.workCache[thy.boardJobID] = backupWork // cache valid works
		// thy.workCacheLock.Unlock()
		thy.logger.Debug("Execution", zap.Duration("cacheWork", time.Since(measuredTime)))

		measuredTime = time.Now()
		headerPacket := thy.MiningFuncs[thy.Client.AlgoName()].ConstructHeaderPackets(work.Header, thy.boardJobID)
		thy.logger.Debug("Execution", zap.Duration("constructPacket", time.Since(measuredTime)))

		thy.logger.Debug("Write Packet",
			zap.Int("BoardID", boardID),
			zap.Uint8("jobID", thy.boardJobID),
			zap.Bool("CleanJob", cleanJob),
			zap.Bool("Timeout", timeout),
			zap.String("Header", fmt.Sprintf("%02X", work.Header)))

		measuredTime = time.Now()
		thy.logger.Debug("Execution", zap.Duration("constructPacket", time.Since(constructStart)))

		time.Sleep(time.Microsecond * 100)
		_, err := thy.port.Write(append(thy.readNoncePacket, append(headerPacket, startMine...)...))
		time.Sleep(time.Millisecond * 1)
		if err != nil {
			thy.logger.Error("port.Write", zap.Error(err))
		}
		thy.logger.Debug("Execution", zap.Duration("writeHeaderAndTrigger", time.Since(measuredTime)))

		thy.jobBoardIDMap[thy.boardJobID] = boardID
	}
DELAY:
	// if !cleanJob {
	instrConsume := time.Since(polldelayMeasuredTime)
	if instrConsume < thy.PollDelay*time.Millisecond {
		time.Sleep(time.Millisecond*thy.PollDelay - instrConsume)
	}
	// }
}

func (thy *Thyroid) checkAndSubmitJob(nNonce SingleNonce, work MiningWork) (goodNonce bool) {
	goodNonce = false
	nonce := nNonce.nonce[:]
	jobid := nNonce.jobid
	workHeader := append(work.Header, nonce...)
	blockhash := thy.MiningFuncs[thy.Client.AlgoName()].RegenHash(workHeader)
	thy.logger.Debug("SubmitJob",
		zap.String("Block", fmt.Sprintf("%02X", workHeader)),
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
		thy.logger.Warn("SubmitJob",
			zap.String("WorkHeader", fmt.Sprintf("%02X", workHeader)),
			zap.String("BlockHash", fmt.Sprintf("%02X", blockhash)),
		)
		thy.logger.Warn("SubmitJob", zap.String("Stat", "Wrong Hash"))
		atomic.AddUint64(&thy.wronghashCounter, 1)
		// thy.wronghashCounter++
	}
	return
}

func (thy *Thyroid) initPort() {
	var err error
	if strings.HasPrefix(thy.FPGADevice, "@") {
		conn, err := net.Dial("tcp", strings.TrimPrefix(thy.FPGADevice, "@"))
		if err != nil {
			thy.logger.Error("initPort", zap.Error(err))
		}
		thy.port = conn
	} else {
		options := serial.OpenOptions{
			PortName:        thy.FPGADevice,
			BaudRate:        thy.BaudRate,
			DataBits:        8,
			StopBits:        1,
			MinimumReadSize: 4,
			// InterCharacterTimeout: 100,
		}

		// Open the port.
		thy.port, err = serial.Open(options)
		if err != nil {
			thy.logger.Fatal("Port", zap.Error(err))
		}
	}
}

func (thy *Thyroid) dispatchJob(cleanJob, timeout bool, startOffset int) {
	for boardID := 0; boardID < thy.muxNums; boardID++ {
		fixedBoardID := (boardID + startOffset) % thy.muxNums
		// log.Printf("dispatch board: %d\n", fixedBoardID)
		thy.singleMinerOnce(fixedBoardID, cleanJob, timeout)
	}
}

func (thy *Thyroid) minePollVer() {
	var cleanJob bool
	var timeout bool
	var lastRefresh []time.Time = make([]time.Time, thy.muxNums)
	for i := range lastRefresh {
		now := time.Now()
		lastRefresh[i] = now
	}
	var boardID int = 0
	for {
		select {
		case <-thy.driverQuit:
			return
		case <-thy.cleanJobChannel:
			thy.workCache = make(map[uint8]MiningWork)
			cleanJob, timeout = true, false
			thy.dispatchJob(cleanJob, timeout, boardID)
			for i := range lastRefresh {
				now := time.Now()
				lastRefresh[i] = now
			}
			continue
		default:
			// log.Printf("mineonce board: %d\n", boardID)
			if time.Now().Sub(lastRefresh[boardID]) > time.Millisecond*thy.NonceTraverseTimeout {
				cleanJob, timeout = false, true
				lastRefresh[boardID] = time.Now()
				thy.singleMinerOnce(boardID, cleanJob, timeout)
			} else {
				cleanJob, timeout = false, false
				thy.singleMinerOnce(boardID, cleanJob, timeout)
			}
		}
		boardID++
		boardID %= thy.muxNums
		// if boardID == thy.muxNums {
		// 	boardID = 0
		// }
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
