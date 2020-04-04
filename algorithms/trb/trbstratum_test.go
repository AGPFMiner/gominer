package trb

import (
	"log"
	"strings"
	"testing"
	"time"

	"github.com/AGPFMiner/gominer/types"
)

var trbPool = &types.Pool{
	URL:  "stratum+tcp://trb.uupool.cn:11002",
	User: "0x23aebde41bab8a5688422582d0faecbf0f84bf67.1",
	Pass: "x",
	Algo: "trb",
}

func TestGetHeaderForWork(t *testing.T) {
	pool := trbPool
	c := &StratumClient{Connectionstring: strings.TrimPrefix(pool.URL, "stratum+tcp://"), User: pool.User, Password: pool.Pass}
	c.SetDeprecatedJobCall(func(jobid string) {

	})
	go c.Start()

	for {
		_, _, header, _, job, err := c.GetHeaderForWork()
		if err != nil {
			continue
		}
		log.Printf("header: %02x, err: %v, job: %v\n", header, err, job)

		time.Sleep(500 * time.Millisecond)
	}
	//4c2adcc59d69789a5d37d5ba95a81c76eb117b86335036b145ed08a8801ea4d0
	//7f97009879cbbbcbd6ca0ced94644d25be4bef15
	//4d062a38
	//00000004
}

func TestTRBStratum(t *testing.T) {
	pool := trbPool

	sc := &StratumClient{Connectionstring: strings.TrimPrefix(pool.URL, "stratum+tcp://"), User: pool.User, Password: pool.Pass}
	sc.Start()
}
