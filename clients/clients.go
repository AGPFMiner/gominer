//Package clients provides some utilities and common code for specific client implementations
package clients

import "github.com/dynm/gominer/types"

//HeaderReporter defines the required method a Groestl client or pool client should implement for miners to be able to report solved headers
type HeaderReporter interface {
	//SubmitHeader reports a solved header
	SubmitHeader(nonce []byte, job interface{}) (err error)
}

//HeaderProvider supplies headers for a miner to mine on
type HeaderProvider interface {
	//GetHeaderForWork providers a header to mine on
	// the deprecationChannel is closed when the job should be abandoned
	GetHeaderForWork() (target []byte, difficulty float64, header []byte, deprecationChannel chan bool, job interface{}, err error)
}

//DeprecatedJobCall is a function that can be registered on a client to be executed when
// the server indicates that all previous jobs should be abandoned
type DeprecatedJobCall func(jobid string)

//CleanJobEventCall is a function that can be registered on a client to be executed when
// cleanJob is true
type CleanJobEventCall func()

// Client defines the interface for a client towards a work provider
type Client interface {
	HeaderProvider
	HeaderReporter
	Start()
	AlgoName() (algo string)
	PoolConnectionStates() (stats types.PoolConnectionStates)
	GetPoolStats() (stats types.PoolStates)
	SetDeprecatedJobCall(call DeprecatedJobCall)
	SetCleanJobEventCall(call CleanJobEventCall)
}

//BaseClient implements some common properties and functionality
type BaseClient struct {
	deprecationChannels map[string]chan bool

	deprecatedJobCall DeprecatedJobCall
	cleanJobEventCall CleanJobEventCall
}

//DeprecateOutstandingJobs closes all deprecationChannels and removes them from the list
// This method is not threadsafe
func (sc *BaseClient) DeprecateOutstandingJobs() {
	if sc.deprecationChannels == nil {
		sc.deprecationChannels = make(map[string]chan bool)
	}

	call := sc.deprecatedJobCall

	for jobid, deprecatedJob := range sc.deprecationChannels {
		close(deprecatedJob)
		delete(sc.deprecationChannels, jobid)
		if call != nil {
			go call(jobid)
		}
	}
	cleanJobEventCall := sc.cleanJobEventCall
	if cleanJobEventCall != nil {
		go cleanJobEventCall()
	}
}

// AddJobToDeprecate add the jobid to the list of jobs that should be deprecated when the times comes
func (sc *BaseClient) AddJobToDeprecate(jobid string) {
	sc.deprecationChannels[jobid] = make(chan bool)
}

// GetDeprecationChannel return the channel that will be closed when a job gets deprecated
func (sc *BaseClient) GetDeprecationChannel(jobid string) chan bool {
	return sc.deprecationChannels[jobid]
}

//SetDeprecatedJobCall sets the function to be called when the previous jobs should be abandoned
func (sc *BaseClient) SetDeprecatedJobCall(call DeprecatedJobCall) {
	sc.deprecatedJobCall = call
}

//SetCleanJobEventCall sets the function to be called when the previous jobs should be abandoned
func (sc *BaseClient) SetCleanJobEventCall(call CleanJobEventCall) {
	sc.cleanJobEventCall = call
}
