package pkg

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"golang.org/x/sync/errgroup"
)

var (
	ErrConnectionClosed = errors.New("connection closed")
	ErrIPNotFound       = errors.New("ip not found")
	ErrInvalidTargeting = errors.New("invalid targeting")
)

func (p *Proxy) copy(account bool, done <-chan struct{}, _ string, src net.Conn, dst net.Conn) (err error) {
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
				break LOOP
			}
			if nw != nr {
				err = io.ErrShortWrite
				break LOOP
			}
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break LOOP
		}
	}

	if account && accounted > 0 {
		// err = p.config.Accountant.Decrement(password, accounted)
	}

	_ = dst.Close()
	return
}

func (p *Proxy) tunnel(_ *Purchase, request *Request, remote, conn net.Conn) error {
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
	} else {
		// p.deleteTracker(purchase, request)
	}

	return nil
}

func hasAccess(purchase *Purchase, request *Request) error {
	if len(purchase.IPs) > 0 {
		_, ok := purchase.IPs[request.UserIP]
		if !ok {
			return ErrIPNotFound
		}
	}

	if request.PurchaseType == PurchaseStatic && request.Country != nil {
		return ErrInvalidTargeting
	}

	if request.PurchaseType == PurchaseResidential && request.IP != nil {
		return ErrInvalidTargeting
	}

	return nil
}
