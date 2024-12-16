package pkg

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

var (
	ErrConnectionClosed = errors.New("connection closed")
	ErrIPNotFound       = errors.New("ip not found")
	ErrInvalidTargeting = errors.New("invalid targeting")
)

func (p *Proxy) copy(account bool, done <-chan struct{}, password string, src net.Conn, dst net.Conn) (err error) {
	buf := make([]byte, p.config.BufferSize)

	var accounted, written int64
LOOP:
	for {
		select {
		case <-done:
			err = ErrConnectionClosed
			break LOOP
		default:
		}

		if p.config.ReadDeadline > 0 {
			if err = src.SetReadDeadline(time.Now().Add(p.config.ReadDeadline)); err != nil {
				fmt.Println("err =", err)
				break LOOP
			}
		}

		nr, er := src.Read(buf)
		accounted += int64(nr)

		if account && accounted >= p.config.AccountBytes {
			// err = p.config.Accountant.Decrement(password, accounted)
			accounted = 0
		}

		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				fmt.Println("ew =", ew)
				break LOOP
			}
			if nw != nr {
				err = io.ErrShortWrite
				fmt.Println("nw & nr =", err)
				break LOOP
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			fmt.Printf("er = %v from %v\n", er, src.RemoteAddr().String())
			break LOOP
		}
	}

	if account && accounted > 0 {
		// err = p.config.Accountant.Decrement(password, accounted)
	}

	_ = dst.Close()
	fmt.Println("Connection closed:", dst.RemoteAddr().String())
	return
}

func (p *Proxy) tunnel(purchase *Purchase, request *Request, remote, conn net.Conn) error {
	accountData := request.IP == nil

	g, _ := errgroup.WithContext(context.Background())
	g.Go(func() error {
		return p.copy(accountData, request.Done, request.Password, conn, remote) //nolint:errcheck
	})
	g.Go(func() error {
		return p.copy(accountData, request.Done, request.Password, remote, conn) //nolint:errcheck
	})

	if err := g.Wait(); err != ErrConnectionClosed {
		// p.stopTracker(purchase, request)
		fmt.Println("err =", err)
		fmt.Println("p.stopTracker(purchase, request)")
	} else {
		// p.deleteTracker(purchase, request)
		fmt.Println("p.deleteTracker(purchase, request)")
	}

	return nil
}

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
