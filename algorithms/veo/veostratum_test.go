package veo

import (
	"github.com/dynm/gominer/clients/stratum"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
)

func TestGetHeaderForWork(t *testing.T) {
	cw := NewClient("stratum+tcp://stratum.amoveopool.com:8822", "BDnSmWXuhuaANFe2vSWo4q+nnPAnFIZ/MIiDnUYh8s3MsmgPAjVh5CUrAUArVsFBrRgCtlVyXFEoLLKnADd+0oU=.2", -1)
	cw.SetDeprecatedJobCall(func(jobid string) {

	})
	cw.Start()
	for {
		_, _, header, _, job, err := cw.GetHeaderForWork()
		time.Sleep(100 * time.Millisecond)
		if err != nil {
			t.Log(err)
			continue
		}
		t.Logf("Job: \n%v", job)
		t.Logf("Header: %02X\n", header)
		t.Logf("RevHeader: %02X\n", stratum.RevHash(header))
		spew.Dump(job)
		break
	}
}
