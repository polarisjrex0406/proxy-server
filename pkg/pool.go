package pkg

import (
	"sync"
)

var (
	requestPool sync.Pool
)

func acquireRequest() *Request {
	v := requestPool.Get()
	if v == nil {
		return &Request{}
	}
	return v.(*Request)
}

func releaseRequest(req *Request) {
	req.reset()
	requestPool.Put(req)
}
