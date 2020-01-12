package ckb

import (
	"log"
	"strings"
	"testing"
	"time"

	"github.com/AGPFMiner/gominer/types"
)

var ckbPool = &types.Pool{
	URL:  "stratum+tcp://ckb.stratum.hashpool.com:4300",
	User: "ckb1qyq8fxuxz49nvatawuqye0fydpm4gulcs6usgyfkrr.1",
	Pass: "x",
	Algo: "ckb",
}

func TestGetHeaderForWork(t *testing.T) {
	pool := ckbPool
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
}

func TestCKBStratum(t *testing.T) {
	pool := ckbPool

	sc := &StratumClient{Connectionstring: strings.TrimPrefix(pool.URL, "stratum+tcp://"), User: pool.User, Password: pool.Pass}
	sc.Start()
}
