package model

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
	"encoding/base64"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Email                 string
	PasswordHash          string
	AccessKey             string
	FullName              string
	Admin                 bool
	Verified              bool
	Enabled               bool
	Unlimited             bool
	JobCount              int
	PageCount             int
	LastJob               time.Time
	LastVerificationEmail time.Time
	SignupDate            time.Time
}

// NewUser is a convenience function to create a new user with the
// provided email and password.
func NewUser(email, password string) User {
	var u User
	u.Email = email
	u.Enabled = true
	u.SetPassword(password)
	u.SignupDate = time.Now().UTC()
	u.GenerateAccessKey()
	return u
}

func (u *User) SetPassword(password string) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password),
		bcrypt.DefaultCost)
	if err != nil {
		panic("SetPassword should never make a bcrypt call that fails")
	}
	u.PasswordHash = base64.StdEncoding.EncodeToString(hash)
}

// CheckPassword will return true if the provided password is valid for the
// user.
func (u *User) CheckPassword(password string) bool {
	hash, err := base64.StdEncoding.DecodeString(u.PasswordHash)
	if err != nil {
		return false
	}
	if err = bcrypt.CompareHashAndPassword(hash,
		[]byte(password)); err != nil {
		return false
	}
	return true
}

// GenerateAccessKey generates and sets a new random access key on the user.
func (u *User) GenerateAccessKey() {
	const numBytes = 256 / 8 // 256 bits
	buf := make([]byte, numBytes)
	n, err := rand.Read(buf)
	if err != nil || n != numBytes {
		panic("Reading info byte buffer from rand should never fail")
	}
	u.AccessKey = base64.StdEncoding.EncodeToString(buf)
}
