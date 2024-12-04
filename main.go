package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
	"github.com/omimic12/proxy-server/config"
	"github.com/omimic12/proxy-server/database"
	"github.com/omimic12/proxy-server/pkg"
	"github.com/omimic12/proxy-server/pkg/auth"
	"github.com/omimic12/proxy-server/pkg/router"
	"github.com/omimic12/proxy-server/pkg/settings"
	"github.com/omimic12/proxy-server/pkg/username"
	"github.com/pariz/gountries"
	"go.uber.org/zap"
)

var (
	ctx = context.Background()
	db  *sql.DB
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

	// Connect to PostgresSQL
	db = database.Connect()
	defer db.Close()

	// Connect to Redis
	options, err := redis.ParseURL(fmt.Sprintf("%s/%d", cfg.Redis.DSN, cfg.Redis.DB.Data))
	if err != nil {
		panic(err)
	}

	redisData := redis.NewClient(options)
	defer redisData.Close() //nolint:errcheck

	_, err = redisData.Ping(context.Background()).Result()
	if err != nil {
		panic(err)
	}

	options, err = redis.ParseURL(fmt.Sprintf("%s/%d", cfg.Redis.DSN, cfg.Redis.DB.Purchase))
	if err != nil {
		logger.Panic("failed to parse redis purchase database", zap.Error(err))
	}

	redisPurchase := redis.NewClient(options)
	defer redisPurchase.Close() //nolint:errcheck

	options, err = redis.ParseURL(fmt.Sprintf("%s/%d", cfg.Redis.DSN, cfg.Redis.DB.Proxy))
	if err != nil {
		logger.Panic("failed to parse redis proxy database", zap.Error(err))
	}

	redisProxy := redis.NewClient(options)
	defer redisProxy.Close() //nolint:errcheck

	_, err = redisProxy.Ping(ctx).Result()
	if err != nil {
		logger.Panic("failed to ping redis proxy database", zap.Error(err))
	}

	a, err := auth.NewRedisGCache(
		ctx,
		cfg.Authorization.CacheSize,
		cfg.Authorization.TTL,
		cfg.Redis.Channel.User,
		redisData,
		redisPurchase,
		parser,
		logger,
	)
	if err != nil {
		panic(err)
	}

	providers := []pkg.Provider{}
	fixedSettings := settings.NewFixed(providers)

	fetchTimeout := time.Second * 5
	rr, err := router.NewWeightedRoundRobin(
		fixedSettings,
		cfg.Proxy.DialTimeout,
		cfg.Proxy.ReadDeadline,
		fetchTimeout,
		cfg.Provider.Static.SyncPeriod,
		redisProxy,
		logger,
	)
	if err != nil {
		panic(err)
	}

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
		pkg.WithAuth(a),
		pkg.WithRouter(rr),

		pkg.WithUsernameParser(parser),
		pkg.WithLogger(logger),

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
