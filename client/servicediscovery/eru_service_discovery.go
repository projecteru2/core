package servicediscovery

import (
	"context"
	"fmt"
	"math"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/projecteru2/core/auth"
	"github.com/projecteru2/core/client/interceptor"
	"github.com/projecteru2/core/client/utils"
	"github.com/projecteru2/core/log"
	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
)

// EruServiceDiscovery watches eru service status
type EruServiceDiscovery struct {
	endpoint   string
	authConfig types.AuthConfig
}

func New(endpoint string, authConfig types.AuthConfig) *EruServiceDiscovery {
	return &EruServiceDiscovery{
		endpoint:   endpoint,
		authConfig: authConfig,
	}
}

func (w *EruServiceDiscovery) Watch(ctx context.Context) (_ <-chan []string, err error) {
	cc, err := w.dial(w.endpoint, w.authConfig)
	logger := log.WithFunc("servicediscovery.Watch").WithField("endpoint", w.endpoint)
	if err != nil {
		logger.Error(ctx, err, "dial")
		return nil, err
	}
	client := pb.NewCoreRPCClient(cc)
	ch := make(chan []string)
	epPusher := &utils.EndpointPusher{}
	epPusher.Register(ch)
	epPusher.Register(lbResolverBuilder.updateCh)
	go func() {
		defer close(ch)
		for {
			watchCtx, cancelWatch := context.WithCancel(ctx)
			stream, err := client.WatchServiceStatus(watchCtx, &pb.Empty{})
			if err != nil {
				logger.Error(ctx, err, "watch service status")
				time.Sleep(10 * time.Second)
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
					logger.Error(ctx, err, "recv service status")
					break
				}
				expectedInterval = time.Duration(status.GetIntervalInSecond())

				epPusher.Push(ctx, status.GetAddresses())
			}
		}
	}()

	return ch, nil
}

func (w *EruServiceDiscovery) dial(addr string, authConfig types.AuthConfig) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStreamInterceptor(interceptor.NewStreamRetry(interceptor.RetryOptions{Max: 1})),
	}

	if authConfig.Username != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(auth.NewCredential(authConfig)))
	}

	target := makeServiceDiscoveryTarget(addr)
	return grpc.NewClient(target, opts...)
}

func makeServiceDiscoveryTarget(addr string) string {
	return fmt.Sprintf("lb://_/%s", addr)
}
