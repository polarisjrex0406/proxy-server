package provider

import (
	"bytes"
	"encoding/base64"
	"strconv"

	"github.com/omimic12/proxy-server/pkg"
	"github.com/valyala/bytebufferpool"
)

const (
	GateDatabay = "http://resi-global-gateways.databay.com:7676"
)

var (
	byteRandomCountryDatabay = []byte("worldwide")
)

type Databay struct {
	username []byte
	password []byte
	weight   uint64
	protocol pkg.Protocol

	purchaseId uint
}

func NewDatabay(username []byte, password []byte, weight uint64, protocol pkg.Protocol, purchaseId uint) *Databay {
	return &Databay{
		username:   username,
		password:   password,
		weight:     weight,
		protocol:   protocol,
		purchaseId: purchaseId,
	}
}

func (s *Databay) Name() string {
	return pkg.ProviderDatabay
}

func (s *Databay) Protocol() pkg.Protocol {
	return s.protocol
}

func (s *Databay) Weight() uint64 {
	return s.weight
}

func (s *Databay) HasCountry(_ string) bool {
	return true
}

func (s *Databay) HasRegion(_ string) bool {
	return true
}

func (s *Databay) HasCity(_ string) bool {
	return true
}

func (s *Databay) HasFeatures(_ ...pkg.Feature) bool {
	return true
}

func (s *Databay) HasRoutes(levels ...pkg.Route) bool {
	return true
}

func (s *Databay) BandwidthLimit() int64 {
	return -1
}

func (s *Databay) Credentials(request *pkg.Request) (string, []byte, []byte, []byte, error) {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	buf.Write(s.username)
	err := s.buildUsername(buf, request)
	if err != nil {
		return "", nil, nil, nil, err
	}

	buf.Write(byteColon)  //nolint:errcheck
	buf.Write(s.password) //nolint:errcheck

	cc := make([]byte, base64.StdEncoding.EncodedLen(buf.Len()))
	base64.StdEncoding.Encode(cc, buf.Bytes())

	return GateDatabay, nil, nil, cc, nil
}

func (s *Databay) buildUsername(username *bytebufferpool.ByteBuffer, request *pkg.Request) error {
	if request.Continent != nil {
		username.Write(byteContinent)     //nolint:errcheck
		username.Write(request.Continent) //nolint:errcheck
	}

	if request.Country != nil {
		if username.Len() > 0 {
			username.Write(byteDash) //nolint:errcheck
		}

		username.Write(byteCountry) //nolint:errcheck

		if bytes.EqualFold(request.Country, byteRandomCountry) {
			request.Country = byteRandomCountryDatabay
		}

		username.Write(request.Country) //nolint:errcheck
	}

	if request.City != nil {
		if username.Len() > 0 {
			username.Write(byteDash) //nolint:errcheck
		}

		username.Write(byteCity)     //nolint:errcheck
		username.Write(request.City) //nolint:errcheck
	}

	if request.Region != nil {
		if username.Len() > 0 {
			username.Write(byteDash) //nolint:errcheck
		}

		username.Write(byteRegion)     //nolint:errcheck
		username.Write(request.Region) //nolint:errcheck
	}

	if request.SessionID != "" {
		if username.Len() > 0 {
			username.Write(byteDash) //nolint:errcheck
		}

		username.Write(byteSession)             //nolint:errcheck
		username.WriteString(request.SessionID) //nolint:errcheck
	}

	if request.SessionDuration != 0 {
		if username.Len() > 0 {
			username.Write(byteDash) //nolint:errcheck
		}

		username.Write(bytesDuration)                                              //nolint:errcheck
		username.WriteString(strconv.Itoa(int(request.SessionDuration.Seconds()))) //nolint:errcheck
	}

	return nil
}

func (s *Databay) PurchasedBy() uint {
	return s.purchaseId
}
