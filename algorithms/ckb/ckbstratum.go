package ckb

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AGPFMiner/gominer/clients"
	"github.com/AGPFMiner/gominer/clients/stratum"
	"github.com/AGPFMiner/gominer/types"
)

const (
	//HashSize is the length of a sha256 hash
	HashSize = 32
)

//Target declares what a solution should be smaller than to be accepted
type Target [HashSize]byte

type stratumJob struct {
	JobID       string
	headerHash  string
	height      int
	parentHash  string
	ExtraNonce2 stratum.ExtraNonce2
	CleanJobs   bool
}

//StratumClient is a ckb client using the stratum protocol
type StratumClient struct {
	accept, reject, discard int32
	lastAccept              int64
	Connectionstring        string
	User, Password          string
	Algo                    string
	mutex                   sync.Mutex // protects following
	stratumclient           *stratum.Client
	target                  Target
	nonce1                  string
	nonce2Size              uint
	Difficulty              float64
	currentJob              stratumJob
	clients.BaseClient
	stopSig chan bool
}

func (sc *StratumClient) GetPoolStats() (info types.PoolStates) {
	info.Status = sc.PoolConnectionStates()
	info.User = sc.User
	info.PoolAddr = "stratum+tcp://" + sc.Connectionstring
	info.Algo = sc.Algo
	info.Accept, info.Reject = sc.accept, sc.reject
	info.Diff = float64(sc.Difficulty)
	info.LastAccepted = sc.lastAccept
	return
}

type CKBStratum struct {
	Id    string `json:"id,omitempty"`
	BHash string `json:"bHash,omitempty"`
	Nonce string `json:"nonce,omitempty"`
	JDiff int    `json:"jDiff,omitempty"`
	JId   string `json:"jId,omitempty"`
}

func (sc *StratumClient) Start() {
	sc.stopSig = make(chan bool)
	sc.startPoolConn()
	for {
		select {
		case <-time.After(5 * time.Second):
			stratumclient := sc.stratumclient
			if stratumclient != nil {
				poolstats := stratumclient.PoolConnectionStates()
				switch poolstats {
				case types.Alive:
					continue
				case types.Dead:
					log.Print("Pool dead, retry after 30s")
					time.Sleep(30)
					fallthrough
				case types.Sick:
					stratumclient.Close()
					sc.startPoolConn()
				}
			}
		case <-sc.stopSig:
			return
		}
	}
}

func (sc *StratumClient) Stop() {
	sc.stopSig <- true
	sc.stratumclient.Close()
}

func (sc *StratumClient) AlgoName() string {
	return sc.Algo
}

func (sc *StratumClient) PoolConnectionStates() types.PoolConnectionStates {
	return sc.stratumclient.PoolConnectionStates()
}

//startPoolConn connects to the stratumserver and processes the notifications
func (sc *StratumClient) startPoolConn() {
	sc.DeprecateOutstandingJobs()

	sc.stratumclient = &stratum.Client{}
	//In case of an error, drop the current stratumclient and restart
	sc.stratumclient.ErrorCallback = func(err error) {
	}

	sc.subscribeToStratumDifficultyChanges()
	sc.subscribeToStratumJobNotifications()

	//Connect to the stratum server
	log.Println("Connecting to", sc.Connectionstring)
	sc.stratumclient.Dial(sc.Connectionstring)

	//Subscribe for mining
	//Close the connection on an error will cause the client to generate an error, resulting in te errorhandler to be triggered

	result, err := sc.stratumclient.Call("mining.subscribe", []interface{}{"AGPFminer", nil})
	if err != nil {
		log.Println("ERROR Error in response from stratum:", err)
		sc.stratumclient.Close()
		return
	}
	stratumRes := result.([]interface{})
	log.Println(stratumRes)
	sc.nonce2Size = uint(stratumRes[2].(float64))
	sc.nonce1 = stratumRes[1].(string)

	go func() {
		result, err = sc.stratumclient.Call("mining.authorize", []string{sc.User, sc.Password})
		if err != nil {
			log.Println("Unable to authorize:", err)
			return
		}
		log.Println("Authorization of", sc.User, ":", result)
	}()
}

// var diff1, _ = big.NewInt(0).SetString("0x00000000FFFF0000000000000000000000000000000000000000000000000000", 0)

func (sc *StratumClient) subscribeToStratumDifficultyChanges() {
	sc.stratumclient.SetNotificationHandler("mining.set_target", func(params []interface{}, result interface{}) {
		targetStr, ok := params[0].(string)
		if !ok {
			log.Print("invalid target string")
			return
		}
		target, err := hex.DecodeString(targetStr)
		if err != nil {
			log.Print("unable to decode target")
			return
		}

		log.Println("Stratum server changed target to", targetStr)
		for i := 0; i < 32; i++ {
			sc.target[i] = target[i]
		}
		// targetInt := big.NewInt(0).SetBytes(target).Uint64()
		// sc.Difficulty = float64(diff1.Uint64()) / float64(targetInt)
	})
}

func (sc *StratumClient) subscribeToStratumJobNotifications() {
	sc.stratumclient.SetNotificationHandler("mining.notify", func(params []interface{}, result interface{}) {
		sj := stratumJob{}
		if len(params) < 2 {
			log.Print("invalid params")
			return
		}
		jobID, ok := params[0].(string)
		if !ok {
			log.Print("invalid jobId")
			return
		}
		sj.JobID = jobID

		powHash, ok := params[1].(string)
		_, err := hex.DecodeString(powHash)
		if !ok || err != nil {
			log.Print("invalid powHash")
			return
		}

		cleanJob, ok := params[4].(bool)
		if !ok {
			log.Print("invalid cleanJob req")
			return
		}

		sj.headerHash = powHash
		sj.CleanJobs = cleanJob
		sj.ExtraNonce2.Size = sc.nonce2Size - 4 //fpga returns 4 bytes

		sc.addNewStratumJob(sj)
	})
}

func (sc *StratumClient) addNewStratumJob(sj stratumJob) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	sc.currentJob = sj
	if sj.CleanJobs {
		sc.discard++
		sc.DeprecateOutstandingJobs()
	}
	sc.AddJobToDeprecate(sj.JobID)
}

//GetHeaderForWork fetches new work from the stratum pool
func (sc *StratumClient) GetHeaderForWork() (target []byte, difficulty float64, header []byte, deprecationChannel chan bool, job interface{}, err error) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	job = sc.currentJob
	if sc.currentJob.JobID == "" {
		err = errors.New("No job received from stratum server yet")
		return
	}

	deprecationChannel = sc.GetDeprecationChannel(sc.currentJob.JobID)

	target = sc.target[:]
	difficulty = -999
	headerHashDecoded, _ := hex.DecodeString(sc.currentJob.headerHash)
	nonce1Decoded, _ := hex.DecodeString(sc.nonce1)
	header = make([]byte, 0, 44)

	header = append(header, headerHashDecoded...)
	header = append(header, nonce1Decoded...)
	header = append(header, sc.currentJob.ExtraNonce2.Bytes()...)
	sc.currentJob.ExtraNonce2.Increment()

	// header, _ = hex.DecodeString("d5a74fba920ad0d35ec5726f26327547cbc82180e356e5ccf6cf2e6bd75f8a6600c904bd0000000000000000")

	return
}

//SubmitHeader reports a solution to the stratum server
func (sc *StratumClient) SubmitHeader(nonce []byte, job interface{}) (err error) {
	sj, _ := job.(stratumJob)
	sc.mutex.Lock()
	c := sc.stratumclient
	sc.mutex.Unlock()
	stratumUser := sc.User
	jobID := sj.JobID
	nonce2Str := hex.EncodeToString(append(sj.ExtraNonce2.Bytes(), stratum.RevBytes(nonce[4:])...))
	strSubmit := []string{stratumUser, jobID, nonce2Str}
	fmt.Printf("strSubmit: %v\n", strSubmit)
	_, err = c.Call("mining.submit", strSubmit)
	if err != nil {
		atomic.AddInt32(&sc.reject, 1)
	} else {
		atomic.AddInt32(&sc.accept, 1)
		sc.lastAccept = time.Now().Unix()
	}
	return
}
