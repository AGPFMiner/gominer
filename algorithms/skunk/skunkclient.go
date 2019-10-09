package skunk

import (
	"strings"

	"github.com/dynm/gominer/algorithms/generalstratum"
	"github.com/dynm/gominer/clients"
	"github.com/dynm/gominer/types"
)

// NewClient creates a new client given a '[stratum+tcp://]host:port' connectionstring
func NewClient(pool *types.Pool) (sc clients.Client) {
	sc = &generalstratum.StratumClient{Connectionstring: strings.TrimPrefix(pool.URL, "stratum+tcp://"), User: pool.User, Password: pool.Pass, Algo: pool.Algo}
	return
}
