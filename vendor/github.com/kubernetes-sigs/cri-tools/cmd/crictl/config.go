/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package crictl

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

// Config is the internal representation of the yaml that determines how
// the app start
type Config struct {
	RuntimeEndpoint string `yaml:"runtime-endpoint"`
	ImageEndpoint   string `yaml:"image-endpoint"`
	Timeout         int    `yaml:"timeout"`
	Debug           bool   `yaml:"debug"`
}

// ReadConfig reads from a file with the given name and returns a config or
// an error if the file was unable to be parsed.
func ReadConfig(filepath string) (*Config, error) {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	config := Config{}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, err
}

func writeConfig(c *Config, filepath string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath, data, 0644)
}

var configCommand = cli.Command{
	Name:                   "config",
	Usage:                  "Get and set crictl options",
	ArgsUsage:              "[<options>]",
	SkipArgReorder:         true,
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "get",
			Usage: "get value: name",
		},
	},
	Action: func(context *cli.Context) error {
		configFile := context.GlobalString("config")
		if _, err := os.Stat(configFile); err != nil {
			if err := writeConfig(nil, configFile); err != nil {
				return err
			}
		}
		// Get config from file.
		config, err := ReadConfig(configFile)
		if err != nil {
			return fmt.Errorf("Failed to load config file: %v", err)
		}
		if context.IsSet("get") {
			get := context.String("get")
			switch get {
			case "runtime-endpoint":
				fmt.Println(config.RuntimeEndpoint)
			case "image-endpoint":
				fmt.Println(config.ImageEndpoint)
			case "timeout":
				fmt.Println(config.Timeout)
			case "debug":
				fmt.Println(config.Debug)
			default:
				logrus.Fatalf("No section named %s", get)
			}
			return nil
		}
		key := context.Args().First()
		if key == "" {
			return cli.ShowSubcommandHelp(context)
		}
		value := context.Args().Get(1)
		switch key {
		case "runtime-endpoint":
			config.RuntimeEndpoint = value
		case "image-endpoint":
			config.ImageEndpoint = value
		case "timeout":
			n, err := strconv.Atoi(value)
			if err != nil {
				logrus.Fatal(err)
			}
			config.Timeout = n
		case "debug":
			var debug bool
			if value == "true" {
				debug = true
			} else if value == "false" {
				debug = false
			} else {
				logrus.Fatal("use true|false for debug")
			}
			config.Debug = debug
		default:
			logrus.Fatalf("No section named %s", key)
		}

		return writeConfig(config, configFile)
	},
}
