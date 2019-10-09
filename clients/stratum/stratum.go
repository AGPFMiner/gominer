//Package stratum implements the basic stratum protocol.
// This is normal jsonrpc but the go standard library is insufficient since we need features like notifications.
package stratum

import (
	"bufio"
	"encoding/json"
	"errors"
	"gominer/types"
	"log"
	"net"
	"sync"
	"time"
)

// request : A remote method is invoked by sending a request to the remote stratum service.
type request struct {
	Method interface{} `json:"method"`
	Params interface{} `json:"params"`
	ID     uint64      `json:"id"`
}

// response is the stratum server's response on a Request
// notification is an inline struct to easily decode messages in a response/notification using a json marshaller
type response struct {
	ID           uint64      `json:"id,omitempty"`
	Result       interface{} `json:"result,omitempty"`
	Error        interface{} `json:"error,omitempty"`
	notification `json:",inline,omitempty"`
}

// notification is a special kind of Request, it has no ID and is sent from the server to the client
type notification struct {
	Method interface{}   `json:"method"`
	Params []interface{} `json:"params"`
}

// func (n *notification) UnmarshalJSON(b []byte) error {
// 	var nTmp notification
// 	if err := json.Unmarshal(b, &nTmp); err != nil {
// 		log.Panicln(err)
// 		return err
// 	}

// 	n.Params = nTmp.Params

// 	switch nTmp.Method.(type) {
// 	case string:
// 		n.Method = n.Method.(string)
// 	case float64:
// 	case float32:
// 	case int:
// 	case uint:
// 	case int64:
// 	case uint64:
// 		n.Method = n.Method.(int)
// 	default:
// 		// n.Method = n.Method
// 	}
// 	return nil
// }

//ErrorCallback is the type of function that be registered to be notified of errors requiring a client
// to be dropped and a new one to be created
type ErrorCallback func(err error)

//NotificationHandler is the signature for a function that handles notifications
type NotificationHandler func(args []interface{}, result interface{})

// Client maintains a connection to the stratum server and (de)serializes requests/reponses/notifications
type Client struct {
	socket net.Conn

	seqmutex sync.Mutex // protects following
	seq      uint64

	callsMutex   sync.Mutex // protects following
	pendingCalls map[uint64]chan interface{}

	ErrorCallback        ErrorCallback
	notificationHandlers map[interface{}]NotificationHandler
	Veo                  bool
	poolstates           types.PoolConnectionStates
	feedDog              chan bool
}

func (c *Client) watchDog() {
	timeout := time.Second * 30
	if c.Veo {
		timeout = time.Second * 300
	}
	for {
		select {
		case <-time.After(timeout):
			c.poolstates = types.Sick

		case <-c.feedDog:
			c.poolstates = types.Alive
		}
	}
}

func (c *Client) PoolConnectionStates() types.PoolConnectionStates {
	return c.poolstates
}

//Dial connects to a stratum+tcp at the specified network address.
// This function is not threadsafe
// If an error occurs, it is both returned here and through the ErrorCallback of the Client
func (c *Client) Dial(host string) (err error) {
	c.poolstates = types.NotReady
	for try := 0; try < 6; try++ {
		c.socket, err = net.DialTimeout("tcp", host, time.Second*5)
		if err != nil {
			log.Print("TCP Dial err: ", err)
			continue
		} else {
			c.poolstates = types.Alive
			c.feedDog = make(chan bool, 1)
			go c.watchDog()
			go c.Listen()
			return
		}
	}
	err = errors.New("TCP Dial Failed, pool has been dead")
	log.Print(err)
	c.poolstates = types.Dead
	c.dispatchError(err)
	return
}

//Close releases the tcp connection
func (c *Client) Close() {
	if c.socket != nil {
		c.socket.Close()
	}
}

//SetNotificationHandler registers a function to handle notification for a specific method.
// This function is not threadsafe and all notificationhandlers should be set prior to calling the Dial function
func (c *Client) SetNotificationHandler(method interface{}, handler NotificationHandler) {
	if c.notificationHandlers == nil {
		c.notificationHandlers = make(map[interface{}]NotificationHandler)
	}
	c.notificationHandlers[method] = handler
}

func (c *Client) dispatchNotification(n notification, r interface{}) {
	// spew.Dump(c.notificationHandlers, n)
	if c.notificationHandlers == nil {
		return
	}

	var method interface{}
	switch n.Method.(type) {
	case string:
		method = n.Method.(string)
	case float64:
		method = int(n.Method.(float64))
	case float32:
		method = int(n.Method.(float32))
	case int:
		method = int(n.Method.(int))
	case uint:
		method = int(n.Method.(uint))
	case int64:
		method = int(n.Method.(int64))
	case uint64:
		method = int(n.Method.(uint64))
	default:
		// n.Method = n.Method
	}
	// spew.Dump(n.Method, method)
	if notificationHandler, exists := c.notificationHandlers[method]; exists {
		notificationHandler(n.Params, r)
	}
}

func (c *Client) dispatch(r response) {
	if r.ID == 0 {
		c.dispatchNotification(r.notification, r.Result)
		return
	}
	c.callsMutex.Lock()
	defer c.callsMutex.Unlock()
	cb, found := c.pendingCalls[r.ID]
	var result interface{}
	if r.Error != nil {
		message := "Oops"
		message, _ = r.Error.(string)
		result = errors.New(message)
	} else {
		result = r.Result
	}
	if found {
		cb <- result
	}
}

func (c *Client) dispatchError(err error) {
	if c.ErrorCallback != nil {
		c.ErrorCallback(err)
	}
}

//Listen reads data from the open connection, deserializes it and dispatches the reponses and notifications
// This is a blocking function and will continue to listen until an error occurs (io or deserialization)
func (c *Client) Listen() {
	reader := bufio.NewReader(c.socket)
	for {
		rawmessage, err := reader.ReadString('\n')
		c.feedDog <- true
		if err != nil {
			c.poolstates = types.Sick
			c.dispatchError(err)
			return
		}
		r := response{}
		// log.Print("[Stratum <---]", rawmessage)
		err = json.Unmarshal([]byte(rawmessage), &r)
		// log.Println(err)
		// spew.Dump(r)
		if err != nil {
			c.poolstates = types.Sick
			c.dispatchError(err)
			return
		}
		c.poolstates = types.Alive
		c.dispatch(r)
	}
}

func (c *Client) registerRequest(requestID uint64) (cb chan interface{}) {
	c.callsMutex.Lock()
	defer c.callsMutex.Unlock()
	if c.pendingCalls == nil {
		c.pendingCalls = make(map[uint64]chan interface{})
	}
	cb = make(chan interface{})
	c.pendingCalls[requestID] = cb
	return
}

func (c *Client) cancelRequest(requestID uint64) {
	c.callsMutex.Lock()
	defer c.callsMutex.Unlock()
	cb, found := c.pendingCalls[requestID]
	if found {
		close(cb)
		delete(c.pendingCalls, requestID)
	}
}

//Call invokes the named function, waits for it to complete, and returns its error status.
func (c *Client) Call(serviceMethod interface{}, args interface{}) (reply interface{}, err error) {
	r := request{Method: serviceMethod, Params: args}

	if c.Veo {
		r.ID = 2
	} else {
		c.seqmutex.Lock()
		c.seq++
		r.ID = c.seq
		c.seqmutex.Unlock()
	}

	rawmsg, err := json.Marshal(r)
	if err != nil {
		return
	}
	call := c.registerRequest(r.ID)
	defer c.cancelRequest(r.ID)

	rawmsg = append(rawmsg, []byte("\n")...)
	_, err = c.socket.Write(rawmsg)
	// log.Print("[Stratum --->]", string(rawmsg), "err:", err)
	if err != nil {
		c.poolstates = types.Sick
		log.Print("Socket Write Error:", err)
		return
	}
	//Make sure the request is cancelled if no response is given
	go func() {
		time.Sleep(10 * time.Second)
		c.cancelRequest(r.ID)
	}()
	reply = <-call

	if reply == nil {
		err = errors.New("Timeout")
		return
	}
	err, _ = reply.(error)
	return
}
