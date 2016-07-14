package main

import (
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
	"gitlab.ricebook.net/platform/core/types"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"
)

var (
	configPath string
	logLevel   string
)

func waitSignal() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGHUP, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGQUIT)
	<-c
	log.Info("Terminating...")
}

func setupLogLevel(l string) error {
	level, err := log.ParseLevel(l)
	if err != nil {
		return err
	}
	log.SetLevel(level)
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

	if config.Docker.APIVersion == "" {
		config.Docker.APIVersion = "v1.23"
	}
	if config.Docker.LogDriver == "" {
		config.Docker.LogDriver = "none"
	}

	return config, nil
}

func serve() {
	if err := setupLogLevel(logLevel); err != nil {
		log.Fatal(err)
	}

	if configPath == "" {
		log.Fatalf("Config path must be set")
	}

	config, err := initConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	cluster, err := calcium.New(config)
	if err != nil {
		log.Fatal(err)
	}

	virbranium := rpc.NewVirbranium(cluster, config)
	s, err := net.Listen("tcp", config.Bind)
	if err != nil {
		log.Fatal(err)
	}

	opts := []grpc.ServerOption{}
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterCoreRPCServer(grpcServer, virbranium)
	go grpcServer.Serve(s)
	go http.ListenAndServe(":46656", nil)

	log.Info("Cluster started successfully.")
	waitSignal()
}

func main() {
	app := cli.NewApp()
	app.Name = "Eru-Core"
	app.Usage = "Run eru core"
	app.Version = "2.0.0"
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
