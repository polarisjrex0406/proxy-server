package provider

import (
	"encoding/base64"
	"net"

	"github.com/omimic12/proxy-server/pkg"
	"github.com/valyala/bytebufferpool"
)

const (
	GateDataImpulse = "gw.dataimpulse.com:823"
)

type DataImpulse struct {
	username []byte
	password []byte
	weight   uint64
	protocol pkg.Protocol
	dialer   pkg.Dialer

	purchaseId uint
}

func NewDataImpulse(username []byte, password []byte, weight uint64, protocol pkg.Protocol, dialer pkg.Dialer, purchaseId uint) *DataImpulse {
	return &DataImpulse{
		username:   username,
		password:   password,
		weight:     weight,
		protocol:   protocol,
		dialer:     dialer,
		purchaseId: purchaseId,
	}
}

func (s *DataImpulse) Name() string {
	return pkg.ProviderDataImpulse
}

func (s *DataImpulse) Protocol() pkg.Protocol {
	return s.protocol
}

func (s *DataImpulse) Weight() uint64 {
	return s.weight
}

func (s *DataImpulse) HasCountry(_ string) bool {
	return true
}

func (s *DataImpulse) HasRegion(_ string) bool {
	return true
}

func (s *DataImpulse) HasCity(_ string) bool {
	return true
}

func (s *DataImpulse) HasFeatures(_ ...pkg.Feature) bool {
	return true
}

func (s *DataImpulse) HasRoutes(levels ...pkg.Route) bool {
	return true
}

func (s *DataImpulse) BandwidthLimit() int64 {
	return -1
}

func (s *DataImpulse) Credentials(request *pkg.Request) (string, []byte, []byte, []byte, error) {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	buf.Write(s.username)
	buf.Write(byteColon)  //nolint:errcheck
	buf.Write(s.password) //nolint:errcheck

	cc := make([]byte, base64.StdEncoding.EncodedLen(buf.Len()))
	base64.StdEncoding.Encode(cc, buf.Bytes())

	return GateDataImpulse, s.username, s.password, cc, nil
}

func (s *DataImpulse) Dial(uri []byte, request *pkg.Request) (rc net.Conn, err error) {
	return s.dialer.Dial(uri, GateDataImpulse, s.username, s.password)
}

func (s *DataImpulse) PurchasedBy() uint {
	return s.purchaseId
}
