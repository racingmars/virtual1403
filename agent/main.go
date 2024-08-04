package main

// Copyright 2021-2024 Matthew R. Wilson <mwilson@mattwilson.org>
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
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/racingmars/virtual1403/scanner"
	"github.com/racingmars/virtual1403/vprinter"
)

// version can be set at build time
var version string = "unknown"

var configFile = flag.String("config", "config.yaml", "name of config file")
var output = flag.String("output", "default", "profile to use for -printfile")
var printFile = flag.String("printfile", "",
	"print a single UTF-8 text file. Use filename \"-\" for stdin")
var useCDC = flag.Bool("cdc", false, "When using -printfile, file has CDC "+
	"carriage control characters in first position of each line")
var useASA = flag.Bool("asa", false, "When using -printfile, file has ASA "+
	"carriage control characters in first position of each line")
var trace = flag.Bool("trace", false, "enable trace logging")
var displayVersion = flag.Bool("version", false, "display version and quit")

func main() {
	flag.Parse()

	if *displayVersion {
		fmt.Printf("Version %s\n", version)
		return
	}

	startupMessage()

	if *trace {
		log.Printf("TRACE: trace logging enabled")
	}

    if *useASA && *useCDC {
        log.Fatalf("FATAL: the -asa and -cdc flags are mutually exclusive")
    }

	if *useCDC && *printFile == "" {
		log.Fatalf("FATAL: the -cdc flag is only used with the -printFile " +
			"parameter.")
	}

	if *useASA && *printFile == "" {
		log.Fatalf("FATAL: the -asa flag is only used with the -printFile " +
			"parameter.")
	}

	// Load configuration file
	inputs, outputs, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("FATAL: Unable to read config `%s`: %v", *configFile, err)
	}

	errs := validateConfig(inputs, outputs)
	if errs != nil {
		for _, err := range errs {
			log.Printf("ERROR: %s", err.Error())
		}
		log.Fatalf("FATAL: invalid configuration")
	}

	// Set up outputs
	for name, conf := range outputs {
		if conf.Mode == "local" {
			// setup for local mode

			// Make sure the output directory exists
			if err = verifyOrCreateDir(conf.OutputDir); err != nil {
				log.Fatalf("FATAL: [%s] %v", name, err.Error())
			}

			// Verify we have a font we can use. If the user doesn't provide a
			// font, we will use our embedded copy of IBM Plex Mono. If the
			// user does provide a font, we will make sure we can read the
			// file, use it in a PDF, and that it is a fixed-width font.
			//
			// Note that the user's custom font is only used for the
			// "default-" profiles; the retro- and modern- profiles will use
			// one of our embedded fonts.
			var font []byte
			if conf.FontFile == "" {
				// easy... just use default font by setting font to null
				log.Printf("INFO:  [%s] Using default font", name)
			} else {
				log.Printf("INFO:  [%s] Attempting to load font %s", name,
					conf.FontFile)
				font, err = vprinter.LoadFont(conf.FontFile)
				if err != nil {
					log.Fatalf("FATAL: [%s] couldn't load requested font: %v",
						name, err)
				}
				log.Printf("INFO:  [%s] Successfully loaded font %s", name,
					conf.FontFile)
			}
			o := outputs[name]
			o.font = font
			outputs[name] = o
		}
	}

	// If user requested that we print a single file, we will do so then quit.
	if *printFile != "" {
		// Does the requested output config exist?
		o, ok := outputs[*output]
		if !ok {
			log.Fatalf("FATAL: Output configuration [%s] doesn't exist",
				*output)
		}

		runFilePrinter(o, *printFile)

		return
	}

	// Otherwise...
	// Start a thread for each input and run until they all stop...which will
	// usually be never; typically user will Ctrl-C out of the agent. We'll
	// wait 250ms between startups so the initial log messages from each don't
	// intermingle.
	var wg sync.WaitGroup
	for input := range inputs {
		wg.Add(1)
		// the output for the input is guaranteed to exist because of the
		// earlier config validation.
		go runPrinter(input, inputs[input].Output, inputs[input],
			outputs[inputs[input].Output], &wg)
		time.Sleep(250 * time.Millisecond)
	}
	wg.Wait()
}

func runPrinter(inputName, outputName string, input InputConfig,
	output OutputConfig, wg *sync.WaitGroup) {

	defer wg.Done()
	var handler scanner.PrinterHandler
	var err error

	log.Printf("INFO:  starting input/output pair [%s]/[%s]",
		inputName, outputName)
	if output.Mode == "local" {
		log.Printf("INFO:  [%s] Will create PDFs in directory `%s`",
			inputName, output.OutputDir)
		// Set up our output handler
		handler, err = newPDFOutputHandler(output.OutputDir, output.Profile,
			output.font, inputName)
		if err != nil {
			log.Printf("ERROR: [%s] %v", inputName, err)
			return
		}
	} else {
		log.Printf("INFO:  [%s] will use online print API at `%s`",
			inputName, output.ServiceAddress)
		handler = newOnlineOutputHandler(output.ServiceAddress, output.APIKey,
			output.Profile, inputName)
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
		handleHercules(input.HerculesAddress, handler, inputName)
		log.Printf("INFO:  [%s] Re-trying Hercules connection in 10 seconds...",
			inputName)
		time.Sleep(10 * time.Second)
	}
}

func runFilePrinter(output OutputConfig, filename string) {
	var r io.ReadCloser
	var jobname string
	if *printFile == "-" {
		r = os.Stdin
		jobname = "stdin"
	} else {
		f, err := os.Open(*printFile)
		if err != nil {
			log.Fatalf("FATAL: Couldn't open file [%s]: %v",
				*printFile, err)
		}
		r = f
		jobname = filepath.Base(filename)
	}
	defer r.Close()

	// job name character set is pretty restricted, we'll change any
	// non-allowed character to _
	jobRegex := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	jobname = jobRegex.ReplaceAllString(jobname, "_")
	// and limit to 25 characters if needed
	if len(jobname) > 25 {
		jobname = jobname[:25]
	}

	var handler scanner.PrinterHandler
	var err error

	if output.Mode == "local" {
		log.Printf("INFO:  Will create PDF in directory `%s`",
			output.OutputDir)
		// Set up our output handler
		handler, err = newPDFOutputHandler(output.OutputDir, output.Profile,
			output.font, "fileReader")
		if err != nil {
			log.Printf("ERROR: %v", err)
			return
		}
	} else {
		log.Printf("INFO:  will use online print API at `%s`",
			output.ServiceAddress)
		handler = newOnlineOutputHandler(output.ServiceAddress, output.APIKey,
			output.Profile, "fileReader")
	}
    if *useCDC {
        err = scanner.ScanCDCUTF8Single(r, jobname, handler, *trace)
    } else if *useASA {
		err = scanner.ScanASAUTF8Single(r, jobname, handler, *trace)
	} else {
		err = scanner.ScanUTF8Single(r, jobname, handler, *trace)
	}
	if err != nil {
		log.Fatalf("FATAL: %v", err)
	}
}

func handleHercules(address string, handler scanner.PrinterHandler,
	inputName string) {
	log.Printf("INFO:  [%s] Connecting to Hercules on %s...", inputName,
		address)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Printf("ERROR: [%s] Couldn't connect: %v", inputName, err)
		return
	}
	defer conn.Close()
	log.Printf("INFO:  [%s] Connection successful.", inputName)

	err = scanner.ScanWithLogTag(conn, handler, *trace, inputName)
	if err == io.EOF {
		// we're done!
		log.Printf("WARN:  [%s] Hercules disconnected.", inputName)
		return
	}
	if err != nil {
		log.Printf("ERROR: [%s] error reading from Hercules: %s", inputName,
			err)
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
	fmt.Fprintln(os.Stderr, "  copyright 2021-2024 Matthew R. Wilson.")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "virtual1403 is free software, distributed under the GPL v3")
	fmt.Fprintln(os.Stderr, "  (or later) license; see COPYING for details.")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "Version %s\n\n", version)
}
