package utils

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cornelk/hashmap"
	"github.com/go-ping/ping"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"golang.org/x/exp/slices"
)

// EndpointPusher pushes endpoints to registered channels if the ep is L3 reachable
type EndpointPusher struct {
	sync.Mutex
	chans              []chan []string
	pendingEndpoints   *hashmap.Map[string, context.CancelFunc]
	availableEndpoints *hashmap.Map[string, struct{}]
}

// NewEndpointPusher .
func NewEndpointPusher() *EndpointPusher {
	return &EndpointPusher{
		pendingEndpoints:   hashmap.New[string, context.CancelFunc](),
		availableEndpoints: hashmap.New[string, struct{}](),
	}
}

// Register registers a channel that will receive the endpoints later
func (p *EndpointPusher) Register(ch chan []string) {
	p.chans = append(p.chans, ch)
}

// Push pushes endpoint candicates
func (p *EndpointPusher) Push(ctx context.Context, endpoints []string) {
	p.delOutdated(ctx, endpoints)
	p.addCheck(ctx, endpoints)
}

func (p *EndpointPusher) delOutdated(ctx context.Context, endpoints []string) {
	p.Lock()
	defer p.Unlock()

	p.pendingEndpoints.Range(func(endpoint string, cancel context.CancelFunc) bool {
		if !slices.Contains(endpoints, endpoint) {
			cancel()
			p.pendingEndpoints.Del(endpoint)
			log.Debugf(ctx, "[EruResolver] pending endpoint deleted: %s", endpoint)
		}
		return true
	})

	p.availableEndpoints.Range(func(endpoint string, _ struct{}) bool {
		if !slices.Contains(endpoints, endpoint) {
			p.availableEndpoints.Del(endpoint)
			log.Debugf(ctx, "[EruResolver] available endpoint deleted: %s", endpoint)
		}
		return true
	})
}

func (p *EndpointPusher) addCheck(ctx context.Context, endpoints []string) {
	for _, endpoint := range endpoints {
		if _, ok := p.pendingEndpoints.Get(endpoint); ok {
			continue
		}
		if _, ok := p.availableEndpoints.Get(endpoint); ok {
			continue
		}

		ctx, cancel := context.WithCancel(ctx)
		p.pendingEndpoints.Set(endpoint, cancel)
		go p.pollReachability(ctx, endpoint)
		log.Debugf(ctx, "[EruResolver] pending endpoint added: %s", endpoint)
	}
}

func (p *EndpointPusher) pollReachability(ctx context.Context, endpoint string) {
	parts := strings.Split(endpoint, ":")
	if len(parts) != 2 {
		log.Errorf(ctx, types.ErrInvalidType, "[EruResolver] wrong format of endpoint: %s", endpoint)
		return
	}

	ticker := time.NewTicker(time.Second) // TODO config from outside?
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Debugf(ctx, "[EruResolver] reachability goroutine ends: %s", endpoint)
			return
		case <-ticker.C:
			p.Lock()
			defer p.Unlock()
			if err := p.checkReachability(ctx, parts[0]); err != nil {
				continue
			}
			p.pendingEndpoints.Del(endpoint)
			p.availableEndpoints.Set(endpoint, struct{}{})
			p.pushEndpoints()
			log.Debugf(ctx, "[EruResolver] available endpoint added: %s", endpoint)
			return
		}
	}
}

func (p *EndpointPusher) checkReachability(ctx context.Context, host string) (err error) {
	pinger, err := ping.NewPinger(host)
	if err != nil {
		log.Error(ctx, err, "[EruResolver] failed to create pinger")
		return
	}
	pinger.SetPrivileged(os.Getuid() == 0)
	defer pinger.Stop()

	pinger.Count = 1
	pinger.Timeout = time.Second
	if err = pinger.Run(); err != nil {
		return
	}
	if pinger.Statistics().PacketsRecv != 1 {
		return types.ErrICMPLost
	}
	return
}

func (p *EndpointPusher) pushEndpoints() {
	endpoints := []string{}
	p.availableEndpoints.Range(func(endpoint string, _ struct{}) bool {
		endpoints = append(endpoints, endpoint)
		return true
	})
	for _, ch := range p.chans {
		ch <- endpoints
	}
}
