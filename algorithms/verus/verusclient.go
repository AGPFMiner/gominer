package verus

import (
	"strings"

	"github.com/AGPFMiner/gominer/clients"
	"github.com/AGPFMiner/gominer/types"
)

// NewClient creates a new SiadClient given a '[stratum+tcp://]host:port' connectionstring
func NewClient(pool *types.Pool) (sc clients.Client) {
	sc = &StratumClient{connectionstring: strings.TrimPrefix(pool.URL, "stratum+tcp://"), User: pool.User, Password: pool.Pass, Algo: pool.Algo}
	return
}
