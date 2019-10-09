package verus

import (
	"bytes"
	"encoding/hex"
	"errors"
	"log"
	"math/big"
	"sync"
	"time"

	"gominer/clients"
	"gominer/clients/stratum"
	"gominer/types"
)

const (
	//HashSize is the length of a groestl hash
	HashSize = 32
)

//Target declares what a solution should be smaller than to be accepted
type Target [HashSize]byte

type stratumJob struct {
	JobID       string
	Version     []byte
	Hash1       []byte
	Hash2       []byte
	Hash3       []byte
	NBits       []byte
	NTime       []byte
	CleanJobs   bool
	ExtraNonce2 stratum.ExtraNonce2
}

//StratumClient is a groestl client using the stratum protocol
type StratumClient struct {
	connectionstring string
	User             string
	Password         string
	Algo             string

	mutex                   sync.Mutex // protects following
	stratumclient           *stratum.Client
	extranonce1             []byte
	extranonce2Size         uint
	target                  Target
	Difficulty              float64
	accept, reject, discard int
	lastAccept              int64
	currentJob              stratumJob
	clients.BaseClient
}

func (sc *StratumClient) GetPoolStats() (info types.PoolStates) {
	info.Status = sc.PoolConnectionStates()
	info.User = sc.User
	info.PoolAddr = "stratum+tcp://" + sc.connectionstring
	info.Algo = sc.Algo
	info.Accept, info.Reject, info.Discard = sc.accept, sc.reject, sc.discard
	info.Diff = sc.Difficulty
	info.LastAccepted = sc.lastAccept
	return
}

func (sc *StratumClient) AlgoName() string {
	return sc.Algo
}

func (sc *StratumClient) PoolConnectionStates() types.PoolConnectionStates {
	return types.Alive
}

//Start connects to the stratumserver and processes the notifications
func (sc *StratumClient) Start() {
	log.Println("sc.Start()")
	sc.mutex.Lock()
	log.Println("mutex.Lock()")

	defer func() {
		log.Println("before mutex.Unlock()")
		sc.mutex.Unlock()
		log.Println("after mutex.Unlock()")
	}()
	log.Println("before deprecate()")
	sc.DeprecateOutstandingJobs()
	log.Println("after mutex.Unlock()")

	sc.stratumclient = &stratum.Client{}
	//In case of an error, drop the current stratumclient and restart
	sc.stratumclient.ErrorCallback = func(err error) {
		log.Println("Error in connection to stratumserver:", err)
		sc.stratumclient.Close()
		log.Println("Retrying")
		sc.Start()
		log.Println("Start()")
	}

	sc.subscribeToStratumDifficultyChanges()
	sc.subscribeToStratumJobNotifications()

	//Connect to the stratum server
	log.Println("Connecting to", sc.connectionstring)
	sc.stratumclient.Dial(sc.connectionstring)

	//Subscribe for mining
	//Close the connection on an error will cause the client to generate an error, resulting in te errorhandler to be triggered
	result, err := sc.stratumclient.Call("mining.subscribe", []string{"AGPFminer"})
	if err != nil {
		log.Println("ERROR Error in response from stratum:", err)
		sc.stratumclient.Close()
		return
	}
	reply, ok := result.([]interface{})
	if !ok || len(reply) < 2 {
		log.Println("ERROR Invalid response from stratum:", result)
		sc.stratumclient.Close()
		return
	}

	//Keep the extranonce1 and extranonce2_size from the reply
	if sc.extranonce1, err = stratum.HexStringToBytes(reply[1]); err != nil {
		log.Println("ERROR Invalid extrannonce1 from startum")
		sc.stratumclient.Close()
		return
	}

	sc.extranonce2Size = uint(32 - len(reply[1].(string))/2)

	//Authorize the miner
	go func() {
		result, err = sc.stratumclient.Call("mining.authorize", []string{sc.User, sc.Password})
		if err != nil {
			log.Println("Unable to authorize:", err)
			sc.stratumclient.Close()
			return
		}
		log.Println("Authorization of", sc.User, ":", result)
	}()

}

func (sc *StratumClient) subscribeToStratumDifficultyChanges() {
	sc.stratumclient.SetNotificationHandler("mining.set_target", func(params []interface{}, result interface{}) {
		if params == nil || len(params) < 1 {
			log.Println("ERROR No target parameter supplied by stratum server")
			return
		}
		target, ok := params[0].(string)
		if !ok {
			log.Println("ERROR Invalid target supplied by stratum server:", params[0])
			return
		}
		log.Println("Stratum server changed difficulty to", target[:16])
		sc.setTarget(target)
		sc.Difficulty = 1.0
	})
}

func (sc *StratumClient) subscribeToStratumJobNotifications() {
	sc.stratumclient.SetNotificationHandler("mining.notify", func(params []interface{}, result interface{}) {
		// log.Println("New job received from stratum server")
		if params == nil || len(params) < 8 {
			log.Println("ERROR Wrong number of parameters supplied by stratum server")
			return
		}

		sj := stratumJob{}

		sj.ExtraNonce2.Size = sc.extranonce2Size

		var ok bool
		var err error
		if sj.JobID, ok = params[0].(string); !ok {
			log.Println("ERROR Wrong job_id parameter supplied by stratum server")
			return
		}
		if sj.Version, err = stratum.HexStringToBytes(params[1]); err != nil {
			log.Println("ERROR Wrong version parameter supplied by stratum server")
			return
		}
		if sj.Hash1, err = stratum.HexStringToBytes(params[2]); err != nil {
			log.Println("ERROR Wrong hash1 parameter supplied by stratum server")
			return
		}
		if sj.Hash2, err = stratum.HexStringToBytes(params[3]); err != nil {
			log.Println("ERROR Wrong hash2 parameter supplied by stratum server")
			return
		}
		if sj.Hash3, err = stratum.HexStringToBytes(params[4]); err != nil {
			log.Println("ERROR Wrong hash3 parameter supplied by stratum server")
			return
		}
		if sj.NTime, err = stratum.HexStringToBytes(params[5]); err != nil {
			log.Println("ERROR Wrong ntime parameter supplied by stratum server")
			return
		}
		if sj.NBits, err = stratum.HexStringToBytes(params[6]); err != nil {
			log.Println("ERROR Wrong nbits parameter supplied by stratum server")
			return
		}
		if sj.CleanJobs, ok = params[7].(bool); !ok {
			log.Println("ERROR Wrong clean_jobs parameter supplied by stratum server")
			return
		}
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

// IntToTarget converts a big.Int to a Target.
func intToTarget(i *big.Int) (t Target, err error) {
	// Check for negatives.
	if i.Sign() < 0 {
		err = errors.New("Negative target")
		return
	}
	// In the event of overflow, return the maximum.
	if i.BitLen() > 256 {
		err = errors.New("Target is too high")
		return
	}
	b := i.Bytes()
	offset := len(t[:]) - len(b)
	copy(t[offset:], b)
	return
}

func difficultyToTarget(difficulty float64) (target Target, err error) {
	diffAsBig := big.NewFloat(difficulty)

	diffOneString := "0x00000000FFFF0000000000000000000000000000000000000000000000000000"
	targetOneAsBigInt := &big.Int{}
	targetOneAsBigInt.SetString(diffOneString, 0)

	targetAsBigFloat := &big.Float{}
	targetAsBigFloat.SetInt(targetOneAsBigInt)
	targetAsBigFloat.Quo(targetAsBigFloat, diffAsBig)
	targetAsBigInt, _ := targetAsBigFloat.Int(nil)
	target, err = intToTarget(targetAsBigInt)
	return
}

func (sc *StratumClient) setDifficulty(difficulty float64) {
	target, err := difficultyToTarget(difficulty)

	if err != nil {
		log.Println("ERROR Error setting difficulty to ", difficulty)
	}
	sc.DeprecateOutstandingJobs()
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	sc.target = target
}

func (sc *StratumClient) setTarget(targetStr string) {
	targetAsBigInt := &big.Int{}
	targetAsBigInt.SetString(targetStr, 16)

	sc.DeprecateOutstandingJobs()
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	target, err := intToTarget(targetAsBigInt)
	if err != nil {
		log.Println("ERROR Error setting target to ", targetStr)
	}
	sc.target = target
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
	difficulty = sc.Difficulty

	//Create the arbitrary transaction
	en2 := sc.currentJob.ExtraNonce2.Bytes()
	err = sc.currentJob.ExtraNonce2.Increment()

	//Construct the header
	/*
		04000100
		6D635EA1D8B9638AEE3D39FD7A7D040EAB56F8B7038DA6E59C010B0000000000
		6BB089D021EB23B7AF029AE661690C32B8D79ECDC470EFD21EA77B244B61824F
		EB7D01915EE56B0C75791FDBFDE8924DD1A379BA5C5442058DF9641BE627DB1C
		298B305D
		543D0E1B
		0FFF252900000000000000000000000000000000000000000000000000000000
		FD4005010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000
	*/
	header = make([]byte, 0, 1487)
	header = append(header, sc.currentJob.Version...) //version
	header = append(header, sc.currentJob.Hash1...)
	header = append(header, sc.currentJob.Hash2...)
	header = append(header, sc.currentJob.Hash3...)
	header = append(header, sc.currentJob.NTime...)
	header = append(header, sc.currentJob.NBits...)
	header = append(header, sc.extranonce1...)
	header = append(header, en2...)
	header = append(header, []byte{0xfd, 0x40, 0x05, 0x01}...)
	emptySol := bytes.Repeat([]byte{0}, 1343)
	header = append(header, emptySol...) //empty sol
	return
}

//SubmitHeader reports a solution to the stratum server
func (sc *StratumClient) SubmitHeader(header []byte, job interface{}) (err error) {
	sj, _ := job.(stratumJob)
	solution := header[140:1487]
	solNonce := header[1487:]
	copy(solution[1332:1332+4], solNonce) //last 15bytes's first 4 bytes
	solutionStr := hex.EncodeToString(solution)
	encodedExtraNonce2 := hex.EncodeToString(sj.ExtraNonce2.Bytes())
	nTime := hex.EncodeToString(sj.NTime)
	sc.mutex.Lock()
	c := sc.stratumclient
	sc.mutex.Unlock()
	stratumUser := sc.User
	/*
		{
			"id": 4,
			"method": "mining.submit",
			"params": [
				"RHkz1um1133mBZBU32ckcAKTY4wdJdCkdK.noname",
				"d785",
				"9707305d",
				"00000000000000000000000000000000000000000000000000000000",
				"fd4005010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
			]
		}
	*/
	strSubmit := []string{stratumUser, sj.JobID, nTime, encodedExtraNonce2, solutionStr}
	_, err = c.Call("mining.submit", strSubmit)
	if err != nil {
		sc.reject++
	} else {
		sc.accept++
		sc.lastAccept = time.Now().Unix()
	}
	return
}
