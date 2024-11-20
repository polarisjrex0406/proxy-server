package pkg

import (
	"io"
)

// ConnectionTracker - track and terminate request execution when user runs out of data
type ConnectionTracker interface {
	io.Closer
	//Watch - track and terminate request execution
	Watch(requestID string, purchaseUUID string, ch chan<- struct{}) (threads int64)

	//Stop - remove record and close request channel
	Stop(requestID string, purchaseUUID string) (threads int64)

	//Delete - remove record but do not send a signal to the request channel
	Delete(requestID string, purchaseUUID string) (threads int64)

	//Threads = return statistics of request execution by purchase uuid
	Threads() map[string]int64
}
