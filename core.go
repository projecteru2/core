package main

import (
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/projecteru2/core/auth"
	"github.com/projecteru2/core/cluster/calcium"
	"github.com/projecteru2/core/metrics"
	"github.com/projecteru2/core/rpc"
	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/utils"
	"github.com/projecteru2/core/versioninfo"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
	"google.golang.org/grpc"

	_ "go.uber.org/automaxprocs"
)

var (
	configPath      string
	embeddedStorage bool
)

func setupLog(l string) error {
	level, err := log.ParseLevel(l)
	if err != nil {
		return err
	}
	log.SetLevel(level)

	formatter := &log.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   true,
	}
	log.SetFormatter(formatter)
	log.SetOutput(os.Stdout)
	return nil
}

func serve() {
	config, err := utils.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("[main] %v", err)
	}

	if err := setupLog(config.LogLevel); err != nil {
		log.Fatalf("[main] %v", err)
	}

	if err := metrics.InitMetrics(config); err != nil {
		log.Fatalf("[main] %v", err)
	}

	cluster, err := calcium.New(config, embeddedStorage)
	if err != nil {
		log.Fatalf("[main] %v", err)
	}
	defer cluster.Finalizer()

	rpcch := make(chan struct{}, 1)
	vibranium := rpc.New(cluster, config, rpcch)
	s, err := net.Listen("tcp", config.Bind)
	if err != nil {
		log.Fatalf("[main] %v", err)
	}

	opts := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(uint32(config.GRPCConfig.MaxConcurrentStreams)),
		grpc.MaxRecvMsgSize(config.GRPCConfig.MaxRecvMsgSize),
	}

	if config.Auth.Username != "" {
		log.Info("[main] Cluster auth enable.")
		auth := auth.NewAuth(config.Auth)
		opts = append(opts, grpc.StreamInterceptor(auth.StreamInterceptor))
		opts = append(opts, grpc.UnaryInterceptor(auth.UnaryInterceptor))
		log.Infof("[main] Username %s Password %s", config.Auth.Username, config.Auth.Password)
	}

	grpcServer := grpc.NewServer(opts...)
	pb.RegisterCoreRPCServer(grpcServer, vibranium)
	go grpcServer.Serve(s)
	if config.Profile != "" {
		http.Handle("/metrics", metrics.Client.ResourceMiddleware(cluster)(promhttp.Handler()))
		go http.ListenAndServe(config.Profile, nil)
	}

	log.Info("[main] Cluster started successfully.")

	// wait for unix signals and try to GracefulStop
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM)
	sig := <-sigs
	log.Infof("[main] Get signal %v.", sig)
	close(rpcch)
	grpcServer.GracefulStop()
	log.Info("[main] gRPC server gracefully stopped.")

	log.Info("[main] Check if cluster still have running tasks.")
	vibranium.Wait()
	log.Info("[main] cluster gracefully stopped.")
}

func main() {
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Print(versioninfo.VersionString())
	}

	app := cli.NewApp()
	app.Name = versioninfo.NAME
	app.Usage = "Run eru core"
	app.Version = versioninfo.VERSION
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
	app.Action = func(c *cli.Context) error {
		serve()
		return nil
	}

	app.Run(os.Args)
}
