package veriblock

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/dynm/gominer/clients"
)

// NewClient creates a new SiadClient given a '[stratum+tcp://]host:port' connectionstring
func NewClient(connectionstring, pooluser, password string, depth int) (vbk clients.Client) {
	vbk = &VBKClient{
		pooladdr: connectionstring,
		username: pooluser,
		password: password,
	}

	return
}

// SiadClient is a simple client to a siad
type VBKClient struct {
	pooladdr string
	username string
	password string
	clients.BaseClient
}

func decodeMessage(resp *http.Response) (msg string, err error) {
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	var data struct {
		Message string `json:"message"`
	}
	if err = json.Unmarshal(buf, &data); err == nil {
		msg = data.Message
	}
	return
}

//Start does nothing
func (vbk *VBKClient) Start() {
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
	if !ok || len(reply) < 3 {
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

	extranonce2Size, ok := reply[2].(float64)
	if !ok {
		log.Println("ERROR Invalid extranonce2_size from stratum", reply[2], "type", reflect.TypeOf(reply[2]))
		sc.stratumclient.Close()
		return
	}
	sc.extranonce2Size = uint(extranonce2Size)

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

//SetDeprecatedJobCall does nothing
func (vbk *VBKClient) SetDeprecatedJobCall(call clients.DeprecatedJobCall) {}

//GetHeaderForWork fetches new work from the SIA daemon
func (vbk *VBKClient) GetHeaderForWork() (target []byte, difficulty float64, header []byte, deprecationChannel chan bool, job interface{}, err error) {
	//the deprecationChannel is not used but return a valid channel anyway
	deprecationChannel = make(chan bool)

	client := &http.Client{}

	req, err := http.NewRequest("GET", sc.siadurl, nil)
	if err != nil {
		return
	}

	req.Header.Add("User-Agent", "Sia-Agent")
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case 200:
	case 400:
		msg, errd := decodeMessage(resp)
		if errd != nil {
			err = fmt.Errorf("Status code %d", resp.StatusCode)
		} else {
			err = fmt.Errorf("Status code %d, message: %s", resp.StatusCode, msg)
		}
		return
	default:
		err = fmt.Errorf("Status code %d", resp.StatusCode)
		return
	}
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if len(buf) < 112 {
		err = fmt.Errorf("Invalid response, only received %d bytes", len(buf))
		return
	}

	target = buf[:32]
	header = buf[32:112]

	return
}

//SubmitHeader reports a solved header to the SIA daemon
func (vbk *VBKClient) SubmitHeader(header []byte, job interface{}) (err error) {
	req, err := http.NewRequest("POST", sc.siadurl, bytes.NewReader(header))
	if err != nil {
		return
	}

	req.Header.Add("User-Agent", "Sia-Agent")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	switch resp.StatusCode {
	case 204:
	default:
		msg, errd := decodeMessage(resp)
		if errd != nil {
			err = fmt.Errorf("Status code %d", resp.StatusCode)
		} else {
			err = fmt.Errorf("%s", msg)
		}
		return
	}
	return
}
