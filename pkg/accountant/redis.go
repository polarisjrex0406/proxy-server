package accountant

import (
	"context"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/valyala/bytebufferpool"
	"go.uber.org/zap"
)

type Redis struct {
	data chan string
}

const (
	strColon = ":"
)

func NewRedis(
	ctx context.Context,
	bufferSize int,
	dataChName string,
	client *redis.Client,
	period time.Duration,
	logger *zap.Logger,
) (*Redis, error) {
	var data = make(chan string, bufferSize)
	go func() {
		ticker := time.NewTicker(period)
		defer ticker.Stop()

		buf := bytebufferpool.Get()
		defer bytebufferpool.Put(buf)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if buf.Len() == 0 {
					continue
				}

				_, err := client.Publish(context.Background(), dataChName, buf.String()).Result()
				if err != nil {
					logger.Error("failed to publish data", zap.Error(err))
				}
				buf.Reset()
			case d := <-data:
				if buf.Len() > 0 {
					_, err := buf.WriteString(strColon)
					if err != nil {
						logger.Error("failed to write semicolon to buffer", zap.Error(err))
						continue
					}
				}

				_, err := buf.WriteString(d)
				if err != nil {
					logger.Error("failed to write usage data to buffer", zap.Error(err))
				}
			}
		}
	}()

	return &Redis{
		data: data,
	}, nil
}

func (r *Redis) Decrement(password string, bytes int64) error {
	r.data <- password + "," + strconv.FormatInt(bytes, 10)
	return nil
}
