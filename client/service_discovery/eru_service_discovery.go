package service_discovery

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/projecteru2/core/client/interceptor"
	pb "github.com/projecteru2/core/rpc/gen"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type eruServiceDiscovery struct {
	endpoint string
}

func New(endpoint string) *eruServiceDiscovery {
	return &eruServiceDiscovery{
		endpoint: endpoint,
	}
}

func (w *eruServiceDiscovery) Watch(ctx context.Context) (_ <-chan []string, err error) {
	cc, err := w.dial(ctx, w.endpoint)
	if err != nil {
		log.Errorf("[EruServiceWatch] dial failed: %v", err)
		return
	}
	client := pb.NewCoreRPCClient(cc)
	ch := make(chan []string)
	go func() {
		defer close(ch)
		for {
			watchCtx, cancelWatch := context.WithCancel(ctx)
			stream, err := client.WatchServiceStatus(watchCtx, &pb.Empty{})
			if err != nil {
				log.Errorf("[EruServiceWatch] watch failed: %v", err)
				return
			}
			expectedInterval := time.Duration(math.MaxInt64) / time.Second

			for {
				cancelTimer := make(chan struct{})
				go func() {
					timer := time.NewTimer(expectedInterval * time.Second)
					defer timer.Stop()
					select {
					case <-timer.C:
						cancelWatch()
					case <-cancelTimer:
						return
					}

				}()
				status, err := stream.Recv()
				close(cancelTimer)
				if err != nil {
					log.Errorf("[EruServiceWatch] recv failed: %v", err)
					break
				}
				expectedInterval = time.Duration(status.GetIntervalInSecond())
				lbResolverBuilder.updateCh <- status.GetAddresses()
				ch <- status.GetAddresses()
			}
		}
	}()

	return ch, nil
}

func (w *eruServiceDiscovery) dial(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithBalancerName("round_robin"),
		grpc.WithStreamInterceptor(interceptor.NewStreamRetry(interceptor.RetryOptions{Max: 1})),
	}

	target := makeServiceDiscoveryTarget(addr)
	return grpc.DialContext(ctx, target, opts...)
}

func makeServiceDiscoveryTarget(addr string) string {
	return fmt.Sprintf("lb://_/%s", addr)
}
