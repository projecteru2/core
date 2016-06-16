package main

import (
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"gitlab.ricebook.net/platform/core/cluster"
	"gitlab.ricebook.net/platform/core/types"
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

	return config, nil
}

func runCluster() {
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

	cluster, err := cluster.NewCalcium(config)
	if err != nil {
		log.Fatal(err)
	}

	log.Info("Calcium started successfully.")
	cluster.Run()
	// TODO
	// add grpc server here
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
	app.Action = func(c *cli.Context) {
		runCluster()
	}

	app.Run(os.Args)
}
