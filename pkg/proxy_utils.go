package pkg

import (
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"time"
)

var (
	ErrConnectionClosed = errors.New("connection closed")
	ErrIPNotFound       = errors.New("ip not found")
	ErrInvalidTargeting = errors.New("invalid targeting")
)

type countConn struct {
	net.Conn
	written int64
	read    int64
}

func (c *countConn) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)
	atomic.AddInt64(&c.read, int64(n))
	return n, err
}

func (c *countConn) Write(b []byte) (n int, err error) {
	n, err = c.Conn.Write(b)
	atomic.AddInt64(&c.written, int64(n))
	return n, err
}

func (c *countConn) Transferred() int64 {
	return atomic.LoadInt64(&c.read) + atomic.LoadInt64(&c.written)
}

func hasAccess(purchase *Purchase, request *Request) error {
	if len(purchase.IPs) > 0 {
		_, ok := purchase.IPs[request.UserIP]
		if !ok {
			return ErrIPNotFound
		}
	}

	if request.PurchaseType == PurchaseStatic && (request.Country != nil || request.Region != nil || request.City != nil) {
		return ErrInvalidTargeting
	}

	if request.PurchaseType == PurchaseResidential && request.IP != nil {
		return ErrInvalidTargeting
	}

	return nil
}

func dialUpstreamHTTP(request *Request, timeout time.Duration) (*countConn, error) {
	hostname, _, _, credentials, err := request.Provider.Credentials(request) // FIXME looks awkward
	if err != nil {
		return nil, err
	}

	fmt.Println("dialUpstreamHTTP() called:")
	fmt.Println("hostname =", hostname)
	fmt.Println("credentials =", credentials)
	fmt.Println("dialUpstreamHTTP() ended:")

	// if len(credentials) > 0 {
	// 	ctx.Request.Header.Set(fasthttp.HeaderProxyAuthorization, "Basic "+zerocopy.String(credentials))
	// }
	// ctx.Request.SetRequestURIBytes(ctx.RequestURI())

	// conn, err := fasthttp.DialTimeout(hostname, timeout)
	// if err != nil {
	// 	return nil, err
	// }

	// return &countConn{conn, 0, 0}, nil
	return nil, nil
}
