package router

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/omimic12/proxy-server/pkg"
	"github.com/omimic12/proxy-server/pkg/dialer"
	"github.com/omimic12/proxy-server/pkg/provider"
	"github.com/omimic12/proxy-server/pkg/zerocopy"
	"go.uber.org/zap"
)

type WeightedRoundRobin struct {
	fetchTimeout time.Duration
	settings     pkg.Settings

	ipStatic           map[string]pkg.Provider
	ipStaticSlice      []pkg.Provider
	ipBackconnectSlice []pkg.Provider
	resellerSlice      []pkg.Provider

	lastResellerIndexes map[uint]int

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

			var ipStatic = make(map[string]pkg.Provider)
			var ipStaticSlice = make([]pkg.Provider, 0)
			var ipBackconnectSlice = make([]pkg.Provider, 0)
			var resellerSlice = make([]pkg.Provider, 0)
			var lastResellerIndexes = make(map[uint]int)

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
					ipStaticSlice = append(ipStaticSlice, p)
					ipStatic[proxy.Host] = p
				case "backconnect":
					ipBackconnectSlice = append(ipBackconnectSlice, p)
				case "provider":
					resellerSlice = append(resellerSlice, p)
					lastResellerIndexes[proxy.PurchaseID] = -1
				default:
					logger.Error("unsupported proxy type " + string(proxy.Type))
				}
			}

			w.ipStatic = ipStatic
			w.ipStaticSlice = ipStaticSlice
			w.ipBackconnectSlice = ipBackconnectSlice
			w.resellerSlice = resellerSlice
			w.lastResellerIndexes = lastResellerIndexes

			logger.Debug("proxies sync: done",
				zap.Int("static", len(w.ipStaticSlice)),
				zap.Int("backconnect", len(w.ipBackconnectSlice)),
				zap.Int("reseller", len(w.resellerSlice)))
		}

		synchronize()

		for {
			<-ticker.C
			synchronize()
		}
	}()

	return w, nil
}

func (r *WeightedRoundRobin) Route(purchase *pkg.Purchase, request *pkg.Request) (pkg.Provider, error) {
	return r.selectIP(purchase, request)

	// if request.IP != nil {
	// 	return r.selectIP(request)
	// }

	// return r.selectProvider(request)
}

func (r *WeightedRoundRobin) selectIP(purchase *pkg.Purchase, request *pkg.Request) (pkg.Provider, error) {
	if purchase.Type == "static" {
		i := 0
		if len(r.ipStaticSlice) > 0 {
			source := rand.NewSource(time.Now().UnixNano())
			rs := rand.New(source)
			max := len(r.ipStaticSlice)
			i = rs.Intn(max-0) + 0
		} else if len(r.ipStaticSlice) == 0 {
			return nil, pkg.ErrIPNotFound
		}
		return r.ipStaticSlice[i], nil
	}
	if purchase.Type == "backconnect" {
		i := 0
		if len(r.ipBackconnectSlice) > 0 {
			source := rand.NewSource(time.Now().UnixNano())
			rs := rand.New(source)
			max := len(r.ipBackconnectSlice)
			i = rs.Intn(max-0) + 0
		} else if len(r.ipBackconnectSlice) == 0 {
			return nil, pkg.ErrIPNotFound
		}
		return r.ipBackconnectSlice[i], nil
	}
	if purchase.Type == "provider" {
		var resellerPurchased = make([]pkg.Provider, 0)

		for _, reseller := range r.resellerSlice {
			if reseller.PurchasedBy() == purchase.ID {
				resellerPurchased = append(resellerPurchased, reseller)
			}
		}

		i := 0
		if len(resellerPurchased) > 0 {
			lastIndex := r.lastResellerIndexes[purchase.ID]
			if lastIndex >= len(resellerPurchased)-1 {
				i = 0
			} else {
				i = lastIndex + 1
			}
			r.lastResellerIndexes[purchase.ID] = i
		} else if len(resellerPurchased) == 0 {
			return nil, pkg.ErrIPNotFound
		}
		return resellerPurchased[i], nil
	}
	return nil, pkg.ErrPurchaseNotFound
}

// func (r *WeightedRoundRobin) selectProvider(request *pkg.Request) (pkg.Provider, error) {
// 	if len(r.providers) == 1 {
// 		for _, p := range r.providers {
// 			return p, nil
// 		}
// 	}

// 	var max = r.roundRobin.Size()
// 	if max > 1 {
// 		max = max * 2
// 	}

// 	//var exclude = make([]pkg.Provider, 0, max)
// 	var i, blocked int
// 	for {
// 		if i >= max {
// 			if blocked >= i {
// 				return nil, pkg.ErrDomainBlocked
// 			}

// 			return nil, pkg.ErrFailedSelectProvider
// 		}

// 		//select p
// 		p, err := r.roundRobin.GetProvider()
// 		if err != nil {
// 			return nil, err
// 		}

// 		if !p.HasFeatures(request.Features...) {
// 			//exclude = append(exclude, p)
// 			i++
// 			continue
// 		}

// 		if !p.HasRoutes(request.Routes...) {
// 			//exclude = append(exclude, p)
// 			i++
// 			continue
// 		}

// 		if request.Country != nil {
// 			if !p.HasCountry(zerocopy.String(request.Country)) {
// 				//exclude = append(exclude, p)
// 				i++
// 				continue
// 			}
// 		}

// 		if request.Region != nil {
// 			if !p.HasRegion(zerocopy.String(request.Region)) {
// 				//exclude = append(exclude, p)
// 				i++
// 				continue
// 			}
// 		}

// 		if request.City != nil {
// 			if !p.HasCity(zerocopy.String(request.City)) {
// 				//exclude = append(exclude, p)
// 				i++
// 				continue
// 			}
// 		}

// 		return p, nil
// 	}
// }

func proxyToProvider(dialTimeout, readDeadline time.Duration, proxy *Proxy) (pkg.Provider, error) {
	var p pkg.Provider
	var d pkg.Dialer = dialer.NewHTTP(dialTimeout, readDeadline)
	switch proxy.Type {
	case "static":
		return provider.NewStatic(
			fmt.Sprintf("%s:%d", proxy.Host, proxy.Port),
			zerocopy.Bytes(proxy.Username),
			zerocopy.Bytes(proxy.Password),
			1,
			"static",
			pkg.Protocol(proxy.Protocol),
			d,
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
				d,
				proxy.PurchaseID,
			)
		case "dataimpulse":
			p = provider.NewDataImpulse(
				zerocopy.Bytes(proxy.Username),
				zerocopy.Bytes(proxy.Password),
				1,
				pkg.Protocol(proxy.Protocol),
				d,
				proxy.PurchaseID,
			)
		case "proxyverse":
			p = provider.NewProxyverse(
				zerocopy.Bytes(proxy.Password),
				1,
				pkg.Protocol(proxy.Protocol),
				d,
				proxy.PurchaseID,
			)
		case "databay":
			p = provider.NewDatabay(
				zerocopy.Bytes(proxy.Username),
				zerocopy.Bytes(proxy.Password),
				1,
				pkg.Protocol(proxy.Protocol),
				d,
				proxy.PurchaseID,
			)
		default:
			return nil, fmt.Errorf("wrong reseller %v", *proxy)
		}
		return p, nil
	default:
		return nil, fmt.Errorf("wrong proxy type %s", proxy.Type)
	}
}
