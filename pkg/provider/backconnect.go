package provider

import (
	"encoding/base64"
	"net"

	"github.com/omimic12/proxy-server/pkg"
	"github.com/valyala/bytebufferpool"
)

type Backconnect struct {
	provider string
	addr     string
	username []byte
	password []byte
	encoded  []byte
	weight   uint64
	protocol pkg.Protocol
	dialer   pkg.Dialer
	region   string
}

func NewBackconnect(
	addr string,
	username []byte,
	password []byte,
	weight uint64,
	provider string,
	protocol pkg.Protocol,
	dialer pkg.Dialer,
	region string,
) (*Backconnect, error) {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	buf.Write(username)  //nolint:errcheck
	buf.WriteString(":") //nolint:errcheck
	buf.Write(password)  //nolint:errcheck

	encoded := make([]byte, base64.StdEncoding.EncodedLen(buf.Len()))
	base64.StdEncoding.Encode(encoded, buf.Bytes())

	return &Backconnect{
		provider: provider,
		addr:     addr,
		username: username,
		password: password,
		protocol: protocol,
		encoded:  encoded,
		weight:   weight,
		dialer:   dialer,
		region: region,
	}, nil
}

func (s *Backconnect) Name() string {
	return s.provider
}

func (s *Backconnect) Protocol() pkg.Protocol {
	return s.protocol
}

func (s *Backconnect) Weight() uint64 {
	return s.weight
}

func (s *Backconnect) HasCountry(_ string) bool {
	return true
}

func (s *Backconnect) HasRegion(region string) bool {
	return s.region == region
}

func (s *Backconnect) HasCity(_ string) bool {
	return false
}

func (s *Backconnect) HasFeatures(_ ...pkg.Feature) bool {
	return false
}

func (s *Backconnect) HasRoutes(_ ...pkg.Route) bool {
	return false
}

func (s *Backconnect) BandwidthLimit() int64 {
	return -1
}

func (s *Backconnect) Credentials(_ *pkg.Request) (string, []byte, []byte, []byte, error) {
	return s.addr, s.username, s.password, s.encoded, nil
}

func (s *Backconnect) Dial(uri []byte, _ *pkg.Request) (rc net.Conn, err error) {
	return s.dialer.Dial(uri, s.addr, s.username, s.password)
}

func (s *Backconnect) PurchasedBy() uint {
	return 0
}
