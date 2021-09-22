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
	"encoding/json"

	"github.com/boltdb/bolt"
)

type boltimpl struct {
	bdb *bolt.DB
}

const (
	userBucketName      = "users"
	accessKeyBucketName = "access_keys"
)

func NewDB(path string) (DB, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}

	// Ensure existence of all buckets
	if err := db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(
			[]byte(userBucketName)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(
			[]byte(accessKeyBucketName)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &boltimpl{bdb: db}, nil
}

func (db *boltimpl) Close() error {
	return db.bdb.Close()
}

func (db *boltimpl) SaveUser(user User) error {
	return db.bdb.Update(func(tx *bolt.Tx) error {
		userBucket := tx.Bucket([]byte(userBucketName))
		buf, err := json.Marshal(user)
		if err != nil {
			return err
		}
		return userBucket.Put([]byte(user.Email), buf)
	})
}

func (db *boltimpl) GetUser(email string) (User, error) {
	var user User
	err := db.bdb.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(userBucketName))
		v := b.Get([]byte(email))
		if err := json.Unmarshal(v, &user); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return user, err
	}

	return user, nil
}
