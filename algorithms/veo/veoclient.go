package veo

import (
	"strings"

	"github.com/dynm/gominer/clients"
	"github.com/dynm/gominer/types"
)

// NewClient creates a new SiadClient given a '[stratum+tcp://]host:port' connectionstring
func NewClient(pool *types.Pool) (sc clients.Client) {
	sc = &StratumClient{connectionstring: strings.TrimPrefix(pool.URL, "stratum+tcp://"), User: pool.User, Algo: pool.Algo}
	return
}
