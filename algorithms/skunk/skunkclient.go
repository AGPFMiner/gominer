package skunk

import (
	"strings"

	"gominer/algorithms/generalstratum"
	"gominer/clients"
	"gominer/types"
)

// NewClient creates a new client given a '[stratum+tcp://]host:port' connectionstring
func NewClient(pool *types.Pool) (sc clients.Client) {
	sc = &generalstratum.StratumClient{Connectionstring: strings.TrimPrefix(pool.URL, "stratum+tcp://"), User: pool.User, Password: pool.Pass, Algo: pool.Algo}
	return
}
