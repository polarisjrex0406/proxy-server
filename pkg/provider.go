package pkg

const (
	ProviderStatic      = "static"
	ProviderDataImpulse = "dataimpulse"
	ProviderTTProxy     = "ttproxy"
	ProviderProxyverse  = "proxyverse"
	ProviderDatabay     = "databay"
)

type Feature []byte

var (
	Rotating        Feature = []byte("rotating")
	Sticky          Feature = []byte("sticky")
	SessionDuration Feature = []byte("duration")
)

type Route string

const (
	RouteContinent Route = "continent"
	RouteCountry   Route = "country"
	RouteRegion    Route = "region"
	RouteCity      Route = "city"
)

type Provider interface {
	Name() string

	//Weight - weight used in provider selection
	Weight() uint64
	Protocol() Protocol

	HasFeatures(feature ...Feature) bool
	HasRoutes(level ...Route) bool
	HasCountry(country string) bool
	HasRegion(region string) bool
	HasCity(city string) bool

	BandwidthLimit() int64

	Credentials(*Request) (hostname string, username, password []byte, encoded []byte, err error)

	PurchasedBy() uint
}
