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
	ErrConnectionClosed   = errors.New("connection closed")
	ErrIPNotFound         = errors.New("ip not found")
	ErrInvalidTargeting   = errors.New("invalid targeting")
	ErrStickyNotSupported = errors.New("sticky not supported")
)

func (p *Proxy) copy(purchase *Purchase, account bool, isRead bool, done <-chan struct{}, password string, src net.Conn, dst net.Conn) (err error) {
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

		if accounted >= p.config.AccountBytes {
			if isRead {
				err = p.config.Measure.IncReadBytes(password, accounted)
			} else {
				err = p.config.Measure.IncWriteBytes(password, accounted)
			}

			if purchase.BandwidthLimited && account {
				err = p.config.Accountant.Decrement(password, accounted)
			}

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

	if accounted >= 0 {
		if isRead {
			err = p.config.Measure.IncReadBytes(password, accounted)
		} else {
			err = p.config.Measure.IncWriteBytes(password, accounted)
		}

		if purchase.BandwidthLimited && account {
			err = p.config.Accountant.Decrement(password, accounted)
		}
	}

	_ = dst.Close()
	return
}

func (p *Proxy) tunnel(purchase *Purchase, request *Request, remote, conn net.Conn) error {
	accountData := request.IP == nil

	g, _ := errgroup.WithContext(context.Background())
	g.Go(func() error {
		return p.copy(purchase, accountData, false, request.Done, request.Password, conn, remote) //nolint:errcheck
	})
	g.Go(func() error {
		return p.copy(purchase, accountData, true, request.Done, request.Password, remote, conn) //nolint:errcheck
	})

	if err := g.Wait(); err != ErrConnectionClosed {
		p.stopTracker(purchase, request)
	} else {
		p.deleteTracker(purchase, request)
	}

	return nil
}

func hasAccess(purchase *Purchase, request *Request) error {
	// if len(purchase.IPs) > 0 {
	// 	_, ok := purchase.IPs[request.UserIP]
	// 	if !ok {
	// 		return ErrIPNotFound
	// 	}
	// }

	if !purchase.CountryTargeting && request.Country != nil {
		return ErrInvalidTargeting
	}

	if !purchase.Sticky && (request.SessionID != "" || request.SessionDuration != 0) {
		return ErrStickyNotSupported
	}

	return nil
}
