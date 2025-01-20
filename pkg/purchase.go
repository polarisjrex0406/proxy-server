package pkg

import (
	"time"
)

type PurchaseType string

const (
	PurchaseStatic      PurchaseType = "static"
	PurchaseBackconnect PurchaseType = "backconnect"
	PurchaseProvider    PurchaseType = "provider"
	PurchaseSubnet      PurchaseType = "subnet"
	PurchaseISPPool     PurchaseType = "isp_pool"
)

type IPVersion string

const (
	IPv4 IPVersion = "ipv4"
	IPv6 IPVersion = "ipv6"
)

type Purchase struct {
	// Purchase
	ID       uint
	IPs      map[string]struct{}
	Threads  int64
	Region   string
	ExpireAt time.Time

	Type string

	IPVersion        IPVersion
	Sticky           bool
	CountryTargeting bool

	BandwidthLimited bool
}
