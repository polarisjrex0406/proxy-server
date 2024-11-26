package pkg

import (
	"time"
)

type PurchaseType string

const (
	PurchaseStatic      PurchaseType = "static"
	PurchaseResidential PurchaseType = "residential"
)

type Purchase struct {
	// Purchase
	ID       uint
	IPs      map[string]struct{}
	Threads  int64
	Region   string
	ExpireAt time.Time
}
