package pkg

import (
	"io"
)

// ConnectionTracker - track and terminate request execution when user runs out of data
type ConnectionTracker interface {
	io.Closer
	//Watch - track and terminate request execution
	Watch(requestID string, purchaseID uint, ch chan<- struct{}) (threads int64)

	//Stop - remove record and close request channel
	Stop(requestID string, purchaseID uint) (threads int64)

	//Delete - remove record but do not send a signal to the request channel
	Delete(requestID string, purchaseID uint) (threads int64)

	//Threads = return statistics of request execution by purchase uuid
	Threads() map[uint]int64
}
