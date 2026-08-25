package utils

import (
	"github.com/jinzhu/configor"

	"github.com/projecteru2/core/types"
)

// LoadConfig loads the config from the YAML file at configPath.
func LoadConfig(configPath string) (types.Config, error) {
	config := types.Config{}

	return config, configor.Load(&config, configPath)
}
