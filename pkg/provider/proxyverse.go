package provider

import (
	"bytes"
	"encoding/base64"
	"strconv"

	"github.com/omimic12/proxy-server/pkg"
	"github.com/valyala/bytebufferpool"
)

const (
	GateProxyverse = "51.81.93.42:9200"
)

var (
	byteCountry                 = []byte("country-")
	byteContinent               = []byte("continent-")
	byteCity                    = []byte("city-")
	byteRegion                  = []byte("region-")
	byteSession                 = []byte("session-")
	bytesDuration               = []byte("duration-")
	byteColon                   = []byte(":")
	byteDash                    = []byte("-")
	byteRandomCountry           = []byte("rr")
	byteRandomCountryProxyverse = []byte("worldwide")
)

type Proxyverse struct {
	password []byte
	weight   uint64
	protocol pkg.Protocol
}

func NewProxyverse(password []byte, weight uint64, protocol pkg.Protocol) *Proxyverse {
	return &Proxyverse{
		password: password,
		weight:   weight,
		protocol: protocol,
	}
}

func (s *Proxyverse) Name() string {
	return pkg.ProviderProxyverse
}

func (s *Proxyverse) Protocol() pkg.Protocol {
	return s.protocol
}

func (s *Proxyverse) Weight() uint64 {
	return s.weight
}

func (s *Proxyverse) HasCountry(_ string) bool {
	return true
}

func (s *Proxyverse) HasRegion(_ string) bool {
	return true
}

func (s *Proxyverse) HasCity(_ string) bool {
	return true
}

func (s *Proxyverse) HasFeatures(_ ...pkg.Feature) bool {
	return true
}

func (s *Proxyverse) HasRoutes(levels ...pkg.Route) bool {
	return true
}

func (s *Proxyverse) BandwidthLimit() int64 {
	return -1
}

func (s *Proxyverse) Credentials(request *pkg.Request) (string, []byte, []byte, []byte, error) {
	buf := bytebufferpool.Get()
	defer bytebufferpool.Put(buf)

	err := s.buildUsername(buf, request)
	if err != nil {
		return "", nil, nil, nil, err
	}

	buf.Write(byteColon)  //nolint:errcheck
	buf.Write(s.password) //nolint:errcheck

	cc := make([]byte, base64.StdEncoding.EncodedLen(buf.Len()))
	base64.StdEncoding.Encode(cc, buf.Bytes())

	return GateProxyverse, nil, nil, cc, nil
}

func (s *Proxyverse) buildUsername(username *bytebufferpool.ByteBuffer, request *pkg.Request) error {
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
			request.Country = byteRandomCountryProxyverse
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
