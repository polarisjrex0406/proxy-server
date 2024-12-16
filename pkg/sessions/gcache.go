package sessions

import (
	"github.com/bluele/gcache"
	"github.com/omimic12/proxy-server/pkg"
	"go.uber.org/zap"
)

type GCache struct {
	cache  gcache.Cache
	logger *zap.Logger
}

func NewGCache(size int, logger *zap.Logger) *GCache {
	cache := gcache.New(size).LRU().Build()

	return &GCache{
		cache:  cache,
		logger: logger,
	}
}

func (r *GCache) Cached(request *pkg.Request) (pkg.Provider, bool) {
	provider, err := r.cache.Get(request.SessionID)
	if err == gcache.KeyNotFoundError {
		return nil, false
	} else if err != nil {
		//something went wrong with gcache
		r.logger.Error(err.Error())
		return nil, false
	}

	p := provider.(pkg.Provider)

	return p, true
}

func (r *GCache) Start(request *pkg.Request) error {
	return r.cache.SetWithExpire(request.SessionID, request.Provider, request.SessionDuration)
}

func (r *GCache) Close() error {
	r.cache.Purge()
	return nil
}
