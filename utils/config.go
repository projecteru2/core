package utils

import (
	"io/ioutil"
	"log"
	"time"

	"gitlab.ricebook.net/platform/core/types"

	yaml "gopkg.in/yaml.v2"
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

	if config.Timeout.Common == 0 {
		log.Fatal("Common timeout not set, exit")
	}

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

	return SetTimeout(config), nil
}

func SetTimeout(config types.Config) types.Config {
	ct := config.Timeout

	ct.Common *= time.Second
	c := ct.Common
	if ct.Backup *= time.Second; ct.Backup == 0 {
		ct.Backup = c
	}
	if ct.BuildImage *= time.Second; ct.BuildImage == 0 {
		ct.BuildImage = c
	}
	if ct.CreateContainer *= time.Second; ct.CreateContainer == 0 {
		ct.CreateContainer = c
	}
	if ct.RemoveContainer *= time.Second; ct.RemoveContainer == 0 {
		ct.RemoveContainer = c
	}
	if ct.RemoveImage *= time.Second; ct.RemoveImage == 0 {
		ct.RemoveImage = c
	}
	if ct.RunAndWait *= time.Second; ct.RunAndWait == 0 {
		ct.RunAndWait = c
	}
	if ct.Realloc *= time.Second; ct.Realloc == 0 {
		ct.Realloc = c
	}

	config.Timeout = ct
	return config
}
