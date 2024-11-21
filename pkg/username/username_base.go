package username

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/omimic12/proxy-server/pkg"
	"github.com/omimic12/proxy-server/pkg/zerocopy"
	"github.com/pariz/gountries"
)

var (
	strUsernameCountryGB = "gb"
	strUsernameCountryUK = "uk"
)

var (
	byteUsernameRegion   = []byte("region")
	byteUsernameCountry  = []byte("country")
	byteUsernameCity     = []byte("city")
	byteUsernameDuration = []byte("duration")
	byteUsernameSession  = []byte("session")
	byteUsernameIP       = []byte("ip")
	byteUsernameDash     = []byte("-")
	byteUsernameRandom   = []byte("rr")
)

var (
	hashPool = sync.Pool{}
)

var (
	ErrInvalidParam     = errors.New("invalid param")
	ErrInvalidTargeting = errors.New("invalid targeting")
	ErrInvalidCountry   = errors.New("invalid country")
	ErrInvalidRegion    = errors.New("invalid region")
)

// acquireHash returns a hash from pool
func acquireHash() *xxhash.Digest {
	v := hashPool.Get()
	if v == nil {
		return xxhash.New()
	}
	return v.(*xxhash.Digest)
}

// releaseHash returns hash to pool
func releaseHash(h *xxhash.Digest) {
	h.Reset()
	hashPool.Put(h)
}

type Base struct {
	sessionDuration    time.Duration
	sessionDurationMax time.Duration
	location           *gountries.Query
}

func NewBaseUsername(sessionDuration time.Duration, sessionDurationMax time.Duration, location *gountries.Query) *Base {
	return &Base{location: location, sessionDuration: sessionDuration, sessionDurationMax: sessionDurationMax}
}

func (s *Base) Parse(username []byte, req *pkg.Request) (err error) {
	var sessionID []byte
	params := bytes.Split(username, byteUsernameDash)

	if len(params) <= 1 {
		return ErrInvalidParam
	}

	for i, p := range params {
		switch i {
		case 0:
			req.ProfileName = p
		case 1:
			req.Product = p
		case 2:
			req.Category = p
		case 3:
			purchaseId, err := strconv.Atoi(zerocopy.String(p))
			if err != nil {
				return err
			}
			req.PurchaseID = uint(purchaseId)
		}

		i++
		if i%2 == 0 || i <= 3 {
			continue
		}

		if i >= len(params) {
			break
		}

		if bytes.EqualFold(p, byteUsernameCountry) {
			if len(params[i]) > 2 {
				return ErrInvalidParam
			}

			req.Country = params[i]
			continue
		} else if bytes.EqualFold(p, byteUsernameRegion) {
			req.Region = params[i]
			continue
		} else if bytes.EqualFold(p, byteUsernameCity) {
			req.City = params[i]
			continue
		} else if bytes.EqualFold(p, byteUsernameSession) {
			sessionID = params[i]
			continue
		} else if bytes.EqualFold(p, byteUsernameDuration) {
			duration, err := strconv.Atoi(zerocopy.String(params[i]))
			if err != nil {
				return err
			}

			req.SessionDuration = time.Duration(duration) * time.Second
			continue
		} else if bytes.EqualFold(p, byteUsernameIP) {
			req.IP = params[i]
			continue
		}
	}

	// if req.Country == nil && req.Region == nil && req.City == nil && req.IP == nil {
	// 	return ErrInvalidTargeting
	// }

	var c gountries.Country
	if (req.Country != nil && !bytes.EqualFold(req.Country, byteUsernameRandom)) && req.City == nil {
		c, err = s.location.FindCountryByAlpha(zerocopy.String(req.Country))
		if err != nil {
			if strings.EqualFold(zerocopy.String(req.Country), strUsernameCountryUK) { //small hack to accept both US and GB
				req.Country = zerocopy.Bytes(strUsernameCountryGB)
				c, err = s.location.FindCountryByAlpha(zerocopy.String(req.Country))
				if err != nil {
					return ErrInvalidCountry
				}
			} else {
				return err
			}
		}
	}

	if (req.Country != nil && !bytes.EqualFold(req.Country, byteUsernameRandom)) && req.Region != nil {
		//Country and region was provided
		_, err = c.FindSubdivisionByCode(zerocopy.String(req.Region))
		if err != nil {
			_, err = c.FindSubdivisionByName(zerocopy.String(req.Region))
		}

		if err != nil {
			return ErrInvalidRegion
		}
	}

	if len(sessionID) > 0 {
		if req.SessionDuration <= 0 || req.SessionDuration > s.sessionDurationMax {
			req.SessionDuration = s.sessionDuration
		}

		digest := acquireHash()
		defer releaseHash(digest)

		if req.Country != nil {
			_, err = digest.Write(req.Country)
			if err != nil {
				return err
			}
		}

		_, err = digest.Write(sessionID)
		if err != nil {
			return err
		}

		_, err = digest.WriteString(req.Password)
		if err != nil {
			return err
		}

		req.SessionID = strconv.FormatUint(digest.Sum64(), 10)
	}

	req.Routes = make([]pkg.Route, 0, 6)
	if req.Continent != nil {
		req.Routes = append(req.Routes, pkg.RouteContinent)
	}

	if req.Country != nil {
		req.Routes = append(req.Routes, pkg.RouteCountry)
	}

	if req.Region != nil {
		req.Routes = append(req.Routes, pkg.RouteRegion)
	}

	if req.City != nil {
		req.Routes = append(req.Routes, pkg.RouteCity)
	}

	req.Features = make([]pkg.Feature, 0, 2)
	if req.SessionID != "" {
		req.Features = append(req.Features, pkg.Sticky)
	} else {
		req.Features = append(req.Features, pkg.Rotating)
	}

	if req.SessionDuration > 0 {
		req.Features = append(req.Features, pkg.SessionDuration)
	}

	req.CreatedAt = time.Now()

	return
}
