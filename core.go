package main

import (
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" //nolint
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	zerolog "github.com/rs/zerolog/log"

	"github.com/projecteru2/core/auth"
	"github.com/projecteru2/core/cluster/calcium"
	"github.com/projecteru2/core/engine/factory"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/metrics"
	"github.com/projecteru2/core/rpc"
	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/selfmon"
	"github.com/projecteru2/core/utils"
	"github.com/projecteru2/core/version"

	cli "github.com/urfave/cli/v2"
	_ "go.uber.org/automaxprocs"
	"google.golang.org/grpc"
)

var (
	configPath      string
	embeddedStorage bool
)

func serve(c *cli.Context) error {
	config, err := utils.LoadConfig(configPath)
	if err != nil {
		zerolog.Fatal().Err(err).Send()
	}

	if err := log.SetupLog(c.Context, &config.Log, config.SentryDSN); err != nil {
		zerolog.Fatal().Err(err).Send()
	}
	defer log.SentryDefer()
	logger := log.WithFunc("main")

	// init engine cache and start engine cache checker
	factory.InitEngineCache(c.Context, config)

	var t *testing.T
	if embeddedStorage {
		t = &testing.T{}
	}
	cluster, err := calcium.New(c.Context, config, t)
	if err != nil {
		logger.Error(c.Context, err)
		return err
	}
	defer cluster.Finalizer()
	cluster.DisasterRecover(c.Context)

	stop := make(chan struct{}, 1)
	vibranium := rpc.New(cluster, config, stop)
	s, err := net.Listen("tcp", config.Bind)
	if err != nil {
		logger.Error(c.Context, err)
		return err
	}

	opts := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(uint32(config.GRPCConfig.MaxConcurrentStreams)),
		grpc.MaxRecvMsgSize(config.GRPCConfig.MaxRecvMsgSize),
	}

	if config.Auth.Username != "" {
		logger.Info(c.Context, "cluster auth enable.")
		auth := auth.NewAuth(config.Auth)
		opts = append(opts, grpc.StreamInterceptor(auth.StreamInterceptor))
		opts = append(opts, grpc.UnaryInterceptor(auth.UnaryInterceptor))
		logger.Infof(c.Context, "username %s password %s", config.Auth.Username, config.Auth.Password)
	}

	grpcServer := grpc.NewServer(opts...)
	pb.RegisterCoreRPCServer(grpcServer, vibranium)
	utils.SentryGo(func() {
		if err := grpcServer.Serve(s); err != nil {
			logger.Error(c.Context, err, "start grpc failed")
		}
	})

	if config.Profile != "" {
		http.Handle("/metrics", metrics.Client.ResourceMiddleware(cluster)(promhttp.Handler()))
		utils.SentryGo(func() {
			server := &http.Server{
				Addr:              config.Profile,
				ReadHeaderTimeout: 3 * time.Second,
			}
			if err := server.ListenAndServe(); err != nil {
				logger.Error(c.Context, err, "start http failed")
			}
		})
	}

	unregisterService, err := cluster.RegisterService(c.Context)
	if err != nil {
		logger.Error(c.Context, err, "failed to register service")
		return err
	}
	logger.Info(c.Context, "cluster started successfully.")

	// wait for unix signals and try to GracefulStop
	ctx, cancel := signal.NotifyContext(c.Context, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer cancel()

	// start node status checker
	utils.SentryGo(func() {
		selfmon.RunNodeStatusWatcher(ctx, config, cluster, t)
	})

	<-ctx.Done()

	logger.Info(c.Context, "interrupt by signal")
	close(stop)
	unregisterService()
	grpcServer.GracefulStop()
	logger.Info(c.Context, "gRPC server gracefully stopped.")

	logger.Info(c.Context, "check if cluster still have running tasks.")
	vibranium.Wait()
	logger.Info(c.Context, "cluster gracefully stopped.")
	return nil

}

func main() {
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Print(version.String())
	}

	app := cli.NewApp()
	app.Name = version.NAME
	app.Usage = "Run eru core"
	app.Version = version.VERSION
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:        "config",
			Value:       "/etc/eru/core.yaml",
			Usage:       "config file path for core, in yaml",
			Destination: &configPath,
			EnvVars:     []string{"ERU_CONFIG_PATH"},
		},
		&cli.BoolFlag{
			Name:        "embedded-storage",
			Usage:       "active embedded storage",
			Destination: &embeddedStorage,
		},
	}
	app.Action = serve
	_ = app.Run(os.Args)
}
