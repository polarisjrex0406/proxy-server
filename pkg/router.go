package pkg

import "errors"

var (
	ErrFailedSelectProvider     = errors.New("failed to select a provider")
	ErrProviderProtocolNotMatch = errors.New("provider protocol not match request")
)

type Router interface {
	//Route - find provider which will be used to route the request
	Route(*Purchase, *Request) (Provider, error)
}
