package pkg

import (
	"sync/atomic"
	"time"
)

type Request struct {
	ID     string
	UserIP string
	Host   string
	Target string

	Protocol Protocol

	//Targeting
	IP        []byte
	Continent []byte
	Country   []byte
	Region    []byte
	City      []byte

	ProfileName []byte
	Category    []byte
	Product     []byte
	PurchaseID  uint

	Routes   []Route
	Features []Feature

	SessionID       string
	SessionDuration time.Duration

	Provider     Provider
	PurchaseUUID string
	PurchaseType PurchaseType

	Password string

	Written int64

	CreatedAt time.Time

	Done chan struct{}
}

func (r *Request) reset() {
	close(r.Done)

	r.ID = ""
	r.UserIP = ""
	r.Host = ""
	r.Protocol = HTTP
	r.Target = ""
	r.Continent = nil
	r.Country = nil
	r.Region = nil
	r.City = nil
	r.IP = nil
	r.SessionID = ""
	r.SessionDuration = 0
	r.Provider = nil
	r.PurchaseUUID = ""
	r.PurchaseType = PurchaseStatic
	r.Password = ""
	atomic.StoreInt64(&r.Written, 0)
	r.Routes = nil
	r.Features = nil
	r.CreatedAt = time.Time{}
	r.Done = make(chan struct{}, 1)
}

func (r *Request) Inc(written int64) int64 {
	return atomic.AddInt64(&r.Written, written)
}

func RequestKey(apiKey string, ID string) string {
	return apiKey + ":" + ID
}
