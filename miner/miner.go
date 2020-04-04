package miner

import (
	j "encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/AGPFMiner/gominer/algorithms/ckb"
	"github.com/AGPFMiner/gominer/algorithms/odocrypt"
	"github.com/AGPFMiner/gominer/algorithms/skunk"
	"github.com/AGPFMiner/gominer/algorithms/trb"
	"github.com/AGPFMiner/gominer/algorithms/veo"
	"github.com/AGPFMiner/gominer/algorithms/verus"
	"github.com/AGPFMiner/gominer/algorithms/xdag"
	"github.com/AGPFMiner/gominer/clients"
	"github.com/AGPFMiner/gominer/driver"
	"github.com/AGPFMiner/gominer/mining"
	"github.com/AGPFMiner/gominer/types"

	"github.com/gorilla/mux"
	"github.com/gorilla/rpc"
	"github.com/gorilla/rpc/json"

	"go.uber.org/zap/zapcore"

	"os"

	"go.uber.org/zap"
)

var atom = zap.NewAtomicLevel()
var logger *zap.Logger

func selectZapLevel(loglevel string) zapcore.Level {
	var level zapcore.Level
	switch loglevel {
	case "debug":
		level = zap.DebugLevel
	case "info":
		level = zap.InfoLevel
	case "error":
		level = zap.ErrorLevel
	default:
		level = zap.InfoLevel
	}
	return level
}
func initLogger(loglevel string) *zap.Logger {
	level := selectZapLevel(loglevel)
	encoderCfg := zap.NewProductionEncoderConfig()
	logger = zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.Lock(os.Stdout),
		atom,
	))
	defer logger.Sync()
	atom.SetLevel(level)
	return logger
}

//Miner do everything
type Miner struct {
	Pools []types.Pool

	Driver, DevPath                 string
	BaudRate                        uint
	MuxNums                         int
	PollDelay, NonceTraverseTimeout int64

	WebEnable bool
	WebListen string

	LogLevel    string
	currentAlgo string

	driver    driver.Driver
	clients   []clients.Client
	miners    []mining.Miner
	activeIdx int
}

func getMinerByName(pool *types.Pool) (mining.Miner, clients.Client, error) {
	switch pool.Algo {
	case "ckb":
		return &ckb.Miner{}, ckb.NewClient(pool), nil
	case "trb":
		return &trb.Miner{}, trb.NewClient(pool), nil
	case "odocrypt":
		return &odocrypt.Miner{}, odocrypt.NewClient(pool), nil
	case "veo":
		return &veo.Miner{}, veo.NewClient(pool), nil
	case "skunk":
		return &skunk.Miner{}, skunk.NewClient(pool), nil
	case "xdag":
		return &xdag.Miner{}, xdag.NewClient(pool), nil
	case "verus":
		return &verus.Miner{}, verus.NewClient(pool), nil
	default:
		return nil, nil, errors.New("Not supported")
	}
}

//Reload the main miner
func (m *Miner) Reload() {
	m.driver.Stop()
	log.Print("Reloading miner")
	loglvl := selectZapLevel(m.LogLevel)
	atom.SetLevel(loglvl)
	for _, cli := range m.clients {
		log.Print("Stopping pool:", cli.GetPoolStats().PoolAddr)
		cli.Stop()
	}
	m.clients = make([]clients.Client, len(m.Pools))
	// m.miners = make([]*mining.Miner, len(m.Pools))

	prevAlgo := m.currentAlgo

	for i, pool := range m.Pools {
		_, client, err := getMinerByName(&pool)
		if err != nil {
			continue
		}
		if pool.Active {
			m.activeIdx = i
			m.currentAlgo = pool.Algo
		}
		go client.Start()
		m.clients[i] = client
		// m.miners[i] = &miner
	}

	driverArgs := &mining.MinerArgs{}
	driverArgs.FPGADevice = m.DevPath
	driverArgs.BaudRate = m.BaudRate
	driverArgs.MuxNums = m.MuxNums
	driverArgs.PollDelay = time.Duration(m.PollDelay)
	if m.NonceTraverseTimeout != 0 {
		driverArgs.NonceTraverseTimeout = time.Duration(m.NonceTraverseTimeout)
	}

	switch m.Driver {
	case "thyroid":
		m.driver.Init(*driverArgs)
	}

	m.driver.SetClient(m.clients[m.activeIdx])
	if prevAlgo != m.currentAlgo {
		prevAlgo = m.currentAlgo
	}

	m.driver.Start()

}

//MinerMain starts the miner
func (m *Miner) MinerMain() {
	log.SetOutput(os.Stdout)

	m.clients = make([]clients.Client, len(m.Pools))
	m.miners = make([]mining.Miner, len(m.Pools))

	logger := initLogger(m.LogLevel)

	driverArgs := &mining.MinerArgs{}
	driverArgs.FPGADevice = m.DevPath
	driverArgs.BaudRate = m.BaudRate
	driverArgs.MuxNums = m.MuxNums
	driverArgs.PollDelay = time.Duration(m.PollDelay)
	if m.NonceTraverseTimeout != 0 {
		driverArgs.NonceTraverseTimeout = time.Duration(m.NonceTraverseTimeout)
	}
	driverArgs.Logger = logger

	switch m.Driver {
	case "thyroid":
		m.driver = driver.NewThyroid(*driverArgs)
	case "thyroidUSB":
		// m.driver = driver.NewThyroidUSB(*driverArgs)
	}

	for i, pool := range m.Pools {
		_, client, err := getMinerByName(&pool)
		if err != nil {
			continue
		}
		if pool.Active {
			m.activeIdx = i
			m.currentAlgo = pool.Algo
		}
		go client.Start()
		m.clients[i] = client
		// m.miners[i] = &miner
	}

	m.driver.RegisterMiningFuncs("ckb", &ckb.MiningFuncs{})
	m.driver.RegisterMiningFuncs("trb", &trb.MiningFuncs{})
	m.driver.RegisterMiningFuncs("odocrypt", &odocrypt.MiningFuncs{})
	m.driver.RegisterMiningFuncs("veo", &veo.MiningFuncs{})
	m.driver.RegisterMiningFuncs("skunk", &skunk.MiningFuncs{})
	m.driver.RegisterMiningFuncs("xdag", &xdag.MiningFuncs{})

	m.driver.SetClient(m.clients[m.activeIdx])

	switch m.currentAlgo {
	case "odocrypt":
		// let driver manage odo bit
	default:
		go m.driver.ProgramBitstream("")
	}
	m.driver.Start()

	s := rpc.NewServer()
	s.RegisterCodec(json.NewCodec(), "application/json")
	s.RegisterCodec(json.NewCodec(), "application/json;charset=UTF-8")
	s.RegisterService(m, "miner")
	r := mux.NewRouter()
	r.Handle("/rpc", s)

	r.HandleFunc("/gominer/f_status", m.GetScriptaStatus)
	r.HandleFunc("/gominer/f_miner", m.MinerCtrl)
	http.ListenAndServe(":1234", r)
}

type MinerRPCArgs struct {
	Who string
}

type MinerRPCReply struct {
	PoolsInfo string
	Activated int
}

func (m *Miner) GetPoolsStats(r *http.Request, args *MinerRPCArgs, reply *MinerRPCReply) error {
	var poolsInfo []*types.PoolStates
	for _, client := range m.clients {
		poolInfo := client.GetPoolStats()
		poolsInfo = append(poolsInfo, &poolInfo)
	}
	res, _ := j.Marshal(poolsInfo)
	// spew.Dump(string(res))
	reply.PoolsInfo = string(res)
	reply.Activated = m.activeIdx
	return nil
}

type DriverRPCReply struct {
	DriverInfo string
}

func (m *Miner) GetHardwareStats(r *http.Request, args *MinerRPCArgs, reply *DriverRPCReply) error {
	driverStats := m.driver.GetDriverStats()
	res, _ := j.Marshal(driverStats)
	reply.DriverInfo = string(res)
	return nil
}

func (m *Miner) GetScriptaStatus(w http.ResponseWriter, r *http.Request) {
	var devsInfo []*types.DriverStates
	if m.MuxNums > 1 {
		devsInfo = m.driver.GetDriverStatsMulti()
	} else if m.MuxNums == 1 {
		ds := m.driver.GetDriverStats()
		var devsInfo []*types.DriverStates
		devsInfo = append(devsInfo, &ds)
	}

	var poolsInfo []*types.PoolStates
	for i, client := range m.clients {
		poolInfo := client.GetPoolStats()
		if i == m.activeIdx {
			poolInfo.Active = true
		} else {
			poolInfo.Active = false
		}
		poolsInfo = append(poolsInfo, &poolInfo)
	}

	data := &types.ScriptaStatus{
		Status: &types.ScriptaMinerStatus{
			Devs:      devsInfo,
			Pools:     poolsInfo,
			MinerUp:   true,
			MinerDown: false,
			Time:      time.Now().Unix(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	j.NewEncoder(w).Encode(data)
	return
}

func (m *Miner) MinerCtrl(w http.ResponseWriter, r *http.Request) {
	cmds, ok := r.URL.Query()["command"]

	if !ok || len(cmds[0]) < 1 {
		log.Println("Url Param 'cmd' is missing")
		return
	}

	log.Print(cmds)
	cmd := cmds[0]
	switch cmd {
	case "programbitstream":
		err := m.driver.ProgramBitstream("")
		log.Print(err)
		if err != nil {
			w.WriteHeader(http.StatusOK)
			return
		}
	case "reload":
		m.Reload()
	}
	// err := m.driver.ProgramBitstream("")
	// if err != nil {
	// 	w.WriteHeader(http.StatusOK)
	// 	return
	// }
}
