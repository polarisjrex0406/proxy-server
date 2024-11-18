package config

import (
	"fmt"
	"sync"

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
		Host string `long:"redis-host" env:"REDIS_HOST" default:"localhost"`
		Port int    `long:"redis-port" env:"REDIS_PORT" default:"6379"`
	}

	Provider struct {
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
}
