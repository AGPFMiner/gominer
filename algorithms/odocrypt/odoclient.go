package odocrypt

import (
	"strings"

	"github.com/AGPFMiner/gominer/algorithms/generalstratum"
	"github.com/AGPFMiner/gominer/clients"
	"github.com/AGPFMiner/gominer/types"
)

// NewClient creates a new SiadClient given a '[stratum+tcp://]host:port' connectionstring
func NewClient(pool *types.Pool) (sc clients.Client) {
	sc = &generalstratum.StratumClient{Connectionstring: strings.TrimPrefix(pool.URL, "stratum+tcp://"), User: pool.User, Password: pool.Pass, Algo: pool.Algo}
	return
}
