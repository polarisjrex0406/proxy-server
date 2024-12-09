package provider

import (
	"encoding/base64"

	"github.com/omimic12/proxy-server/pkg"
	"github.com/valyala/bytebufferpool"
)

const (
	GateTTProxy = "http://dynamic.ttproxy.com:10001"
)

type TTProxy struct {
	username []byte
	password []byte
	weight   uint64
	protocol pkg.Protocol

	purchaseId uint
}

func NewTTProxy(username []byte, password []byte, weight uint64, protocol pkg.Protocol, purchaseId uint) *TTProxy {
	return &TTProxy{
		username:   username,
		password:   password,
		weight:     weight,
		protocol:   protocol,
		purchaseId: purchaseId,
	}
}

func (s *TTProxy) Name() string {
	return pkg.ProviderTTProxy
}

func (s *TTProxy) Protocol() pkg.Protocol {
	return s.protocol
}

func (s *TTProxy) Weight() uint64 {
	return s.weight
}

func (s *TTProxy) HasCountry(_ string) bool {
	return true
}

func (s *TTProxy) HasRegion(_ string) bool {
	return true
}

func (s *TTProxy) HasCity(_ string) bool {
	return true
}

func (s *TTProxy) HasFeatures(_ ...pkg.Feature) bool {
	return true
}

func (s *TTProxy) HasRoutes(levels ...pkg.Route) bool {
	return true
}

func (s *TTProxy) BandwidthLimit() int64 {
	return -1
}

func (s *TTProxy) Credentials(request *pkg.Request) (string, []byte, []byte, []byte, error) {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	buf.Write(s.username)
	buf.Write(byteColon)  //nolint:errcheck
	buf.Write(s.password) //nolint:errcheck

	cc := make([]byte, base64.StdEncoding.EncodedLen(buf.Len()))
	base64.StdEncoding.Encode(cc, buf.Bytes())

	return GateTTProxy, nil, nil, cc, nil
}

func (s *TTProxy) PurchasedBy() uint {
	return s.purchaseId
}
