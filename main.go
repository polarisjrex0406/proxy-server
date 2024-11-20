package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net/http"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
	"github.com/omimic12/proxy-server/config"
	"github.com/omimic12/proxy-server/database"
	"github.com/omimic12/proxy-server/pkg"
	"github.com/omimic12/proxy-server/pkg/username"
	"github.com/pariz/gountries"
	"go.uber.org/zap"
)

var (
	ctx         = context.Background()
	redisClient *redis.Client
	db          *sql.DB
)

func main() {
	// 1. Load configuration
	cfg, err := config.GetConfig()
	if err != nil {
		panic(err)
	}

	lc := zap.NewProductionConfig()
	if cfg.Debug {
		lc = zap.NewDevelopmentConfig()
		lc.Development = true
	}

	logger, err := lc.Build()
	if err != nil {
		panic(err)
	}
	defer logger.Sync() //nolint:errcheck

	parser := username.NewBaseUsername(cfg.Session.Duration, cfg.Session.DurationMax, gountries.New())

	// Test PostgreSQL caching via Redis
	// Connect to PostgresSQL
	db = database.Connect()
	defer db.Close()

	// Connect to Redis
	redisClient = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
	})
	defer redisClient.Close()

	// Fake certification for MITM proxy
	caCertFile := flag.String("cacertfile", "/root/.local/share/mkcert/rootCA.pem", "certificate .pem file for trusted CA")
	caKeyFile := flag.String("cakeyfile", "/root/.local/share/mkcert/rootCA-key.pem", "key .pem file for trusted CA")
	flag.Parse()

	httpServer := newHttp(cfg)

	ch := make(chan map[string]int64)

	p := pkg.NewProxy(
		pkg.WithZeroThreadsChannel(ch),
		pkg.WithAccountBytes(cfg.Accountant.Bytes),
		pkg.WithBufferSize(cfg.Proxy.BufferSize),
		pkg.WithReadDeadline(cfg.Proxy.ReadDeadline),
		pkg.WithDialTimeout(cfg.Proxy.DialTimeout),
		pkg.WithHTTPServer(httpServer),

		pkg.WithUsernameParser(parser),

		pkg.WithCA(*caCertFile, *caKeyFile),
	)

	if cfg.Proxy.PortHTTP > 0 {
		go func() {
			logger.Info(fmt.Sprintf("Proxy: HTTP Starting :%d", cfg.Proxy.PortHTTP))
			defer logger.Info("Proxy: HTTP Stopped")

			err = p.ListenHTTP(ctx, cfg.Proxy.PortHTTP)
			if err != http.ErrServerClosed {
				logger.Error("HTTP Proxy failed to listen", zap.Error(err))
			}
		}()
	}

	select {}
}

func newHttp(conf *config.Config) *http.Server {
	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", conf.Proxy.PortHTTP),

		ReadTimeout:  conf.HTTP.ReadTimeout,  // not applied to Hijacked connections
		WriteTimeout: conf.HTTP.WriteTimeout, // not applied to Hijacked connections
		IdleTimeout:  conf.HTTP.IdleTimeout,
	}
	return srv
}
