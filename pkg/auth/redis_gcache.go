package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bluele/gcache"
	"github.com/go-redis/redis/v8"
	"github.com/lib/pq"
	"github.com/omimic12/proxy-server/pkg"
	"go.uber.org/zap"
)

type RedisGCache struct {
	parser        pkg.UsernameParser
	redisData     *redis.Client
	redisPurchase *redis.Client
	cache         gcache.Cache
	cacheTTL      time.Duration
	logger        *zap.Logger

	db *sql.DB
}

func NewRedisGCache(
	signalCtx context.Context,
	records int,
	cacheTTL time.Duration,
	redisCh string,
	redisData, redisPurchase *redis.Client,
	parser pkg.UsernameParser,
	logger *zap.Logger,

	db *sql.DB,
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

		db: db,
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
	fmt.Println("r.cache.Get(password) =", err)
	fmt.Println("r.cacheRedisFromPrimary(ctx, password) =", r.cacheRedisFromPrimary(ctx, password))
	if err == gcache.KeyNotFoundError {
		jsonData, err := r.redisPurchase.Get(ctx, password).Result()
		fmt.Println("r.redisPurchase.Get(ctx, password).Result() =", err)
		if err == redis.Nil {
			return nil, pkg.ErrPurchaseNotFound
		} else if err != nil {
			return nil, err
		}

		cached := make(map[string]interface{})
		if err := json.Unmarshal([]byte(jsonData), &cached); err != nil {
			return nil, err // Handle JSON unmarshaling error
		}

		cachedData, err := json.MarshalIndent(cached, "", " ")
		if err != nil {
			return nil, err
		}
		fmt.Println("cachedData =", string(cachedData))

		// threads := gjson.GetBytes(data, "threads")

		// fmt.Println("gjson.GetBytes(data, 'threads') =", threads)

		// purchase := &pkg.Purchase{
		// 	IPs: make(map[string]struct{}),
		// }

		// ips := gjson.GetBytes(data, "ips")
		// if ips.IsArray() {
		// 	ips.ForEach(func(key, value gjson.Result) bool {
		// 		purchase.IPs[value.String()] = struct{}{}
		// 		return true
		// 	})
		// }

		// ttl, err := r.redisData.TTL(ctx, password).Result()
		// if err != redis.Nil && err != nil {
		// 	return nil, err
		// }

		// if ttl <= 0 || ttl > r.cacheTTL {
		// 	ttl = r.cacheTTL
		// }

		// err = r.cache.SetWithExpire(password, purchase, ttl)
		// if err != nil {
		// 	return nil, err
		// }

		// return purchase, nil
	} else if err != nil {
		return nil, err
	}

	purchase, ok := u.(*pkg.Purchase)
	if !ok {
		return nil, errors.New("invalid purchase type")
	}

	return purchase, nil
}

func (r *RedisGCache) cacheRedisFromPrimary(ctx context.Context, password string) error {
	// Find purchase by password
	var productId uint
	var bandwidth, trafficLeft, duration, ipCount, threads *int
	var ips []string
	var region string
	var expireAt time.Time
	query := fmt.Sprintf(
		"SELECT product_id, bandwidth, traffic_left, duration, ip_count, ips, threads, region, expire_at FROM tbl_purchases WHERE pswd = '%v'",
		password,
	)
	if err := r.db.QueryRow(query).Scan(
		&productId, &bandwidth, &trafficLeft, &duration, &ipCount, pq.Array(&ips), &threads, &region, &expireAt,
	); err != nil {
		return err
	}

	cached := make(map[string]interface{})
	cached["product_id"] = productId
	cached["bandwidth"] = bandwidth
	cached["traffic_left"] = trafficLeft
	cached["duration"] = duration
	cached["ip_count"] = ipCount
	cached["ips"] = ips
	cached["threads"] = threads
	cached["region"] = region
	cached["expireAt"] = expireAt

	// Marshal the map to JSON for pretty printing
	jsonData, err := json.Marshal(cached)
	if err != nil {
		return err
	}

	if err := r.redisPurchase.Set(ctx, password, jsonData, 1*time.Hour).Err(); err != nil {
		return err
	}

	return nil
}
