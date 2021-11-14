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
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golangcollege/sessions"
	"github.com/gorilla/handlers"
	"golang.org/x/crypto/acme/autocert"

	"github.com/racingmars/virtual1403/vprinter"
	"github.com/racingmars/virtual1403/webserver/assets"
	"github.com/racingmars/virtual1403/webserver/db"
	"github.com/racingmars/virtual1403/webserver/mailer"
)

type application struct {
	font           []byte
	db             db.DB
	mailconfig     mailer.Config
	serverBaseURL  string
	session        *sessions.Session
	templateCache  map[string]*template.Template
	quotaJobs      int
	quotaPages     int
	quotaPeriod    time.Duration
	maxLinesPerJob int
	printerSeats   chan bool
}

//go:embed IBMPlexMono-Regular.ttf
var defaultFont []byte

//go:embed favicon.ico
var favicon []byte

func main() {
	var app application
	var err error

	config, errs := readConfig("config.yaml")
	if len(errs) > 0 {
		for _, err := range errs {
			log.Printf("ERROR: configuration: %v", err)
		}
		log.Fatal("FATAL: configuration errors")
	}

	// If the user requested a font file, see if we can load it. Otherwise,
	// use our standard embedded font.
	if config.FontFile != "" {
		log.Printf("INFO:  loading font %s", config.FontFile)
		app.font, err = vprinter.LoadFont(config.FontFile)
		if err != nil {
			log.Fatalf("FATAL: unable to load font: %v", err)
		}
	} else {
		app.font = defaultFont
	}

	// Copy the configured quota values to the application state
	app.maxLinesPerJob = config.MaxLinesPerJob
	if app.maxLinesPerJob <= 0 {
		log.Printf("WARN:  no max_lines_per_job, individual job size " +
			"will be unbounded")
	} else {
		log.Printf("INFO:  max_lines_per_job is %d", app.maxLinesPerJob)
	}

	app.quotaJobs = config.QuotaJobs
	app.quotaPages = config.QuotaPages
	if app.quotaJobs <= 0 && app.quotaPages <= 0 {
		log.Printf("WARN:  no quotas are set; all users will " +
			"be permitted unlimited use")
	}
	if app.quotaJobs > 0 {
		log.Printf("INFO:  user jobs quota is %d", app.quotaJobs)
	}
	if app.quotaPages > 0 {
		log.Printf("INFO:  user pages quota is %d", app.quotaPages)
	}

	if config.QuotaPeriod <= 0 {
		log.Printf("WARN:  no valid quota_period; setting to 24 hours")
		config.QuotaPeriod = 24
	}

	app.quotaPeriod = time.Duration(config.QuotaPeriod) * time.Hour
	log.Printf("INFO:  quota period is %s", app.quotaPeriod.String())

	// If there is a limit on concurrent print job, set up the available user
	// seats
	if config.ConcurrentPrintJobs > 0 {
		log.Printf("INFO:  limiting concurrent print jobs to %d",
			config.ConcurrentPrintJobs)
		app.printerSeats = make(chan bool, config.ConcurrentPrintJobs)
	}

	// Initialize HTML template cache for UI
	templateCache, err := newTemplateCache()
	if err != nil {
		log.Fatalf("FATAL: unable to load templates: %v", err)
	}
	app.templateCache = templateCache

	// Open BoltDB database file
	app.db, err = db.NewDB(config.DatabaseFile)
	if err != nil {
		panic(err)
	}
	defer app.db.Close()

	app.mailconfig = config.MailConfig

	if config.CreateAdmin != "" {
		if err := app.createAdmin(config.CreateAdmin); err != nil {
			log.Fatalf("FATAL: unable to create admin user: %v", err)
		}
	}

	app.serverBaseURL = config.BaseURL

	// Get session cookie secret key from DB and initialize session manager
	sessionSecret, err := app.db.GetSessionSecret()
	if err != nil {
		log.Fatalf("FATAL: unable to get session secret key: %v", err)
	}
	log.Printf("INFO:  got session secret: %s",
		hex.EncodeToString(sessionSecret))
	app.session = sessions.New(sessionSecret)
	app.session.Lifetime = 3 * time.Hour

	// Build UI routes
	mux := http.NewServeMux()
	mux.Handle("/favicon.ico", http.HandlerFunc(serveFavicon))
	mux.Handle("/static/", http.FileServer(http.FS(assets.Content)))
	mux.Handle("/docs/setup", http.HandlerFunc(app.docsSetup))
	mux.Handle("/", app.session.Enable(http.HandlerFunc(app.home)))
	mux.Handle("/login", app.session.Enable(http.HandlerFunc(app.login)))
	mux.Handle("/signup", app.session.Enable(http.HandlerFunc(app.signup)))
	mux.Handle("/changepassword", app.session.Enable(http.HandlerFunc(
		app.changePassword)))
	mux.Handle("/logout", app.session.Enable(http.HandlerFunc(app.logout)))
	mux.Handle("/user", app.session.Enable(http.HandlerFunc(app.userInfo)))
	mux.Handle("/regenkey", app.session.Enable(http.HandlerFunc(app.regenkey)))
	mux.Handle("/resend", app.session.Enable(http.HandlerFunc(
		app.resendVerification)))
	mux.Handle("/verify", app.session.Enable(http.HandlerFunc(app.verifyUser)))

	// Admin pages
	mux.Handle("/admin/users", app.session.Enable(http.HandlerFunc(
		app.adminListUsers)))
	mux.Handle("/admin/jobs", app.session.Enable(http.HandlerFunc(
		app.adminListJobs)))
	mux.Handle("/admin/edituser", app.session.Enable(http.HandlerFunc(
		app.adminEditUser)))
	mux.Handle("/admin/doedituser", app.session.Enable(http.HandlerFunc(
		app.adminEditUserPost)))
	mux.Handle("/admin/deleteuser", app.session.Enable(http.HandlerFunc(
		app.adminDeleteUser)))

	// The print API -- not part of the UI
	mux.Handle("/print", http.HandlerFunc(app.printjob))

	// If running plain HTTP service, we're ready to go
	if config.TLSListenPort <= 0 {
		log.Printf("INFO:  Starting plain HTTP server on :%d",
			config.ListenPort)
		err = http.ListenAndServe(fmt.Sprintf(":%d", config.ListenPort),
			handlers.CombinedLoggingHandler(os.Stdout, mux))
		log.Fatal(err)
		return
	}

	// Otherwise we set up a redirect on plain HTTP port and host w/ TLS and
	// autocert.
	go func() {
		log.Printf("INFO:  Starting plain HTTP redirect server on: %d",
			config.ListenPort)
		err := http.ListenAndServe(fmt.Sprintf(":%d", config.ListenPort),
			handlers.CombinedLoggingHandler(os.Stdout, http.HandlerFunc(
				generateRedirectHandler(config.TLSListenPort))))
		if err != nil {
			log.Fatal(err)
		}
	}()

	m := &autocert.Manager{
		Cache:      app.db,
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(config.TLSDomain),
	}
	s := &http.Server{
		Addr:      ":" + strconv.Itoa(config.TLSListenPort),
		TLSConfig: m.TLSConfig(),
		Handler:   handlers.CombinedLoggingHandler(os.Stdout, mux),
	}
	log.Printf("INFO:  Starting TLS HTTP server on %s", s.Addr)
	if err := s.ListenAndServeTLS("", ""); err != nil {
		log.Fatal(err)
	}
}

func generateRedirectHandler(port int) func(http.ResponseWriter,
	*http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		host := strings.Split(r.Host, ":")
		http.Redirect(w, r, fmt.Sprintf("https://%s:%d/", host[0], port),
			http.StatusMovedPermanently)
	}
}

func serveFavicon(w http.ResponseWriter, r *http.Request) {
	w.Write(favicon)
}
