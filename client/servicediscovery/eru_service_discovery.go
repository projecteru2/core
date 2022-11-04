package servicediscovery

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/projecteru2/core/auth"
	"github.com/projecteru2/core/client/interceptor"
	"github.com/projecteru2/core/client/utils"
	"github.com/projecteru2/core/log"
	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"

	"google.golang.org/grpc"
)

// EruServiceDiscovery watches eru service status
type EruServiceDiscovery struct {
	endpoint   string
	authConfig types.AuthConfig
}

// New EruServiceDiscovery
func New(endpoint string, authConfig types.AuthConfig) *EruServiceDiscovery {
	return &EruServiceDiscovery{
		endpoint:   endpoint,
		authConfig: authConfig,
	}
}

// Watch .
func (w *EruServiceDiscovery) Watch(ctx context.Context) (_ <-chan []string, err error) {
	cc, err := w.dial(ctx, w.endpoint, w.authConfig)
	logger := log.WithFunc("servicediscovery.Watch").WithField("endpoint", w.endpoint)
	if err != nil {
		logger.Error(ctx, err, "dial failed")
		return
	}
	client := pb.NewCoreRPCClient(cc)
	ch := make(chan []string)
	epPusher := utils.NewEndpointPusher()
	epPusher.Register(ch)
	epPusher.Register(lbResolverBuilder.updateCh)
	go func() {
		defer close(ch)
		for {
			watchCtx, cancelWatch := context.WithCancel(ctx)
			stream, err := client.WatchServiceStatus(watchCtx, &pb.Empty{})
			if err != nil {
				logger.Error(ctx, err, "watch failed, try later")
				time.Sleep(10 * time.Second) // TODO can config
				continue
			}
			expectedInterval := time.Duration(math.MaxInt64) / time.Second

			for {
				cancelTimer := make(chan struct{})
				go func(expectedInterval time.Duration) {
					timer := time.NewTimer(expectedInterval * time.Second)
					defer timer.Stop()
					select {
					case <-timer.C:
						cancelWatch()
					case <-cancelTimer:
						return
					}
				}(expectedInterval)
				status, err := stream.Recv()
				close(cancelTimer)
				if err != nil {
					logger.Error(ctx, err, "recv failed")
					break
				}
				expectedInterval = time.Duration(status.GetIntervalInSecond())

				epPusher.Push(ctx, status.GetAddresses())
			}
		}
	}()

	return ch, nil
}

func (w *EruServiceDiscovery) dial(ctx context.Context, addr string, authConfig types.AuthConfig) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithStreamInterceptor(interceptor.NewStreamRetry(interceptor.RetryOptions{Max: 1})),
	}

	if authConfig.Username != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(auth.NewCredential(authConfig)))
	}

	target := makeServiceDiscoveryTarget(addr)
	return grpc.DialContext(ctx, target, opts...)
}

func makeServiceDiscoveryTarget(addr string) string {
	return fmt.Sprintf("lb://_/%s", addr)
}
