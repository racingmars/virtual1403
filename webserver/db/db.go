package db

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
	"errors"

	"github.com/racingmars/virtual1403/webserver/model"
	"golang.org/x/crypto/acme/autocert"
)

var ErrNotFound = errors.New("not found")

const SessionSecretKeyLength = 32

type DB interface {
	// Close will close the database.
	Close() error

	// SaveUser saves a new user, or updates an existing user, identified by
	// user.Email.
	SaveUser(user model.User) error

	// GetUser retrieves a user from the database by email.
	GetUser(email string) (model.User, error)

	// GetUserForAccessKey returns the user with the provided access key.
	GetUserForAccessKey(key string) (model.User, error)

	// GetUsers returns all users in the database.
	GetUsers() ([]model.User, error)

	// DeleteUser deletes the user with  the provided email address.
	DeleteUser(email string) error

	// LogJob will record that a job was just processed for the user with the
	// provided email address. This will add to the job log and update the
	// user's record with the last job time and increase the job count for the
	// user.
	LogJob(email, jobinfo string, pages int) error

	// GetUserJobLog returns up to size rows from the job log for the user
	// with the provided email address. Jobs are returned in descending order
	// of time.
	GetUserJobLog(email string, size int) ([]model.JobLogEntry, error)

	// GetJobLog returns up to size rows from the job log in descending order
	// of time.
	GetJobLog(size int) ([]model.JobLogEntry, error)

	// GetSessionSecret will return a 32-byte random value to use as the
	// session secret key. If none exists in the database, this function will
	// generate one and save it. Essentially, on first startup, each new
	// database generates a random value which will be used for the life of
	// the database file.
	GetSessionSecret() ([]byte, error)

	// We also use our database as an autocert cache
	autocert.Cache
}
