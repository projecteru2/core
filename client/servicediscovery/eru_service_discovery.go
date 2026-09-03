package servicediscovery

import (
	"context"
	"math"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"

	"github.com/projecteru2/core/auth"
	"github.com/projecteru2/core/client/interceptor"
	"github.com/projecteru2/core/log"
	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
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

// Watch streams the core addresses the cluster publishes; the connection it watches over follows them as well.
func (w *EruServiceDiscovery) Watch(ctx context.Context) (<-chan []string, error) {
	logger := log.WithFunc("servicediscovery.EruServiceDiscovery.Watch").WithField("endpoint", w.endpoint)
	cores := manual.NewBuilderWithScheme("lb")
	cores.InitialState(addressState(w.endpoint))
	cc, err := w.dial(cores)
	if err != nil {
		logger.Error(ctx, err, "dial")
		return nil, err
	}
	client := pb.NewCoreRPCClient(cc)
	ch := make(chan []string)
	go func() {
		defer close(ch)
		defer func() { _ = cc.Close() }()
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
				addresses := status.GetAddresses()
				if len(addresses) == 0 {
					continue
				}
				cores.UpdateState(addressState(addresses...))
				select {
				case ch <- addresses:
				case <-ctx.Done():
				}
			}
			cancelWatch()
		}
	}()

	return ch, nil
}

func (w *EruServiceDiscovery) dial(cores resolver.Builder) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStreamInterceptor(interceptor.NewStreamRetry(interceptor.RetryOptions{Max: 1})),
		grpc.WithResolvers(cores),
	}

	if w.authConfig.Username != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(auth.NewCredential(w.authConfig)))
	}

	return grpc.NewClient("lb:///"+w.endpoint, opts...)
}

func addressState(endpoints ...string) resolver.State {
	return resolver.State{Addresses: utils.Map(endpoints, func(ep string) resolver.Address { return resolver.Address{Addr: ep} })}
}
