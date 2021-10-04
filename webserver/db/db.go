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
)

var ErrNotFound = errors.New("not found")

type DB interface {
	Close() error

	SaveUser(user model.User) error
	GetUser(email string) (model.User, error)
	GetUserForAccessKey(key string) (model.User, error)
	GetUsers() ([]model.User, error)
	DeleteUser(email string) error
}
