package pkg

import (
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
	IPs               map[string]struct{}
}
