package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"gitlab.ricebook.net/platform/core/cluster/calcium"
	"gitlab.ricebook.net/platform/core/rpc"
	"gitlab.ricebook.net/platform/core/rpc/gen"
	"gitlab.ricebook.net/platform/core/stats"
	"gitlab.ricebook.net/platform/core/types"
	"gitlab.ricebook.net/platform/core/versioninfo"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"
)

var (
	configPath string
	logLevel   string
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
	return nil
}

func initConfig(configPath string) (types.Config, error) {
	config := types.Config{}

	bytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		return config, err
	}

	if err := yaml.Unmarshal(bytes, &config); err != nil {
		return config, err
	}

	if config.Timeout.Common == 0 {
		log.Fatal("Common timeout not set, exit")
	}

	if config.Docker.APIVersion == "" {
		config.Docker.APIVersion = "v1.23"
	}
	if config.Docker.LogDriver == "" {
		config.Docker.LogDriver = "none"
	}
	if config.Scheduler.Type == "complex" {
		if config.Scheduler.ShareBase == 0 {
			config.Scheduler.ShareBase = 10
		}
		if config.Scheduler.MaxShare == 0 {
			config.Scheduler.MaxShare = -1
		}
	}
	return config, nil
}

func serve() {
	if err := setupLog(logLevel); err != nil {
		log.Fatal(err)
	}

	if configPath == "" {
		log.Fatalf("Config path must be set")
	}

	config, err := initConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	stats.NewStatsdClient(config.Statsd)

	cluster, err := calcium.New(config)
	if err != nil {
		log.Fatal(err)
	}

	vibranium := rpc.New(cluster, config)
	s, err := net.Listen("tcp", config.Bind)
	if err != nil {
		log.Fatal(err)
	}

	opts := []grpc.ServerOption{grpc.MaxConcurrentStreams(100)}
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterCoreRPCServer(grpcServer, vibranium)
	go grpcServer.Serve(s)
	go http.ListenAndServe(":46656", nil)

	log.Info("Cluster started successfully.")

	// wait for unix signals and try to GracefulStop
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM)
	<-sigs
	grpcServer.GracefulStop()
	log.Info("gRPC server gracefully stopped.")

	log.Info("Now check if cluster still have running tasks...")
	vibranium.Wait()
	log.Info("cluster gracefully stopped.")
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
		cli.StringFlag{
			Name:        "config",
			Value:       "/etc/eru/core.yaml",
			Usage:       "config file path for core, in yaml",
			Destination: &configPath,
			EnvVar:      "ERU_CONFIG_PATH",
		},
		cli.StringFlag{
			Name:        "log-level",
			Value:       "INFO",
			Usage:       "set log level",
			Destination: &logLevel,
			EnvVar:      "ERU_LOG_LEVEL",
		},
	}
	app.Action = func(c *cli.Context) error {
		serve()
		return nil
	}

	app.Run(os.Args)
}
