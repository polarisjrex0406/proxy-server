package pkg

import (
	"context"
	"errors"
)

var (
	ErrNotEnoughData    = errors.New("not enough data")
	ErrMissingAuth      = errors.New("missing auth")
	ErrPurchaseNotFound = errors.New("purchase not found")
	ErrDomainBlocked    = errors.New("domain blocked")
	ErrIPNotAllowed     = errors.New("ip not allowed")
)

type Auth interface {
	Authenticate(ctx context.Context, password string) (*Purchase, error)
}
