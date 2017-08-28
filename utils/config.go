package utils

import (
	"io/ioutil"

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
