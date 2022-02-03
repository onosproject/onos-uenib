// SPDX-FileCopyrightText: 2021-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"github.com/onosproject/onos-lib-go/pkg/atomix"
	configlib "github.com/onosproject/onos-lib-go/pkg/config"
)

var config *Config

// Config is the onos-uenib configuration
type Config struct {
	// Atomix is the Atomix configuration
	Atomix atomix.Config `yaml:"atomix,omitempty"`
}

// GetConfig gets the onos-uenib configuration
func GetConfig() (Config, error) {
	if config == nil {
		config = &Config{}
		if err := configlib.Load(config); err != nil {
			return Config{}, err
		}
	}
	return *config, nil
}

// GetConfigOrDie gets the onos-uenib configuration or panics
func GetConfigOrDie() Config {
	config, err := GetConfig()
	if err != nil {
		panic(err)
	}
	return config
}
