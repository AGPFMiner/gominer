package verus

import (
	"encoding/hex"
	"strconv"
	"testing"

	"github.com/davecgh/go-spew/spew"
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

//pool.vrsc.52hash.com:18888 -u RHkz1um1133mBZBU32ckcAKTY4wdJdCkdK.noname -t 1
func TestGetHeaderForWork(t *testing.T) {
	cw := NewClient("stratum+tcp://pool.vrsc.52hash.com:18888", "RHkz1um1133mBZBU32ckcAKTY4wdJdCkdK", "x", -1)
	cw.SetDeprecatedJobCall(func(jobid string) {

	})
	cw.Start()
	for {
		_, _, header, _, job, err := cw.GetHeaderForWork()
		if err != nil {
			t.Log(err)
			continue
		}
		t.Logf("Job: \n%v", job)
		t.Logf("Header: %02X\n", header)
		spew.Dump(job)
		break
	}
}
