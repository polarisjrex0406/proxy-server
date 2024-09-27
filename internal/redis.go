package internal

import (
	"context"
	"database/sql"
	"time"

	"github.com/go-redis/redis/v8"
)

func GetCachedData(ctx context.Context, redisClient *redis.Client, DB *sql.DB, query string) (string, error) {
	// Check Redis cache
	cacheKey := query
	cachedData, err := redisClient.Get(ctx, cacheKey).Result()
	if err == redis.Nil {
		// Cache miss, query PostgreSQL
		var result string
		err = DB.QueryRow(query).Scan(&result)
		if err != nil {
			return "", err
		}

		// Cache the result in Redis
		err = redisClient.Set(ctx, cacheKey, result, 1*time.Hour).Err()
		if err != nil {
			return "", err
		}

		return result, nil
	} else if err != nil {
		return "", err
	}

	// Return cached result
	return cachedData, nil
}
