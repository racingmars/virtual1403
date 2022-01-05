package main

// Copyright 2022 Matthew R. Wilson <mwilson@mattwilson.org>
//
// This file is part of virtual1403
// <https://github.com/racingmars/virtual1403>.
//
// virtual1403 is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// virtual1403 is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with virtual1403. If not, see <https://www.gnu.org/licenses/>.

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type OutputConfig struct {
	Mode           string `yaml:"mode"`
	ServiceAddress string `yaml:"service_address"`
	APIKey         string `yaml:"access_key"`
	OutputDir      string `yaml:"output_directory"`
	FontFile       string `yaml:"font_file"`
	Profile        string `yaml:"profile"`
	font           []byte
}

type InputConfig struct {
	HerculesAddress string `yaml:"hercules_address"`
	Output          string `yaml:"output"`
}

type Configuration struct {
	InputConfig  `yaml:",inline"`
	OutputConfig `yaml:",inline"`
	Inputs       []struct {
		Name        string `yaml:"name"`
		InputConfig `yaml:",inline"`
	} `yaml:"inputs"`
	Outputs []struct {
		Name         string `yaml:"name"`
		OutputConfig `yaml:",inline"`
	} `yaml:"outputs"`
}

func loadConfig(path string) (map[string]InputConfig, map[string]OutputConfig,
	error) {

	var c Configuration
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&c); err != nil {
		return nil, nil, err
	}

	inputs := make(map[string]InputConfig)
	c.InputConfig.Output = "default"
	inputs["default"] = c.InputConfig
	for _, i := range c.Inputs {
		if strings.TrimSpace(i.Name) == "" {
			return nil, nil, errors.New(
				"all inputs require a value in the \"name\" field")
		}
		inputs[i.Name] = i.InputConfig
	}

	outputs := make(map[string]OutputConfig)
	outputs["default"] = c.OutputConfig
	for _, o := range c.Outputs {
		if strings.TrimSpace(o.Name) == "" {
			return nil, nil, errors.New(
				"all outputs require a value in the \"name\" field")
		}
		outputs[o.Name] = o.OutputConfig
	}

	return inputs, outputs, nil
}

func validateConfig(inputs map[string]InputConfig,
	outputs map[string]OutputConfig) []error {

	var errs []error

	for name, config := range inputs {
		if config.HerculesAddress == "" {
			errs = append(errs,
				fmt.Errorf(
					"input [%s] must set 'hercules_address'",
					name))
		}

		if config.Output == "" {
			errs = append(errs,
				fmt.Errorf(
					"input [%s] must set 'output'",
					name))
		}

		if _, ok := outputs[config.Output]; !ok {
			errs = append(errs,
				fmt.Errorf(
					"input [%s] refers to output `%s`, which doesn't exist",
					name, config.Output))
		}

		// Don't allow multiple inputs to connect to the same Hercules socket
		// device.
		for othername, otherconfig := range inputs {
			if othername != name &&
				otherconfig.HerculesAddress == config.HerculesAddress {
				errs = append(errs,
					fmt.Errorf("input [%s] and input [%s] have the same "+
						"'hercules_address'; this is not allowed",
						name, othername))
			}
		}
	}

	for name, config := range outputs {
		if !(config.Mode == "local" || config.Mode == "online") {
			errs = append(errs,
				fmt.Errorf(
					"output [%s] 'mode' must be either 'local' or 'online'",
					name))
		}

		if config.Mode == "local" {
			if config.OutputDir == "" {
				errs = append(errs,
					fmt.Errorf("output [%s] must set 'output_directory'",
						name))
			}
		}

		if config.Mode == "online" {
			if config.ServiceAddress == "" {
				errs = append(errs,
					fmt.Errorf("output [%s] must set 'service_address'", name))
			}
			if config.APIKey == "" {
				errs = append(errs,
					fmt.Errorf("output [%s] must set 'api_key'", name))
			}
		}
	}

	return errs
}
