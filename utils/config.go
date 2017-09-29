package utils

import (
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/projecteru2/core/types"

	yaml "gopkg.in/yaml.v2"
)

const (
	defaultTTL    = 30
	defaultPrefix = "/v2/keys"
)

func LoadConfig(configPath string) (types.Config, error) {
	config := types.Config{}

	bytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		return config, err
	}

	if err := yaml.Unmarshal(bytes, &config); err != nil {
		return config, err
	}

	config.AppDir = strings.TrimRight(config.AppDir, "/")
	if config.LockTimeout == 0 {
		config.LockTimeout = defaultTTL
	}

	if config.GlobalTimeout == 0 {
		log.Fatal("[Config] Global timeout invaild, exit")
	}
	config.GlobalTimeout = config.GlobalTimeout * time.Second
	// Fxxk etcd client
	config.Etcd.Prefix = filepath.Join(defaultPrefix, config.Etcd.Prefix)

	if config.Docker.APIVersion == "" {
		config.Docker.APIVersion = "v1.23"
	}
	if config.Docker.LogDriver == "" {
		config.Docker.LogDriver = "none"
	}
	if config.Scheduler.ShareBase == 0 {
		config.Scheduler.ShareBase = 10
	}
	if config.Scheduler.MaxShare == 0 {
		config.Scheduler.MaxShare = -1
	}

	return config, nil
}
