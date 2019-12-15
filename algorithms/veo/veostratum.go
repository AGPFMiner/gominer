package veo

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/dynm/gominer/clients"
	"github.com/dynm/gominer/clients/stratum"
	"github.com/dynm/gominer/types"

	"github.com/mitchellh/mapstructure"
)

const (
	//HashSize is the length of a sha256 hash
	HashSize = 32

	ErrorIDInvalidShare = 21
	ErrorIDLowDiffShare = 23

	MessageIDSubscribe   = 1
	MessageIDSubmitStart = 2

	MethodIDSubscribe    = 0
	MethodIDSubmitWork   = 1
	MethodIDNewBlockHash = 2
	MethodIDNewJobDiff   = 3
)

//Target declares what a solution should be smaller than to be accepted
type Target [HashSize]byte

type stratumJob struct {
	JobID string
	bHash string
	nonce string
}

//StratumClient is a groestl client using the stratum protocol
type StratumClient struct {
	connectionstring string
	User             string
	Algo             string
	mutex            sync.Mutex // protects following
	stratumclient    *stratum.Client
	target           Target
	Difficulty       int
	accept, reject   int32
	lastAccept       int64
	currentJob       stratumJob
	clients.BaseClient
	stopSig chan bool
}

func (sc *StratumClient) GetPoolStats() (info types.PoolStates) {
	info.Status = sc.PoolConnectionStates()
	info.User = sc.User
	info.PoolAddr = "stratum+tcp://" + sc.connectionstring
	info.Algo = sc.Algo
	info.Accept, info.Reject = sc.accept, sc.reject
	info.Diff = float64(sc.Difficulty)
	info.LastAccepted = sc.lastAccept
	return
}

type VeoStratum struct {
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
	sc.stratumclient.Veo = true
	//In case of an error, drop the current stratumclient and restart
	sc.stratumclient.ErrorCallback = func(err error) {
	}

	sc.subscribeToStratumDifficultyChanges()
	sc.subscribeToStratumJobNotifications()

	//Connect to the stratum server
	log.Println("Connecting to", sc.connectionstring)
	sc.stratumclient.Dial(sc.connectionstring)

	//Subscribe for mining
	//Close the connection on an error will cause the client to generate an error, resulting in te errorhandler to be triggered

	result, err := sc.stratumclient.Call(MethodIDSubscribe, VeoStratum{Id: sc.User})
	// if err != nil {
	// 	log.Println("ERROR Error in response from stratum:", err)
	// 	sc.stratumclient.Close()
	// 	return
	// }
	var reply VeoStratum
	err = mapstructure.Decode(result, &reply)
	// spew.Dump(result, reply)
	if err != nil {
		// log.Println("ERROR Invalid response from stratum:", err)
		// sc.stratumclient.Close()
		// return
	}
}

func (sc *StratumClient) subscribeToStratumDifficultyChanges() {
	sc.stratumclient.SetNotificationHandler(MethodIDNewJobDiff, func(params []interface{}, result interface{}) {
		log.Println("New diff change")
		var reply VeoStratum
		mapstructure.Decode(result, &reply)
		diff := reply.JDiff

		log.Println("Stratum server changed difficulty to", diff)
		sc.setDifficulty(diff)
	})
}

func (sc *StratumClient) subscribeToStratumJobNotifications() {
	sc.stratumclient.SetNotificationHandler(MethodIDNewBlockHash, func(params []interface{}, result interface{}) {
		// log.Println("New job received from stratum server")

		sj := stratumJob{}
		var reply VeoStratum
		mapstructure.Decode(result, &reply)
		sj.bHash = reply.BHash
		sj.JobID = reply.BHash
		diff := reply.JDiff

		if diff != 0 {
			log.Println("Stratum server changed difficulty to", diff)
			sc.setDifficulty(diff)
		}

		sc.addNewStratumJob(sj)
	})
}

func (sc *StratumClient) addNewStratumJob(sj stratumJob) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	sc.currentJob = sj
	sc.DeprecateOutstandingJobs()
	sc.AddJobToDeprecate(sj.JobID)
}

func (sc *StratumClient) setDifficulty(difficulty int) {
	sc.DeprecateOutstandingJobs()
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	sc.Difficulty = difficulty

}

//GetHeaderForWork fetches new work from the stratum pool
func (sc *StratumClient) GetHeaderForWork() (target []byte, difficulty float64, header []byte, deprecationChannel chan bool, job interface{}, err error) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	// log.Println("GetHeaderForWork")
	job = sc.currentJob
	if sc.currentJob.JobID == "" {
		err = errors.New("No job received from stratum server yet")
		return
	}

	deprecationChannel = sc.GetDeprecationChannel(sc.currentJob.JobID)

	/*
		7b28f9ec
		00000000
		c1930959
		00000000
		00045400000e6e
	*/
	target = sc.target[:]
	difficulty = float64(sc.Difficulty)
	bHashDecoded, _ := base64.StdEncoding.DecodeString(sc.currentJob.bHash)
	// log.Printf("bHashDecoded: %02X\n", bHashDecoded)
	// en2 := sc.currentJob.extraNonce.Bytes()
	// err = sc.currentJob.extraNonce.Increment()
	//Construct the header
	header = make([]byte, 0, 55)
	// log.Printf("Header1: %02x\n", header)

	header = append(header, bHashDecoded...)
	// log.Printf("Header2: %02x\n", header)
	rd := make([]byte, 4)
	rand.Read(rd)
	header = append(header, rd...)
	header = append(header, []byte{0x00, 0x00, 0x00, 0x00}...)
	rand.Read(rd)
	header = append(header, rd...)
	header = append(header, []byte{0x00, 0x00, 0x00, 0x00}...)
	// log.Printf("Header3: %02x,enSize: %d\n", header, sc.currentJob.extraNonce.Size)

	return
}

//SubmitHeader reports a solution to the stratum server
func (sc *StratumClient) SubmitHeader(header []byte, job interface{}) (err error) {
	// sj, _ := job.(stratumJob)
	header1, header2 := header[:48], header[49:56]
	headerFin := append(header1, header2...)
	nonceEncoded := base64.StdEncoding.EncodeToString(headerFin[32:55])
	sc.mutex.Lock()
	c := sc.stratumclient
	sc.mutex.Unlock()
	stratumUser := sc.User
	strSubmit := &VeoStratum{Id: stratumUser, Nonce: nonceEncoded}
	fmt.Printf("header: %02x\nstrSubmit: %v\n", header, strSubmit)
	reply, err := c.Call(MethodIDSubmitWork, strSubmit)
	if err != nil {
		log.Println("veo submit share err:", reply, err)
	}
	switch reply.(type) {
	case map[string]interface{}:
		rep := reply.(map[string]interface{})
		if rep["acc"] == 1 || err == nil {
			err = nil
			sc.accept++
			sc.lastAccept = time.Now().Unix()
		} else {
			sc.reject++
		}
	}
	return
}
