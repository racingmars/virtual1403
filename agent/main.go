package main

// Copyright 2021 Matthew R. Wilson <mwilson@mattwilson.org>
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
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/racingmars/virtual1403/scanner"
	"github.com/racingmars/virtual1403/vprinter"
)

// We will embed IBM Plex Mono as a nice default font to use if the user
// doesn't specify an alternative in the configuration file.
//go:embed IBMPlexMono-Regular.ttf
var defaultFont []byte

// version can be set at build time
var version string = "unknown"

type configuration struct {
	HerculesAddress string `yaml:"hercules_address"`
	Mode            string `yaml:"mode"`
	ServiceAddress  string `yaml:"service_address"`
	APIKey          string `yaml:"access_key"`
	OutputDir       string `yaml:"output_directory"`
	FontFile        string `yaml:"font_file"`
}

var trace = flag.Bool("trace", false, "enable trace logging")

func main() {
	displayVersion := flag.Bool("version", false,
		"display version and quit")
	flag.Parse()

	if *displayVersion {
		fmt.Printf("Version %s\n", version)
		return
	}

	var handler scanner.PrinterHandler
	startupMessage()

	if *trace {
		log.Printf("TRACE: trace logging enabled")
	}

	// Load configuration file
	conf, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("FATAL: %v", err)
	}

	errs := validateConfig(conf)
	if errs != nil {
		for _, err := range errs {
			log.Printf("ERROR: %s", err.Error())
		}
		log.Fatalf("FATAL: invalid configuration")
	}

	if conf.Mode == "local" {
		// setup for local mode

		// Make sure the output directory exists
		if err = verifyOrCreateDir(conf.OutputDir); err != nil {
			log.Fatalf("FATAL: %v", err.Error())
		}
		log.Printf("INFO:  Will create PDFs in directory `%s`",
			conf.OutputDir)

		// Verify we have a font we can use. If the user doesn't provide a
		// font, we will use our embedded copy of IBM Plex Mono. If the user
		// does provide a font, we will make sure we can read the file, use
		// it in a PDF, and that it is a fixed-width font.
		var font []byte
		if conf.FontFile == "" {
			// easy... just use default font
			log.Printf("INFO:  Using default font")
			font = defaultFont
		} else {
			log.Printf("INFO:  Attempting to load font %s", conf.FontFile)
			font, err = vprinter.LoadFont(conf.FontFile)
			if err != nil {
				log.Fatalf("FATAL: couldn't load requested font: %v", err)
			}
			log.Printf("INFO:  Successfully loaded font %s", conf.FontFile)
		}

		// Set up our output handler
		handler, err = newPDFOutputHandler(conf.OutputDir, font)
		if err != nil {
			log.Fatalf("FATAL: %v", err)
		}
	} else if conf.Mode == "online" {
		// setup for online mode
		log.Printf("INFO:  will use online print API at `%s`",
			conf.ServiceAddress)
		handler = newOnlineOutputHandler(conf.ServiceAddress, conf.APIKey)
	}

	// Hercules sometimes closes connections on the printer socket device even
	// when everything is still up and running -- seems to happen, at least,
	// if you kill the client (e.g. us) and then re-connect...it's like the
	// socket close on Hercules' side is queued up and immediately executed on
	// the next client the connects. Also, if someone stops and starts
	// Hercules, we want the agent to automatically re-connect. So, we just
	// loop forever with a 10 second pause between connection failures or
	// disconnects.
	for {
		handleHercules(conf.HerculesAddress, handler)
		log.Printf("INFO:  Re-trying Hercules connection in 10 seconds...")
		time.Sleep(10 * time.Second)
	}
}

func loadConfig(path string) (configuration, error) {
	var c configuration
	f, err := os.Open(path)
	if err != nil {
		return c, err
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&c); err != nil {
		return c, err
	}

	return c, nil
}

func validateConfig(c configuration) []error {
	var errs []error

	// Verify required fields are present
	if c.HerculesAddress == "" {
		errs = append(errs,
			errors.New("must set 'hercules_address' in the config file"))
	}

	if !(c.Mode == "local" || c.Mode == "online") {
		errs = append(errs,
			errors.New("'mode' must be either 'local' or 'online'"))
	}

	if c.Mode == "local" {
		if c.OutputDir == "" {
			errs = append(errs,
				errors.New("must set 'output_directory' in the config file"))
		}
	}

	if c.Mode == "online" {
		if c.ServiceAddress == "" {
			errs = append(errs,
				errors.New("must set 'service_address' in the config file"))
		}
		if c.APIKey == "" {
			errs = append(errs,
				errors.New("must set 'api_key' in the config file"))
		}
	}

	return errs
}

func handleHercules(address string, handler scanner.PrinterHandler) {
	log.Printf("INFO:  Connecting to Hercules on %s...\n", address)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Printf("ERROR: Couldn't connect: %v\n", err)
		return
	}
	defer conn.Close()
	log.Printf("INFO:  Connection successful.\n")

	err = scanner.Scan(conn, handler, *trace)
	if err == io.EOF {
		// we're done!
		log.Printf("INFO:  Hercules disconnected.\n")
		return
	}
	if err != nil {
		log.Printf("ERROR: error reading from Hercules: %s\n", err)
		return
	}
}

// verifyOrCreateDir will check if path exists and is a directory. If so, the
// returned error will be nil. If path doesn't exist, we will try to create
// the directory, and if successful, returned error will be nil. In other
// cases, error will be non-nil and the caller should not assume path is a
// directory that it may use.
func verifyOrCreateDir(path string) error {
	stat, err := os.Stat(path)

	if err == nil {
		if stat.IsDir() {
			// Directory exists
			return nil
		}

		// Path exists, but isn't a directory
		return fmt.Errorf("`%s` is not a directory", path)
	}

	// Try to create the directory
	log.Printf("INFO:  creating directory `%s`", path)
	if err = os.MkdirAll(path, 0755); err != nil {
		return err
	}

	return nil
}

func startupMessage() {
	fmt.Fprintln(os.Stderr, `       _      _               _   _ _  _    ___ _____`)
	fmt.Fprintln(os.Stderr, `__   _(_)_ __| |_ _   _  __ _| | / | || |  / _ \___ /`)
	fmt.Fprintln(os.Stderr, "\\ \\ / / | '__| __| | | |/ _` | | | | || |_| | | ||_ \\")
	fmt.Fprintln(os.Stderr, ` \ V /| | |  | |_| |_| | (_| | | | |__   _| |_| |__) |`)
	fmt.Fprintln(os.Stderr, `  \_/ |_|_|   \__|\__,_|\__,_|_| |_|  |_|  \___/____/`)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "virtual1403 <https://github.com/racingmars/virtual1403/>")
	fmt.Fprintln(os.Stderr, "  copyright 2021 Matthew R. Wilson.")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "virtual1403 is free software, distributed under the GPL v3")
	fmt.Fprintln(os.Stderr, "  (or later) license; see COPYING for details.")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "Version %s\n\n", version)
}
