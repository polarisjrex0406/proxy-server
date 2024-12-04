package settings

import (
	"context"

	"github.com/omimic12/proxy-server/pkg"
)

type Fixed struct {
	providers []pkg.Provider
}

func NewFixed(providers []pkg.Provider) *Fixed {
	return &Fixed{providers: providers}
}

func (f *Fixed) LoadProviders(_ context.Context) ([]pkg.Provider, error) {
	return f.providers, nil
}
