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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/racingmars/virtual1403/webserver/db"
	"github.com/racingmars/virtual1403/webserver/mailer"
	"github.com/racingmars/virtual1403/webserver/model"
	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	DatabaseFile string        `yaml:"database_file"`
	CreateAdmin  string        `yaml:"create_admin"`
	FontFile     string        `yaml:"font_file"`
	ListenPort   int           `yaml:"listen_port"`
	MailConfig   mailer.Config `yaml:"mail_config"`
}

func readConfig(path string) (ServerConfig, []error) {
	var c ServerConfig
	var errs []error

	f, err := os.Open(path)
	if err != nil {
		return c, []error{err}
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&c); err != nil {
		return c, []error{err}
	}

	if c.DatabaseFile == "" {
		errs = append(errs, fmt.Errorf("database file is required"))
	}

	if c.ListenPort < 1 || c.ListenPort > 65535 {
		errs = append(errs, fmt.Errorf("port number %d is invalid",
			c.ListenPort))
	}

	if !mailer.ValidateAddress(c.MailConfig.FromAddress) {
		errs = append(errs,
			fmt.Errorf("address `%s` does not appear to be valid",
				c.MailConfig.FromAddress))
	}

	if c.MailConfig.Server == "" {
		errs = append(errs, fmt.Errorf("mail_config.server is required"))
	}
	if c.MailConfig.Port == 0 {
		errs = append(errs, fmt.Errorf("mail_config.port is required"))
	}
	if c.MailConfig.Port < 1 || c.MailConfig.Port > 65535 {
		errs = append(errs, fmt.Errorf("mail_config.port (%d) is invalid",
			c.MailConfig.Port))
	}
	if c.MailConfig.Username == "" {
		errs = append(errs, fmt.Errorf("mail_config.username is required"))
	}
	if c.MailConfig.Password == "" {
		errs = append(errs, fmt.Errorf("mail_config.password is required"))
	}

	return c, errs
}

func (a *application) createAdmin(email string) error {
	// Only proceed if admin user doesn't already exist
	_, err := a.db.GetUser(email)
	if err != db.ErrNotFound {
		log.Printf("INFO  admin account %s already exists", email)
		return nil
	}

	// Generate random password. 128 bits; if it's good enough for AES, it's
	// good enough for us!
	pwbytes := make([]byte, 128/8)
	if n, err := rand.Read(pwbytes); err != nil || n != len(pwbytes) {
		// shouldn't be possible to have an error reading rand
		panic(err)
	}
	pwstring := hex.EncodeToString(pwbytes)

	u := model.NewUser(email, pwstring)
	u.Admin = true
	u.Verified = true
	u.Enabled = true

	err = a.db.SaveUser(u)
	if err != nil {
		return err
	}

	log.Printf("INFO  Created new admin account: %s ; %s ; %s", email,
		pwstring, u.AccessKey)
	return nil
}
