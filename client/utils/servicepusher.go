package utils

import (
	"context"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/alphadose/haxmap"
	probing "github.com/prometheus-community/pro-bing"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// EndpointPusher pushes endpoints to registered channels once they are L3 reachable.
type EndpointPusher struct {
	sync.Mutex
	chans              []chan []string
	pendingEndpoints   *haxmap.Map[string, context.CancelFunc]
	availableEndpoints *haxmap.Map[string, struct{}]
}

func NewEndpointPusher() *EndpointPusher {
	return &EndpointPusher{
		pendingEndpoints:   haxmap.New[string, context.CancelFunc](),
		availableEndpoints: haxmap.New[string, struct{}](),
	}
}

func (p *EndpointPusher) Register(ch chan []string) {
	p.chans = append(p.chans, ch)
}

func (p *EndpointPusher) Push(ctx context.Context, endpoints []string) {
	p.delOutdated(ctx, endpoints)
	p.addCheck(ctx, endpoints)
}

func (p *EndpointPusher) delOutdated(ctx context.Context, endpoints []string) {
	p.Lock()
	defer p.Unlock()
	logger := log.WithFunc("utils.EndpointPusher.delOutdated")
	p.pendingEndpoints.ForEach(func(endpoint string, cancel context.CancelFunc) bool {
		if !slices.Contains(endpoints, endpoint) {
			cancel()
			p.pendingEndpoints.Del(endpoint)
			logger.Debugf(ctx, "pending endpoint deleted: %s", endpoint)
		}
		return true
	})

	p.availableEndpoints.ForEach(func(endpoint string, _ struct{}) bool {
		if !slices.Contains(endpoints, endpoint) {
			p.availableEndpoints.Del(endpoint)
			logger.Debugf(ctx, "available endpoint deleted: %s", endpoint)
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

		pollCtx, cancel := context.WithCancel(ctx)
		p.pendingEndpoints.Set(endpoint, cancel)
		go p.pollReachability(pollCtx, endpoint)
		log.WithFunc("utils.EndpointPusher.addCheck").Debugf(ctx, "pending endpoint added: %s", endpoint)
	}
}

func (p *EndpointPusher) pollReachability(ctx context.Context, endpoint string) {
	logger := log.WithFunc("utils.EndpointPusher.pollReachability")
	parts := strings.Split(endpoint, ":")
	if len(parts) != 2 {
		logger.Warnf(ctx, "wrong endpoint format: %s", endpoint)
		return
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Debugf(ctx, "reachability goroutine ends: %s", endpoint)
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
			logger.Debugf(ctx, "available endpoint added: %s", endpoint)
			return
		}
	}
}

func (p *EndpointPusher) checkReachability(ctx context.Context, host string) (err error) {
	pinger, err := probing.NewPinger(host)
	if err != nil {
		log.WithFunc("utils.EndpointPusher.checkReachability").Error(ctx, err, "create pinger")
		return err
	}
	pinger.SetPrivileged(os.Getuid() == 0)
	defer pinger.Stop()

	pinger.Count = 1
	pinger.Timeout = time.Second
	if err = pinger.Run(); err != nil {
		return err
	}
	if pinger.Statistics().PacketsRecv != 1 {
		return types.ErrICMPLost
	}
	return err
}

func (p *EndpointPusher) pushEndpoints() {
	endpoints := []string{}
	p.availableEndpoints.ForEach(func(endpoint string, _ struct{}) bool {
		endpoints = append(endpoints, endpoint)
		return true
	})
	for _, ch := range p.chans {
		ch <- endpoints
	}
}
