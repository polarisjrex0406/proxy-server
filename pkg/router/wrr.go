package router

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/omimic12/proxy-server/pkg"
	"github.com/omimic12/proxy-server/pkg/provider"
	"github.com/omimic12/proxy-server/pkg/roundrobin"
	"github.com/omimic12/proxy-server/pkg/zerocopy"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type WeightedRoundRobin struct {
	fetchTimeout time.Duration
	settings     pkg.Settings

	roundRobin *roundrobin.RoundRobin
	providers  map[string]pkg.Provider

	ipStatic      []pkg.Provider
	ipBackconnect []pkg.Provider
	ipReseller    []pkg.Provider

	logger *zap.Logger
}

type Proxy struct {
	Type     string `json:"type"`
	Protocol string `json:"protocol"`
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     int    `json:"port"`

	PurchaseID uint   `json:"purchase_id"`
	Region     string `json:"region"`
	Reseller   string `json:"reseller"`
}

func NewWeightedRoundRobin(
	settings pkg.Settings,
	dialTimeout time.Duration,
	dialReadDeadline time.Duration,
	fetchTimeout time.Duration,
	proxySyncPeriod time.Duration,
	redisProxy *redis.Client,
	logger *zap.Logger,
) (*WeightedRoundRobin, error) {
	w := &WeightedRoundRobin{
		settings:     settings,
		logger:       logger,
		fetchTimeout: fetchTimeout,
	}

	go func() {
		ticker := time.NewTicker(proxySyncPeriod)
		defer ticker.Stop()

		var synchronize = func() {
			keys, err := redisProxy.Keys(context.Background(), "*").Result()
			if err != nil {
				logger.Error("failed to get proxy keys", zap.Error(err))
				return
			}

			var ipStatic = make([]pkg.Provider, 0)
			var ipBackconnect = make([]pkg.Provider, 0)
			var ipReseller = make([]pkg.Provider, 0)

			for _, key := range keys {
				data, err := redisProxy.Get(context.Background(), key).Bytes()
				if err != nil {
					logger.Error("failed to get proxy", zap.String("key", key), zap.Error(err))
					continue
				}

				var proxy = new(Proxy)
				err = json.Unmarshal(data, proxy)
				if err != nil {
					logger.Error("failed to unmarshal proxy", zap.Error(err))
					continue
				}

				p, err := proxyToProvider(dialTimeout, dialReadDeadline, proxy)
				if err != nil {
					logger.Error("failed to convert proxy to provider", zap.Error(err))
					continue
				}

				switch proxy.Type {
				case "static":
					ipStatic = append(ipStatic, p)
				case "backconnect":
					ipBackconnect = append(ipStatic, p)
				case "provider":
					ipReseller = append(ipStatic, p)
				default:
					logger.Error("unsupported proxy type " + string(proxy.Type))
				}
			}

			w.ipStatic = ipStatic
			w.ipBackconnect = ipBackconnect
			w.ipReseller = ipReseller

			logger.Debug("proxies sync: done",
				zap.Int("static", len(w.ipStatic)),
				zap.Int("backconnect", len(w.ipBackconnect)),
				zap.Int("reseller", len(w.ipReseller)))
		}

		synchronize()

		for {
			<-ticker.C
			synchronize()
		}
	}()

	// return w, w.synchronize(true)
	return w, nil
}

func (r *WeightedRoundRobin) Route(purchase *pkg.Purchase, request *pkg.Request) (pkg.Provider, error) {
	return r.selectIP(purchase, request)

	// if request.IP != nil {
	// 	return r.selectIP(request)
	// }

	// return r.selectProvider(request)
}

func (r *WeightedRoundRobin) synchronize(fetch bool) error {
	var providers = make([]pkg.Provider, 0)

	if fetch {
		var err error
		ctx, cancel := context.WithTimeout(context.Background(), r.fetchTimeout)
		defer cancel()

		providers, err = r.settings.LoadProviders(ctx)
		if err != nil {
			return err
		}

		r.logger.Info(fmt.Sprintf("LoadProviders: %d", len(providers)))
	} else {
		for _, f := range r.providers {
			providers = append(providers, f)
		}
	}

	if len(providers) == 0 {
		return errors.New("providers response is empty")
	}

	roundRobin := roundrobin.NewRoundRobin(providers)
	r.providers = make(map[string]pkg.Provider)
	for _, p := range providers {
		r.providers[p.Name()] = p
	}

	r.roundRobin = roundRobin
	return nil
}

func (r *WeightedRoundRobin) selectIP(purchase *pkg.Purchase, request *pkg.Request) (pkg.Provider, error) {
	var p pkg.Provider
	if purchase.Type == "static" {
		i := 0
		if len(r.ipStatic) > 0 {
			source := rand.NewSource(time.Now().UnixNano())
			rs := rand.New(source)
			max := len(r.ipStatic) - 1
			i = rs.Intn(max-0) + 0
		} else if len(r.ipStatic) == 0 {
			return nil, pkg.ErrIPNotFound
		}
		return r.ipStatic[i], nil
	}
	if purchase.Type == "backconnect" {
		i := 0
		if len(r.ipBackconnect) > 0 {
			source := rand.NewSource(time.Now().UnixNano())
			rs := rand.New(source)
			max := len(r.ipBackconnect) - 1
			i = rs.Intn(max-0) + 0
		} else if len(r.ipBackconnect) == 0 {
			return nil, pkg.ErrIPNotFound
		}
		return r.ipBackconnect[i], nil
	}
	if purchase.Type == "provider" {
	}
	return p, nil
}

func (r *WeightedRoundRobin) selectProvider(request *pkg.Request) (pkg.Provider, error) {
	if len(r.providers) == 1 {
		for _, p := range r.providers {
			return p, nil
		}
	}

	var max = r.roundRobin.Size()
	if max > 1 {
		max = max * 2
	}

	//var exclude = make([]pkg.Provider, 0, max)
	var i, blocked int
	for {
		if i >= max {
			if blocked >= i {
				return nil, pkg.ErrDomainBlocked
			}

			return nil, pkg.ErrFailedSelectProvider
		}

		//select p
		p, err := r.roundRobin.GetProvider()
		if err != nil {
			return nil, err
		}

		if !p.HasFeatures(request.Features...) {
			//exclude = append(exclude, p)
			i++
			continue
		}

		if !p.HasRoutes(request.Routes...) {
			//exclude = append(exclude, p)
			i++
			continue
		}

		if request.Country != nil {
			if !p.HasCountry(zerocopy.String(request.Country)) {
				//exclude = append(exclude, p)
				i++
				continue
			}
		}

		if request.Region != nil {
			if !p.HasRegion(zerocopy.String(request.Region)) {
				//exclude = append(exclude, p)
				i++
				continue
			}
		}

		if request.City != nil {
			if !p.HasCity(zerocopy.String(request.City)) {
				//exclude = append(exclude, p)
				i++
				continue
			}
		}

		return p, nil
	}
}

func proxyToProvider(dialTimeout, readDeadline time.Duration, proxy *Proxy) (pkg.Provider, error) {
	var p pkg.Provider
	switch proxy.Type {
	case "static":
		return provider.NewStatic(
			fmt.Sprintf("%s:%d", proxy.Host, proxy.Port),
			zerocopy.Bytes(proxy.Username),
			zerocopy.Bytes(proxy.Password),
			1,
			"static",
			pkg.Protocol(proxy.Protocol),
		)
	case "backconnect":
		return nil, nil
	case "provider":
		switch proxy.Reseller {
		case "ttproxy":
			p = provider.NewTTProxy(
				zerocopy.Bytes(proxy.Username),
				zerocopy.Bytes(proxy.Password),
				1,
				pkg.Protocol(proxy.Protocol),
			)
		case "dataimpulse":
			p = provider.NewDataImpulse(
				zerocopy.Bytes(proxy.Username),
				zerocopy.Bytes(proxy.Password),
				1,
				pkg.Protocol(proxy.Protocol),
			)
		case "proxyverse":
			p = provider.NewProxyverse(
				zerocopy.Bytes(proxy.Password),
				1,
				pkg.Protocol(proxy.Protocol),
			)
		case "databay":
		default:
			return nil, fmt.Errorf("wrong reseller %s", proxy.Reseller)
		}
		return p, nil
	default:
		return nil, fmt.Errorf("wrong proxy type %s", proxy.Type)
	}
}
