package tracker

import (
	"context"
	"strings"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/omimic12/proxy-server/pkg"
	"go.uber.org/zap"
)

type Map struct {
	mu        sync.RWMutex
	requests  map[string]chan<- struct{}
	purchases map[string]int64
	client    *redis.Client

	logger *zap.Logger
}

func NewMap(client *redis.Client, logger *zap.Logger) *Map {
	return &Map{
		mu:        sync.RWMutex{},
		client:    client,
		logger:    logger,
		purchases: make(map[string]int64),
		requests:  make(map[string]chan<- struct{}),
	}
}

func (r *Map) Watch(requestID string, purchaseUUID string, ch chan<- struct{}) int64 {
	r.mu.Lock()
	r.requests[requestID] = ch
	threads, ok := r.purchases[purchaseUUID]
	if !ok {
		threads = 0
	}

	threads++
	r.purchases[purchaseUUID] = threads
	r.mu.Unlock()

	return threads
}

func (r *Map) Stop(requestID string, purchaseUUID string) int64 {
	r.mu.Lock()
	d, ok := r.requests[requestID]
	if !ok {
		r.mu.Unlock()
		return 0
	}

	d <- struct{}{}
	delete(r.requests, requestID)

	threads := r.purchases[purchaseUUID]
	threads -= 1

	if threads <= 0 {
		delete(r.purchases, purchaseUUID)
	} else {
		r.purchases[purchaseUUID] = threads
	}

	r.mu.Unlock()

	return threads
}

func (r *Map) Delete(requestID string, purchaseUUID string) int64 {
	r.mu.Lock()
	_, ok := r.requests[requestID]
	if !ok {
		r.mu.Unlock()
		return 0
	}

	delete(r.requests, requestID)

	threads := r.purchases[purchaseUUID]
	threads -= 1

	if threads <= 0 {
		delete(r.purchases, purchaseUUID)
	} else {
		r.purchases[purchaseUUID] = threads
	}

	r.mu.Unlock()

	return threads
}

func (r *Map) Threads() map[string]int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.purchases
}

func (r *Map) Close() error {
	r.mu.Lock()
	for key, value := range r.requests {
		value <- struct{}{}

		delete(r.requests, key)
	}
	r.mu.Unlock()

	return nil
}

func (r *Map) Listen(ctx context.Context, chUserInvalidate string) error {
	userInvalidate := r.client.Subscribe(ctx, chUserInvalidate)
	defer userInvalidate.Close() //nolint:errcheck

	for {
		select {
		case m := <-userInvalidate.Channel():
			requestKey := pkg.RequestKey(m.Payload, "")
			r.mu.Lock()
			for key, value := range r.requests {
				if !strings.HasPrefix(key, requestKey) {
					continue
				}

				value <- struct{}{}

				delete(r.requests, key)
			}
			r.mu.Unlock()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
