package pkg

import (
	"time"

	"github.com/detailyang/domaintree-go"
)

type PurchaseType string

const (
	PurchaseStatic      PurchaseType = "static"
	PurchaseResidential PurchaseType = "residential"
)

type Purchase struct {
	UUID              string
	Type              PurchaseType
	Threads           int64
	WhitelistIP       map[string]struct{}
	BlockedDomains    *domaintree.DomainTree
	BlacklistHostname int

	ID          uint
	TrafficLeft *int
	IPCount     *int
	IPs         map[string]struct{}
	// Threads     *int
	Region   string
	ExpireAt time.Time
}
