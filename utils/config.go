package utils

import (
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/projecteru2/core/types"

	log "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
)

const (
	defaultTTL    = 30
	defaultPrefix = "/v2/keys"
)

// LoadConfig load config from yaml
func LoadConfig(configPath string) (types.Config, error) {
	config := types.Config{}

	bytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		return config, err
	}

	if err := yaml.Unmarshal(bytes, &config); err != nil {
		return config, err
	}

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
		config.Docker.APIVersion = "1.32"
	}
	// 默认是 journald
	if config.Docker.Log.Type == "" {
		config.Docker.Log.Type = "journald"
	}
	if config.Scheduler.ShareBase == 0 {
		config.Scheduler.ShareBase = 100
	}
	if config.Scheduler.MaxShare == 0 {
		config.Scheduler.MaxShare = -1
	}

	return config, nil
}
