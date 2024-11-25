package pkg

import (
	"time"
)

type PurchaseType string

const (
	PurchaseStatic      PurchaseType = "static"
	PurchaseResidential PurchaseType = "residential"
)

type ProxyServiceType string

const (
	ProxyStatic      = ProxyServiceType("static")
	ProxyBackconnect = ProxyServiceType("backconnect")
	ProxyProvider    = ProxyServiceType("provider")
	ProxySubnet      = ProxyServiceType("subnet")
	ProxyISPPool     = ProxyServiceType("isp_pool")
)

type IPVersion string

const (
	IPVersion4 = IPVersion("ipv4")
	IPVersion6 = IPVersion("ipv6")
)

type Purchase struct {
	// Purchase
	ID          uint
	TrafficLeft *int
	IPCount     *int
	IPs         map[string]struct{}
	Threads     *int
	Region      string
	ExpireAt    time.Time

	// Product
	ProxyServiceType ProxyServiceType
	CountryTargeting bool
	IPVersion        IPVersion
	Protocols        []Protocol
	StickySession    bool
}
