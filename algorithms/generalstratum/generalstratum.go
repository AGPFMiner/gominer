package generalstratum

import (
	"encoding/hex"
	"errors"
	"log"
	"math/big"
	"reflect"
	"sync"
	"time"

	"github.com/dynm/gominer/clients"
	"github.com/dynm/gominer/clients/stratum"
	"github.com/dynm/gominer/types"
)

const (
	//HashSize is the length of a groestl hash
	HashSize = 32
)

//Target declares what a solution should be smaller than to be accepted
type Target [HashSize]byte

type StratumJob struct {
	JobID        string
	PrevHash     []byte
	Coinbase1    []byte
	Coinbase2    []byte
	MerkleBranch [][]byte
	Version      []byte
	NBits        []byte
	NTime        []byte
	CleanJobs    bool
	ExtraNonce2  stratum.ExtraNonce2
}

//StratumClient is a client using the stratum protocol
type StratumClient struct {
	Connectionstring        string
	User                    string
	Password                string
	Algo                    string
	accept, reject, discard int
	lastAccept              int64

	mutex           sync.Mutex // protects following
	stratumclient   *stratum.Client
	extranonce1     []byte
	extranonce2Size uint
	target          Target
	Difficulty      float64
	currentJob      StratumJob
	clients.BaseClient
}

func (sc *StratumClient) GetPoolStats() (info types.PoolStates) {
	info.Status = sc.PoolConnectionStates()
	info.User = sc.User
	info.PoolAddr = "stratum+tcp://" + sc.Connectionstring
	info.Algo = sc.Algo
	info.Accept, info.Reject, info.Discard = sc.accept, sc.reject, sc.discard
	info.Diff = sc.Difficulty
	info.LastAccepted = sc.lastAccept
	return
}

func (sc *StratumClient) AlgoName() string {
	return sc.Algo
}

func (sc *StratumClient) Start() {
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
					stratumclient.Close()
					time.Sleep(30)
					sc.startPoolConn()
					// fallthrough
				case types.Sick:
					log.Print("Pool sick, reconnecting")
					stratumclient.Close()
					sc.startPoolConn()
				}
			}
		}
	}
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
	err := sc.stratumclient.Dial(sc.Connectionstring)
	if err != nil {
		return
	}

	//Subscribe for mining
	//Close the connection on an error will cause the client to generate an error, resulting in te errorhandler to be triggered
	result, err := sc.stratumclient.Call("mining.subscribe", []string{"AGPFminer"})
	if err != nil {
		log.Println("ERROR Error in response from stratum:", err)
		return
	}
	reply, ok := result.([]interface{})
	if !ok || len(reply) < 3 {
		log.Println("ERROR Invalid response from stratum:", result)
		return
	}

	//Keep the extranonce1 and extranonce2_size from the reply
	if sc.extranonce1, err = stratum.HexStringToBytes(reply[1]); err != nil {
		log.Println("ERROR Invalid extrannonce1 from startum")
		return
	}

	extranonce2Size, ok := reply[2].(float64)
	if !ok {
		log.Println("ERROR Invalid extranonce2_size from stratum", reply[2], "type", reflect.TypeOf(reply[2]))
		return
	}
	sc.extranonce2Size = uint(extranonce2Size)

	//Authorize the miner
	go func() {
		result, err = sc.stratumclient.Call("mining.authorize", []string{sc.User, sc.Password})
		if err != nil {
			log.Println("Unable to authorize:", err)
			return
		}
		log.Println("Authorization of", sc.User, ":", result)
	}()

}

func (sc *StratumClient) subscribeToStratumDifficultyChanges() {
	sc.stratumclient.SetNotificationHandler("mining.set_difficulty", func(params []interface{}, result interface{}) {
		if params == nil || len(params) < 1 {
			log.Println("ERROR No difficulty parameter supplied by stratum server")
			return
		}
		diff, ok := params[0].(float64)
		if !ok {
			log.Println("ERROR Invalid difficulty supplied by stratum server:", params[0])
			return
		}
		log.Println("Stratum server changed difficulty to", diff)
		sc.setDifficulty(diff)
		sc.Difficulty = diff
	})
}

func (sc *StratumClient) subscribeToStratumJobNotifications() {
	sc.stratumclient.SetNotificationHandler("mining.notify", func(params []interface{}, result interface{}) {
		// log.Println("New job received from stratum server")
		if params == nil || len(params) < 9 {
			log.Println("ERROR Wrong number of parameters supplied by stratum server")
			return
		}

		sj := StratumJob{}

		sj.ExtraNonce2.Size = sc.extranonce2Size

		var ok bool
		var err error
		if sj.JobID, ok = params[0].(string); !ok {
			log.Println("ERROR Wrong job_id parameter supplied by stratum server")
			return
		}
		if sj.PrevHash, err = stratum.HexStringToBytes(params[1]); err != nil {
			log.Println("ERROR Wrong prevhash parameter supplied by stratum server")
			return
		}
		if sj.Coinbase1, err = stratum.HexStringToBytes(params[2]); err != nil {
			log.Println("ERROR Wrong coinb1 parameter supplied by stratum server")
			return
		}
		if sj.Coinbase2, err = stratum.HexStringToBytes(params[3]); err != nil {
			log.Println("ERROR Wrong coinb2 parameter supplied by stratum server")
			return
		}

		//Convert the merklebranch parameter
		merklebranch, ok := params[4].([]interface{})
		if !ok {
			log.Println("ERROR Wrong merkle_branch parameter supplied by stratum server")
			return
		}
		sj.MerkleBranch = make([][]byte, len(merklebranch), len(merklebranch))
		for i, branch := range merklebranch {
			if sj.MerkleBranch[i], err = stratum.HexStringToBytes(branch); err != nil {
				log.Println("ERROR Wrong merkle_branch parameter supplied by stratum server")
				return
			}
		}

		if sj.Version, err = stratum.HexStringToBytes(params[5]); err != nil {
			log.Println("ERROR Wrong version parameter supplied by stratum server")
			return
		}
		if sj.NBits, err = stratum.HexStringToBytes(params[6]); err != nil {
			log.Println("ERROR Wrong nbits parameter supplied by stratum server")
			return
		}
		if sj.NTime, err = stratum.HexStringToBytes(params[7]); err != nil {
			log.Println("ERROR Wrong ntime parameter supplied by stratum server")
			return
		}
		if sj.CleanJobs, ok = params[8].(bool); !ok {
			log.Println("ERROR Wrong clean_jobs parameter supplied by stratum server")
			return
		}
		sc.addNewStratumJob(sj)
	})
}

func (sc *StratumClient) addNewStratumJob(sj StratumJob) {
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
	var target Target
	var err error
	target, err = difficultyToTarget(difficulty)

	if err != nil {
		log.Println("ERROR Error setting difficulty to ", difficulty)
	}
	sc.DeprecateOutstandingJobs()
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
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

	arbtx := []byte{}
	arbtx = append(arbtx, sc.currentJob.Coinbase1...)
	arbtx = append(arbtx, sc.extranonce1...)
	arbtx = append(arbtx, en2...)
	arbtx = append(arbtx, sc.currentJob.Coinbase2...)

	coinbaseHash := stratum.SHA256d(arbtx)

	//Construct the merkleroot from the arbitrary transaction and the merklebranches
	merkleRoot := coinbaseHash
	for _, h := range sc.currentJob.MerkleBranch {
		m := append(merkleRoot, h...)
		merkleRoot = stratum.SHA256d(m)
	}

	//Construct the header
	header = make([]byte, 0, 80+HashSize)
	header = append(header, sc.currentJob.Version...) //version
	header = append(header, sc.currentJob.PrevHash...)
	header = append(header, stratum.RevHash(merkleRoot)...)
	header = append(header, sc.currentJob.NTime...)
	header = append(header, sc.currentJob.NBits...)
	header = append(header, []byte{0, 0, 0, 0}[:]...) //empty nonce
	header = stratum.RevHash(header)
	header = append(header, sc.target[:]...)
	return
}

//SubmitHeader reports a solution to the stratum server
func (sc *StratumClient) SubmitHeader(nonce []byte, job interface{}) (err error) {
	sj, _ := job.(StratumJob)
	// nonce := hex.EncodeToString(stratum.RevBytes(header[84:88]))
	nonceStr := hex.EncodeToString(nonce[4:])
	encodedExtraNonce2 := hex.EncodeToString(sj.ExtraNonce2.Bytes())
	nTime := hex.EncodeToString(sj.NTime)
	sc.mutex.Lock()
	c := sc.stratumclient
	sc.mutex.Unlock()
	stratumUser := sc.User
	strSubmit := []string{stratumUser, sj.JobID, encodedExtraNonce2, nTime, nonceStr}
	_, err = c.Call("mining.submit", strSubmit)
	if err != nil {
		sc.reject++
	} else {
		sc.accept++
		sc.lastAccept = time.Now().Unix()
	}
	return
}
