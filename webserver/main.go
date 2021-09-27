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
	"fmt"
	"log"
	"net/http"

	"github.com/racingmars/virtual1403/vprinter"
	"github.com/racingmars/virtual1403/webserver/db"
	"github.com/racingmars/virtual1403/webserver/mailer"
)

type application struct {
	font       []byte
	db         db.DB
	mailconfig mailer.Config
}

//go:embed IBMPlexMono-Regular.ttf
var defaultFont []byte

func main() {
	var app application
	var err error

	config, errs := readConfig("config.yaml")
	if len(errs) > 0 {
		for _, err := range errs {
			log.Printf("ERROR configuration: %v", err)
		}
		log.Fatal("FATAL configuration errors")
	}

	// If the user requested a font file, see if we can load it. Otherwise,
	// use our standard embedded font.
	if config.FontFile != "" {
		log.Printf("INFO  loading font %s", config.FontFile)
		app.font, err = vprinter.LoadFont(config.FontFile)
		if err != nil {
			log.Fatalf("FATAL unable to load font: %v", err)
		}
	} else {
		app.font = defaultFont
	}

	app.db, err = db.NewDB(config.DatabaseFile)
	if err != nil {
		panic(err)
	}
	defer app.db.Close()

	app.mailconfig = config.MailConfig

	if config.CreateAdmin != "" {
		if err := app.createAdmin(config.CreateAdmin); err != nil {
			log.Fatalf("FATAL unable to create admin user: %v", err)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", home)
	mux.HandleFunc("/print", app.printjob)

	log.Printf("Starting server on :%d", config.ListenPort)
	err = http.ListenAndServe(fmt.Sprintf(":%d", config.ListenPort), mux)
	log.Fatal(err)
}

func home(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Hello from virtual1403"))
}
