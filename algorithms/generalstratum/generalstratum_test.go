package generalstratum

import (
	"encoding/hex"
	"github.com/AGPFMiner/gominer/types"
	"strconv"
	"strings"
	"testing"
)

func TestDifficultyToTarget(t *testing.T) {
	diff, _ := strconv.ParseFloat("65.32477875", 64)

	expectedTarget := "0x00000000000404CB000000000000000000000000000000000000000000000000"

	target, err := difficultyToTarget(diff)
	if err != nil {
		t.Error(err)
	}

	if expectedTarget != ("0x" + hex.EncodeToString(target[:])) {
		t.Error("0x"+hex.EncodeToString(target[:]), "returned instead of", expectedTarget)
	}
}

var odoPool = &types.Pool{
	URL:  "stratum+tcp://dgb-odocrypt.f2pool.com:11115",
	User: "DEesW1UoEAUtM8mrwGHjfz1gdwPwqqRPzJ",
	Pass: "x",
	Algo: "odocrypt",
}

func TestGetHeaderForWork(t *testing.T) {
	pool := odoPool
	cw := &StratumClient{Connectionstring: strings.TrimPrefix(pool.URL, "stratum+tcp://"), User: pool.User, Password: pool.Pass}
	cw.SetDeprecatedJobCall(func(jobid string) {

	})
	cw.Start()

	for {
		_, _, header, _, job, err := cw.GetHeaderForWork()
		if err != nil {
			// t.Log(err)
			continue
		}
		t.Logf("Job: \n%v", job)
		t.Logf("Header: %02X\n", header)
		// t.Logf("RevHeader: %02X\n", stratum.RevHash(header))
		// spew.Dump(job)
		// break
	}
}
