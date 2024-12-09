package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bluele/gcache"
	"github.com/go-redis/redis/v8"
	"github.com/omimic12/proxy-server/pkg"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

type RedisGCache struct {
	parser        pkg.UsernameParser
	redisData     *redis.Client
	redisPurchase *redis.Client
	cache         gcache.Cache
	cacheTTL      time.Duration
	logger        *zap.Logger
}

func NewRedisGCache(
	signalCtx context.Context,
	records int,
	cacheTTL time.Duration,
	redisCh string,
	redisData, redisPurchase *redis.Client,
	parser pkg.UsernameParser,
	logger *zap.Logger,
) (*RedisGCache, error) {
	cache := gcache.New(records).
		LRU().
		Build()

	r := &RedisGCache{
		parser:        parser,
		redisData:     redisData,
		redisPurchase: redisPurchase,
		cache:         cache,
		cacheTTL:      cacheTTL,
		logger:        logger,
	}

	go func() {
		deleteCh := redisData.Subscribe(context.Background(), redisCh).Channel()
		defer func() {
			cache.Purge()
		}()

		for {
			select {
			case <-signalCtx.Done():
				return
			case data := <-deleteCh:
				cache.Remove(data.Payload)
			}
		}
	}()

	return r, nil
}

func (r *RedisGCache) Authenticate(ctx context.Context, password string) (*pkg.Purchase, error) {
	u, err := r.cache.Get(password)
	if err == gcache.KeyNotFoundError {
		data, err := r.redisPurchase.Get(ctx, password).Bytes()
		if err == redis.Nil {
			return nil, pkg.ErrPurchaseNotFound
		} else if err != nil {
			return nil, err
		}

		t := gjson.GetBytes(data, "type").String()
		if t != "static" && t != "backconnect" && t != "provider" && t != "subnet" && t != "isp_pool" {
			return nil, fmt.Errorf("invalid purchase type %s", password)
		}

		threads := gjson.GetBytes(data, "threads").Int()

		purchase := &pkg.Purchase{
			ID:      uint(gjson.GetBytes(data, "id").Int()),
			Threads: threads,
			IPs:     make(map[string]struct{}),
			Type:    t,
		}

		bandwidthLimited := gjson.GetBytes(data, "bandwidth_limited").Bool()
		if bandwidthLimited {
			value, err := r.redisData.Get(ctx, password).Int64()
			if err == redis.Nil {
				return nil, pkg.ErrPurchaseNotFound
			} else if err != nil {
				return nil, err
			}

			if value <= 0 {
				return nil, pkg.ErrNotEnoughData
			}
		}

		ips := gjson.GetBytes(data, "ips")
		if ips.IsArray() {
			ips.ForEach(func(key, value gjson.Result) bool {
				purchase.IPs[value.String()] = struct{}{}
				return true
			})
		}

		ttl, err := r.redisData.TTL(ctx, password).Result()
		if err != redis.Nil && err != nil {
			return nil, err
		}

		if ttl <= 0 || ttl > r.cacheTTL {
			ttl = r.cacheTTL
		}

		err = r.cache.SetWithExpire(password, purchase, ttl)
		if err != nil {
			return nil, err
		}

		return purchase, nil
	} else if err != nil {
		return nil, err
	}

	purchase, ok := u.(*pkg.Purchase)
	if !ok {
		return nil, errors.New("invalid purchase type")
	}

	return purchase, nil
}
