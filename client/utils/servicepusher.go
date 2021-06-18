package utils

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/go-ping/ping"
	"github.com/projecteru2/core/log"
)

// EndpointPusher pushes endpoints to registered channels if the ep is L3 reachable
type EndpointPusher struct {
	chans              []chan []string
	pendingEndpoints   sync.Map
	availableEndpoints sync.Map
}

// NewEndpointPusher .
func NewEndpointPusher() *EndpointPusher {
	return &EndpointPusher{}
}

// Register registers a channel that will receive the endpoints later
func (p *EndpointPusher) Register(ch chan []string) {
	p.chans = append(p.chans, ch)
}

// Push pushes endpoint candicates
func (p *EndpointPusher) Push(endpoints []string) {
	p.delOutdated(endpoints)
	p.addCheck(endpoints)
}

func (p *EndpointPusher) delOutdated(endpoints []string) {
	newEps := make(map[string]struct{})
	for _, e := range endpoints {
		newEps[e] = struct{}{}
	}

	p.pendingEndpoints.Range(func(key, value interface{}) bool {
		ep, ok := key.(string)
		if !ok {
			log.Error("[EruResolver] failed to cast key while ranging pendingEndpoints")
			return true
		}
		cancel, ok := value.(context.CancelFunc)
		if !ok {
			log.Error("[EruResolver] failed to cast value while ranging pendingEndpoints")
		}
		if _, ok := newEps[ep]; !ok {
			cancel()
			p.pendingEndpoints.Delete(ep)
			log.Debugf(context.TODO(), "[EruResolver] pending endpoint deleted: %s", ep)
		}
		return true
	})

	p.availableEndpoints.Range(func(key, _ interface{}) bool {
		ep, ok := key.(string)
		if !ok {
			log.Error("[EruResolver] failed to cast key while ranging availableEndpoints")
			return true
		}
		if _, ok := newEps[ep]; !ok {
			p.availableEndpoints.Delete(ep)
			log.Debugf(context.TODO(), "[EruResolver] available endpoint deleted: %s", ep)
		}
		return true
	})
}

func (p *EndpointPusher) addCheck(endpoints []string) {
	for _, endpoint := range endpoints {
		if _, ok := p.pendingEndpoints.Load(endpoint); ok {
			continue
		}
		if _, ok := p.availableEndpoints.Load(endpoint); ok {
			continue
		}

		ctx, cancel := context.WithCancel(context.Background())
		p.pendingEndpoints.Store(endpoint, cancel)
		go p.pollReachability(ctx, endpoint)
		log.Debugf(ctx, "[EruResolver] pending endpoint added: %s", endpoint)
	}
}
func (p *EndpointPusher) pollReachability(ctx context.Context, endpoint string) {
	parts := strings.Split(endpoint, ":")
	if len(parts) != 2 {
		log.Errorf(context.TODO(), "[EruResolver] wrong format of endpoint: %s", endpoint)
		return
	}

	for {
		select {
		case <-ctx.Done():
			log.Debugf(ctx, "[EruResolver] reachability goroutine ends: %s", endpoint)
			return
		default:
		}

		time.Sleep(time.Second)
		if err := p.checkReachability(parts[0]); err != nil {
			continue
		}

		p.pendingEndpoints.Delete(endpoint)
		p.availableEndpoints.Store(endpoint, struct{}{})
		p.pushEndpoints()
		log.Debugf(ctx, "[EruResolver] available endpoint added: %s", endpoint)
		return
	}
}

func (p *EndpointPusher) checkReachability(host string) (err error) {
	pinger, err := ping.NewPinger(host)
	if err != nil {
		log.Errorf(context.TODO(), "[EruResolver] failed to create pinger: %+v", err)
		return
	}
	defer pinger.Stop()

	pinger.Count = 1
	pinger.Timeout = time.Second
	if err = pinger.Run(); err != nil {
		return
	}
	if pinger.Statistics().PacketsRecv != 1 {
		return errors.New("icmp packet lost")
	}
	return
}

func (p *EndpointPusher) pushEndpoints() {
	endpoints := []string{}
	p.availableEndpoints.Range(func(key, value interface{}) bool {
		endpoint, ok := key.(string)
		if !ok {
			log.Error("[EruResolver] failed to cast key while ranging availableEndpoints")
			return true
		}
		endpoints = append(endpoints, endpoint)
		return true
	})
	for _, ch := range p.chans {
		ch <- endpoints
	}
}
