package pkg

import "context"

type Settings interface {
	LoadProviders(ctx context.Context) ([]Provider, error)
}
