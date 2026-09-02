package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" //nolint:gosec
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	zerolog "github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
	"google.golang.org/grpc"

	"github.com/projecteru2/core/auth"
	"github.com/projecteru2/core/cluster/calcium"
	"github.com/projecteru2/core/engine/factory"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/metrics"
	"github.com/projecteru2/core/rpc"
	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/selfmon"
	"github.com/projecteru2/core/store/etcdv3/embedded"
	"github.com/projecteru2/core/utils"
	"github.com/projecteru2/core/version"
)

var (
	configPath      string
	embeddedStorage bool
)

func serve(ctx context.Context, _ *cli.Command) error {
	config, err := utils.LoadConfig(configPath)
	if err != nil {
		zerolog.Fatal().Err(err).Send()
	}

	if err = log.SetupLog(ctx, &config.Log, config.SentryDSN); err != nil {
		zerolog.Fatal().Err(err).Send()
	}
	defer log.SentryFlush()
	defer log.SentryDefer()
	logger := log.WithFunc("main.serve")

	var embeddedETCD *embedded.Cluster
	if embeddedStorage {
		if embeddedETCD, err = embedded.New(filepath.Join(os.TempDir(), "eru-core-etcd")); err != nil {
			logger.Error(ctx, err, "start embedded storage")
			return err
		}
		defer embeddedETCD.Close()
	}
	cluster, err := calcium.New(ctx, config, embeddedETCD)
	if err != nil {
		logger.Error(ctx, err)
		return err
	}
	defer cluster.Finalizer()

	stor := cluster.GetStore()
	factory.InitEngineCache(ctx, config, stor)

	cluster.DisasterRecover(ctx)

	stop := make(chan struct{})
	vibranium := rpc.New(cluster, config, stop)
	s, err := net.Listen("tcp", config.Bind)
	if err != nil {
		logger.Error(ctx, err)
		return err
	}

	opts := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(config.GRPCConfig.MaxConcurrentStreams),
		grpc.MaxRecvMsgSize(config.GRPCConfig.MaxRecvMsgSize),
	}

	if config.Auth.Username != "" {
		logger.Infof(ctx, "cluster auth enabled for %s", config.Auth.Username)
		auth := auth.NewAuth(config.Auth)
		opts = append(opts, grpc.StreamInterceptor(auth.StreamInterceptor), grpc.UnaryInterceptor(auth.UnaryInterceptor))
	}

	grpcServer := grpc.NewServer(opts...)
	pb.RegisterCoreRPCServer(grpcServer, vibranium)
	utils.SentryGo(func() {
		if serveErr := grpcServer.Serve(s); serveErr != nil {
			logger.Error(ctx, serveErr, "start grpc server")
		}
	})

	if config.Profile != "" {
		http.Handle("/metrics", metrics.Client.ResourceMiddleware(cluster)(promhttp.Handler()))
		utils.SentryGo(func() {
			server := &http.Server{
				Addr:              config.Profile,
				ReadHeaderTimeout: 3 * time.Second,
			}
			if serveErr := server.ListenAndServe(); serveErr != nil {
				logger.Error(ctx, serveErr, "start http server")
			}
		})
	}

	unregisterService, err := cluster.RegisterService(ctx)
	if err != nil {
		logger.Error(ctx, err, "register service")
		return err
	}
	logger.Info(ctx, "cluster started")

	signalCtx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer cancel()

	utils.SentryGo(func() {
		selfmon.RunNodeStatusWatcher(signalCtx, config, cluster, stor, cluster.GetWAL())
	})

	<-signalCtx.Done()

	logger.Info(ctx, "interrupt by signal")
	close(stop)
	unregisterService()
	grpcServer.GracefulStop()
	logger.Info(ctx, "grpc server gracefully stopped")

	logger.Info(ctx, "waiting for running tasks")
	vibranium.Wait()
	logger.Info(ctx, "cluster gracefully stopped")
	return nil
}

func main() {
	cli.VersionPrinter = func(_ *cli.Command) {
		fmt.Print(version.String())
	}

	app := &cli.Command{
		Name:    version.NAME,
		Usage:   "Run eru core",
		Version: version.VERSION,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config",
				Value:       "/etc/eru/core.yaml",
				Usage:       "config file path for core, in yaml",
				Destination: &configPath,
				Sources:     cli.EnvVars("ERU_CONFIG_PATH"),
			},
			&cli.BoolFlag{
				Name:        "embedded-storage",
				Usage:       "active embedded storage",
				Destination: &embeddedStorage,
			},
		},
		Action: serve,
	}
	if err := app.Run(context.Background(), os.Args); err != nil {
		os.Exit(1)
	}
}
