package config

import (
	"fmt"
	"sync"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/joho/godotenv"
)

var (
	instance *Config
	once     sync.Once
)

// LoadConfig loads the configuration from .env and command-line flags.
func LoadConfig() (*Config, error) {
	var cfg Config
	if err := godotenv.Load(); err != nil {
		return nil, fmt.Errorf("error loading .env file: %w", err)
	}
	fp := flags.NewParser(&cfg, flags.Default)
	// Parse flags
	if _, err := fp.Parse(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// GetConfig returns the singleton instance of Config.
func GetConfig() (*Config, error) {
	var err error
	once.Do(func() {
		instance, err = LoadConfig()
	})
	return instance, err
}

type Config struct {
	Debug bool `long:"debug" env:"DEBUG"`

	Server struct {
		Port int `long:"server-port" env:"SERVER_PORT" default:"8080"`
	}

	Postgres struct {
		DBName   string `long:"postgres-db-name" env:"POSTGRES_DB_NAME" default:"mimicproxy"`
		User     string `long:"postgres-user" env:"POSTGRES_USER" default:"postgres"`
		Password string `long:"postgres-password" env:"POSTGRES_PASSWORD"`
		Host     string `long:"postgres-host" env:"POSTGRES_HOST" default:"localhost"`
		Port     int    `long:"postgres-port" env:"POSTGRES_PORT" default:"5432"`
		SSLMode  string `long:"postgres-ssl-mode" env:"POSTGRES_SSL_MODE" default:"disable"`
	}

	Redis struct {
		DSN  string `long:"redis-dsn" env:"REDIS_DSN" default:"redis://localhost:6379" description:""`
		Host string `long:"redis-host" env:"REDIS_HOST" default:"localhost"`
		Port int    `long:"redis-port" env:"REDIS_PORT" default:"6379"`
		DB   struct {
			Purchase int `long:"redis-db-purchase" env:"REDIS_DB_PURCHASE" default:"1" description:""`
			Data     int `long:"redis-db-data" env:"REDIS_DB_DATA" default:"2" description:""`
			Proxy    int `long:"redis-db-proxy" env:"REDIS_DB_PROXY" default:"3" description:""`
		}
		Channel struct {
			User     string `long:"redis-ch-user" env:"REDIS_CH_USER" default:"user" description:""`
			Data     string `long:"redis-ch-data" env:"REDIS_CH_DATA" default:"data" description:""`
			Activity string `long:"redis-ch-activity" env:"REDIS_CH_ACTIVITY" default:"activity" description:""`
			Restart  string `long:"redis-ch-restart" env:"REDIS_CH_RESTART" default:"restart" description:""`
		}
	}

	HTTP struct {
		ReadBufferSize  int           `long:"http-read-buffer" env:"FASTHTTP_READ_BUFFER" default:"4096" description:""`
		WriteBufferSize int           `long:"http-write-buffer" env:"FASTHTTP_WRITE_BUFFER" default:"4096" description:""`
		ReadTimeout     time.Duration `long:"http-read-timeout" env:"FASTHTTP_READ_TIMEOUT" default:"15s" description:""`
		WriteTimeout    time.Duration `long:"http-write-timeout" env:"FASTHTTP_WRITE_TIMEOUT" default:"15s" description:""`
		IdleTimeout     time.Duration `long:"http-idle-timeout" env:"FASTHTTP_IDLE_TIMEOUT" default:"30s" description:""`
		SSLDomain       string        `long:"http-ssl-domain" env:"FASTHTTP_SSL_DOMAIN" default:""`
		SSLCache        string        `long:"http-ssl-cache" env:"FASTHTTP_SSL_CACHE" default:"./certs"`
	}

	Proxy struct {
		PortHTTP     int           `long:"proxy-port-http" env:"PROXY_PORT_HTTP" default:"8080" description:""`
		BufferSize   int           `long:"proxy-buffer-size" env:"PROXY_BUFFER_SIZE" default:"4096" description:""`
		ReadDeadline time.Duration `long:"proxy-read-deadline" env:"PROXY_READ_DEADLINE" default:"30s" description:""`
		DialTimeout  time.Duration `long:"proxy-dial-timeout" env:"PROXY_DIAL_TIMEOUT" default:"10s" description:""`
	}

	Provider struct {
		Static struct {
			SyncPeriod time.Duration `long:"provider-sync-period" env:"PROVIDER_SYNC_PERIOD" default:"1m"`
		}

		TTProxy struct {
			BaseURL          string `long:"provider-ttp-base-url" env:"PROVIDER_TTP_BASEURL" default:"https://api.ttproxy.com/v1/subLicense/"`
			License          string `long:"provider-ttp-license" env:"PROVIDER_TTP_LICENSE"`
			Secret           string `long:"provider-ttp-secret" env:"PROVIDER_TTP_SECRET"`
			ProxyCredentials struct {
				Host string `long:"provider-ttp-proxy-cred-host" env:"PROVIDER_TTP_PROXY_CRED_HOST" default:"dynamic.ttproxy.com"`
				Port int    `long:"provider-ttp-proxy-cred-port" env:"PROVIDER_TTP_PROXY_CRED_PORT" default:"10001"`
			}
		}
		DataImpulse struct {
			BaseURL          string `long:"provider-di-base-url" env:"PROVIDER_DI_BASEURL" default:"https://api.dataimpulse.com/provider/"`
			Login            string `long:"provider-di-login" env:"PROVIDER_DI_LOGIN"`
			Password         string `long:"provider-di-password" env:"PROVIDER_DI_PASSWORD"`
			ProxyCredentials struct {
				Host string `long:"provider-di-proxy-cred-host" env:"PROVIDER_DI_PROXY_CRED_HOST" default:"gw.dataimpulse.com"`
				Port int    `long:"provider-di-proxy-cred-port" env:"PROVIDER_DI_PROXY_CRED_PORT" default:"823"`
			}
		}
		Proxyverse struct {
			ProxyCredentials struct {
				Host     string `long:"provider-pv-proxy-cred-host" env:"PROVIDER_PV_PROXY_CRED_HOST" default:"51.81.93.42"`
				Port     int    `long:"provider-pv-proxy-cred-port" env:"PROVIDER_PV_PROXY_CRED_PORT" default:"9200"`
				Username string `long:"provider-pv-proxy-cred-username" env:"PROVIDER_PV_PROXY_CRED_USERNAME"`
				Password string `long:"provider-pv-proxy-cred-password" env:"PROVIDER_PV_PROXY_CRED_PASSWORD"`
			}
		}
		Databay struct {
			ProxyCredentials struct {
				Host     string `long:"provider-db-proxy-cred-host" env:"PROVIDER_DB_PROXY_CRED_HOST" default:"resi-global-gateways.databay.com"`
				Port     int    `long:"provider-db-proxy-cred-port" env:"PROVIDER_DB_PROXY_CRED_PORT" default:"7676"`
				Username string `long:"provider-db-proxy-cred-username" env:"PROVIDER_DB_PROXY_CRED_USERNAME"`
				Password string `long:"provider-db-proxy-cred-password" env:"PROVIDER_DB_PROXY_CRED_PASSWORD"`
			}
		}
	}

	Sync struct {
		Activity time.Duration `long:"sync-activity" env:"SYNC_ACTIVITY" default:"1s"`
		Data     time.Duration `long:"sync-data" env:"SYNC_DATA" default:"1s"`
	}

	Session struct {
		CacheSize   int           `long:"session-cache-size" env:"SESSION_CACHE_SIZE" default:"10000" description:""`
		Duration    time.Duration `long:"session-duration" env:"SESSION_DURATION" default:"10m" description:""`
		DurationMax time.Duration `long:"session-duration-max" env:"SESSION_DURATION_MAX" default:"20m" description:""`
	}

	Accountant struct {
		Bytes int64 `long:"accountant-bytes" env:"ACCOUNTANT_BYTES" default:"256000" description:""`
	}

	Authorization struct {
		CacheSize int           `long:"authorization-cache-size" env:"AUTHORIZATION_CACHE_SIZE" default:"1000" description:""`
		TTL       time.Duration `long:"authorization-ttl" env:"AUTHORIZATION_TTL" default:"5m" description:""`
	}
}
