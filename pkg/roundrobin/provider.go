package roundrobin

import (
	"errors"
	"math/big"
	"sync"

	"github.com/omimic12/proxy-server/pkg"
)

func NewRoundRobin(nodes []pkg.Provider) *RoundRobin {
	var rr = new(RoundRobin)
	m := make(map[int]pkg.Provider)
	for i, v := range nodes {
		m[i] = v
	}

	rr.nodes = m
	rr.lastNodeIndex = -1
	if len(m) == 0 {
		return rr
	}

	rr.currentNodeWeight = m[0].Weight()

	var weights []uint64
	for _, v := range nodes {
		weights = append(weights, v.Weight())
	}
	rr.weightGCD = calcGCD(weights...)
	return rr
}

type RoundRobin struct {
	mu                sync.RWMutex
	nodes             map[int]pkg.Provider
	lastNodeIndex     int
	currentNodeWeight uint64
	weightGCD         uint64
}

func (rr *RoundRobin) SetNode(index int, node pkg.Provider) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	rr.nodes[index] = node
	rr.weightGCD = calcGCD(rr.getWeights()...)
}

func (rr *RoundRobin) DeleteNode(provider pkg.Provider) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	var index int
	for i, p := range rr.nodes {
		if p.Name() != provider.Name() {
			continue
		}

		index = i
		break
	}

	if index <= -1 {
		return
	}

	delete(rr.nodes, index)
	rr.weightGCD = calcGCD(rr.getWeights()...)
}

func (rr *RoundRobin) GetProvider() (pkg.Provider, error) {
	rr.mu.RLock()
	defer rr.mu.RUnlock()
	for {
		if len(rr.nodes) == 0 {
			return nil, errors.New("providers were not found")
		} else if len(rr.nodes) == 1 {
			for _, node := range rr.nodes {
				return node, nil
			}
		}

		rr.lastNodeIndex = (rr.lastNodeIndex + 1) % len(rr.nodes)

		if rr.lastNodeIndex == 0 {
			rr.currentNodeWeight = rr.currentNodeWeight - rr.weightGCD

			if rr.currentNodeWeight <= 0 {
				rr.currentNodeWeight = rr.getMaxWeight()

				if rr.currentNodeWeight == 0 {
					return nil, errors.New("current node weight is 0")
				}
			}
		}

		if weight := rr.nodes[rr.lastNodeIndex].Weight(); weight >= rr.currentNodeWeight {
			return rr.nodes[rr.lastNodeIndex], nil
		}
	}
}

func (rr *RoundRobin) Size() int {
	rr.mu.RLock()
	defer rr.mu.RUnlock()

	return len(rr.nodes)
}

func calcGCD(values ...uint64) uint64 {
	if len(values) == 0 {
		return 0
	}

	z := values[0]
	for _, n := range values {
		z = gcd(n, z)
	}
	return z
}

func gcd(m, n uint64) uint64 {
	x := new(big.Int)
	y := new(big.Int)
	z := new(big.Int)
	a := new(big.Int).SetUint64(m)
	b := new(big.Int).SetUint64(n)
	return z.GCD(x, y, a, b).Uint64()
}

func (rr *RoundRobin) getMaxWeight() uint64 {
	var max uint64
	for _, v := range rr.nodes {
		if v.Weight() >= max {
			max = v.Weight()
		}
	}

	return max
}

func (rr *RoundRobin) getWeights() []uint64 {
	var weights = make([]uint64, len(rr.nodes))
	for _, v := range rr.nodes {
		weights = append(weights, v.Weight())
	}
	return weights
}
