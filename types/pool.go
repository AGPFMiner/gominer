package types

type Pool struct {
	URL    string `json:"url"`
	User   string `json:"user"`
	Pass   string `json:"pass"`
	Algo   string `json:"algo"`
	Active bool   `json:"active,omitempty"`
}

type PoolConnectionStates int

const (
	NotReady PoolConnectionStates = iota + 1
	Alive
	Sick
	Dead
)

type PoolStates struct {
	Status       PoolConnectionStates `json:"status"`
	User         string               `json:"user"`
	PoolAddr     string               `json:"pooladdr"`
	Algo         string               `json:"algo"`
	Accept       int32                `json:"accept"`
	Reject       int32                `json:"reject"`
	Discard      int32                `json:"discard"`
	Diff         float64              `json:"diff"`
	LastAccepted int64                `json:"lastaccepted"`
	Active       bool                 `json:"active"`
}

type HardwareStats int

const (
	Programming HardwareStats = iota + 1
	Running
	NoResponse
	Stopped
)

type DriverStates struct {
	DriverName  string          `json:"name"`
	Status      HardwareStats   `json:"status"`
	Temperature string          `json:"temperature"`
	Voltage     string          `json:"voltage"`
	NonceNum    [3]float64      `json:"noncenum"`
	Hashrate    [3]float64      `json:"hashrate"`
	NonceStats  *map[int]uint64 `json:"nonestats"`
	Algo        string          `json:"algo"`
}

type ScriptaMinerStatus struct {
	Devs      []*DriverStates `json:"devs"`
	MinerDown bool            `json:"minerDown"`
	MinerUp   bool            `json:"minerUp"`
	Pools     []*PoolStates   `json:"pools"`
	Time      int64           `json:"time"`
}
type ScriptaStatus struct {
	Status *ScriptaMinerStatus `json:"status"`
}
