package provider

import (
	"encoding/base64"

	"github.com/omimic12/proxy-server/pkg"
	"github.com/valyala/bytebufferpool"
)

type Static struct {
	provider string
	addr     string
	username []byte
	password []byte
	encoded  []byte
	weight   uint64
	protocol pkg.Protocol
}

func NewStatic(
	addr string,
	username []byte,
	password []byte,
	weight uint64,
	provider string,
	protocol pkg.Protocol,
) (*Static, error) {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	buf.Write(username)  //nolint:errcheck
	buf.WriteString(":") //nolint:errcheck
	buf.Write(password)  //nolint:errcheck

	encoded := make([]byte, base64.StdEncoding.EncodedLen(buf.Len()))
	base64.StdEncoding.Encode(encoded, buf.Bytes())

	return &Static{
		provider: provider,
		addr:     addr,
		username: username,
		password: password,
		protocol: protocol,
		encoded:  encoded,
		weight:   weight,
	}, nil
}

func (s *Static) Name() string {
	return s.provider
}

func (s *Static) Protocol() pkg.Protocol {
	return s.protocol
}

func (s *Static) Weight() uint64 {
	return s.weight
}

func (s *Static) HasCountry(_ string) bool {
	return true
}

func (s *Static) HasRegion(_ string) bool {
	return false
}

func (s *Static) HasCity(_ string) bool {
	return false
}

func (s *Static) HasFeatures(_ ...pkg.Feature) bool {
	return false
}

func (s *Static) HasRoutes(_ ...pkg.Route) bool {
	return false
}

func (s *Static) BandwidthLimit() int64 {
	return -1
}

func (s *Static) Credentials(_ *pkg.Request) (string, []byte, []byte, []byte, error) {
	return s.addr, nil, nil, s.encoded, nil
}
