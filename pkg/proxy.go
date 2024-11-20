package pkg

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

var (
	strBasic = []byte("Basic ")
)

type Proxy struct {
	config *Options
}

func NewProxy(options ...Option) *Proxy {
	var option = new(Options)
	for _, o := range options {
		o(option)
	}

	p := &Proxy{config: option}

	if p.config.HTTPServer != nil {
		p.config.HTTPServer.Handler = http.HandlerFunc(p.handlerHTTP)
	}
	return p
}

func (p *Proxy) selectProvider(request *Request) error {
	var err error

	if request.SessionID != "" {
		var ok bool
		request.Provider, ok = p.config.Sessions.Cached(request)
		if !ok {
			request.Provider, err = p.config.Router.Route(request)
			if err != nil {
				return err
			}

			err = p.config.Sessions.Start(request)
			if err != nil {
				return err
			}
		}

		return nil
	}

	request.Provider, err = p.config.Router.Route(request)
	return err
}

func (p *Proxy) logError(err error, request *Request) {
	pn := ""
	if request.Provider != nil {
		pn = request.Provider.Name()
	}

	p.config.Logger.Error(err.Error(),
		zap.ByteString("continent", request.Continent),
		zap.ByteString("country", request.Country),
		zap.ByteString("region", request.Region),
		zap.ByteString("city", request.City),
		zap.ByteString("ip", request.IP),
		zap.String("provider", pn),
		zap.String("user_ip", request.UserIP))
}

func (p *Proxy) PublishThreads(ctx context.Context, ch <-chan map[string]int64, period time.Duration, channel string, client *redis.Client) error {
	ticker := time.NewTicker(period)
	defer ticker.Stop()

	var cache = make(map[string]int64)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case m := <-ch:
			for k, v := range m {
				cache[k] = v
			}
		case <-ticker.C:
			threads := p.config.ConnectionTracker.Threads()
			if len(threads) == 0 {
				continue
			}

			if len(cache) > 0 {
				for k, v := range cache {
					threads[k] = v
				}

				cache = make(map[string]int64)
			}

			data, err := json.Marshal(threads)
			if err != nil {
				return err
			}

			err = client.Publish(context.Background(), channel, string(data)).Err()
			if err != nil {
				p.config.Logger.Error("failed to publish threads statistics", zap.Error(err))
			}
		}
	}
}
