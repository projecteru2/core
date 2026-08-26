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

const retryInterval = 10 * time.Second

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
	cc, err := w.dial()
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
		for ctx.Err() == nil {
			watchCtx, cancelWatch := context.WithCancel(ctx)
			stream, err := client.WatchServiceStatus(watchCtx, &pb.Empty{})
			if err != nil {
				logger.Error(ctx, err, "watch service status")
				cancelWatch()
				select {
				case <-ctx.Done():
					return
				case <-time.After(retryInterval):
				}
				continue
			}
			expectedInterval := time.Duration(math.MaxInt64)
			watchdog := time.AfterFunc(expectedInterval, cancelWatch)

			for {
				watchdog.Reset(expectedInterval)
				status, err := stream.Recv()
				watchdog.Stop()
				if err != nil {
					logger.Error(ctx, err, "recv service status")
					break
				}
				expectedInterval = time.Duration(status.GetIntervalInSecond()) * time.Second

				epPusher.Push(ctx, status.GetAddresses())
			}
			cancelWatch()
		}
	}()

	return ch, nil
}

func (w *EruServiceDiscovery) dial() (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStreamInterceptor(interceptor.NewStreamRetry(interceptor.RetryOptions{Max: 1})),
	}

	if w.authConfig.Username != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(auth.NewCredential(w.authConfig)))
	}

	return grpc.NewClient(fmt.Sprintf("lb://_/%s", w.endpoint), opts...)
}
