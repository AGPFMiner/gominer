package xdag

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"gominer/clients"
	"gominer/types"
)

// NewClient creates a new XdagClient given a '[stratum+tcp://]host:port' connectionstring
func NewClient(pool *types.Pool) (sc clients.Client) {
	s := XdagClient{}
	s.connectionstring = pool.URL
	s.pooluser = pool.User
	s.Algo = pool.Algo
	s.siadurl = "http://127.0.0.1:1234"
	sc = &s
	return
}

// XdagClient is a simple client to a siad
type XdagClient struct {
	siadurl                 string
	Algo                    string
	pooluser                string
	connectionstring        string
	accept, reject, discard int
	lastAccept              int64
}

func (sc *XdagClient) GetPoolStats() (info types.PoolStates) {
	info.Status = sc.PoolConnectionStates()
	info.User = sc.pooluser
	info.PoolAddr = sc.siadurl
	info.Algo = sc.Algo
	info.Accept, info.Reject = sc.accept, sc.reject
	info.Diff = -1
	info.LastAccepted = sc.lastAccept
	return
}

func (sc *XdagClient) AlgoName() string {
	return sc.Algo
}

func (sc *XdagClient) PoolConnectionStates() types.PoolConnectionStates {
	return types.Alive
}

//Start does nothing
func (sc *XdagClient) Start() {
	exec.Command("killall", "xdag").Run()
	time.Sleep(time.Millisecond * 500)
	workername := "AGPFminer"
	pooluser := sc.pooluser
	splited := strings.Split(sc.pooluser, ".")
	if len(splited) == 2 {
		pooluser = splited[0]
		workername = splited[1]
	}
	go func() {
		log.Println("CMD:", "xdag", "-F", "-p", sc.connectionstring, "-a", pooluser, "-w", workername)
		cmd := exec.Command("xdag", "-F", "-p", sc.connectionstring, "-a", pooluser, "-w", workername)
		cmd.Run()
		log.Panic("Xdag RPC crashed")
	}()
	time.Sleep(time.Second * 20)
}

//SetDeprecatedJobCall does nothing
func (sc *XdagClient) SetDeprecatedJobCall(call clients.DeprecatedJobCall) {}

//GetHeaderForWork fetches new work from the SIA daemon
func (sc *XdagClient) GetHeaderForWork() (target []byte, difficulty float64, header []byte, deprecationChannel chan bool, job interface{}, err error) {
	//the deprecationChannel is not used but return a valid channel anyway
	deprecationChannel = make(chan bool)

	client := &http.Client{}

	req, err := http.NewRequest("GET", sc.siadurl+"/getWork", nil)
	if err != nil {
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	// 061065683691061195ffa6d10612b5b0696d0613743290ef06144ce6e3520615e45e43410616f22fe5d50617fe87f6ed06186af489610619067fd601061a19891f7f061b1025e420061cdbd71408061dfc508a70061e12042972061f467c846806205d41a71c06211699cd5406229d212ac00623dbb05c6a0624b04dc76c0625db8065d2
	if len(buf) < 288 {
		err = fmt.Errorf("Invalid response, only received %d bytes", len(buf))
		return
	}

	// target = buf[:32]
	header, _ = hex.DecodeString(string(buf))

	return
}

//SubmitHeader reports a solved header to the SIA daemon
func (sc *XdagClient) SubmitHeader(nonce []byte, job interface{}) (err error) {
	nonceLen := len(nonce)
	if nonceLen < 144 {
		err = errors.New("Wrong Nonce Len")
		return
	}
	nonceStrip := nonce[nonceLen-8 : nonceLen]
	log.Printf("%02X\n", nonceStrip)
	req, err := http.NewRequest("POST", sc.siadurl+"/submit", bytes.NewBufferString(hex.EncodeToString(nonceStrip)))
	if err != nil {
		return
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	log.Print("xdag resp:", string(buf))
	if err != nil {
		sc.reject++
	} else {
		sc.accept++
		sc.lastAccept = time.Now().Unix()
	}
	return
}
