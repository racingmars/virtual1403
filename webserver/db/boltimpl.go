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
	"strings"

	"github.com/boltdb/bolt"

	"github.com/racingmars/virtual1403/webserver/model"
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

// SaveUser will save a new user or update an existing user in the database.
// We also want an index on the user's access key, so we *always* keep the
// user bucket and access key bucket in sync here. We delete and recreate the
// access key each time, even if it hasn't changed. This is simpler logic and
// user record updates aren't frequent enough that we need to optimize this.
func (db *boltimpl) SaveUser(user model.User) error {
	// Prepare the user for saving in DB by converting to JSON.
	userjson, err := json.Marshal(&user)
	if err != nil {
		return err
	}

	return db.bdb.Update(func(tx *bolt.Tx) error {
		userBucket := tx.Bucket([]byte(userBucketName))
		accessBucket := tx.Bucket([]byte(accessKeyBucketName))

		// Does the user already exist?
		olduserjson := userBucket.Get([]byte(strings.ToLower(user.Email)))
		if olduserjson != nil {
			// yes... let's grab the old access key so we can delete it
			var olduser model.User
			if err := json.Unmarshal(olduserjson, &olduser); err != nil {
				return err
			}
			accessBucket.Delete([]byte(olduser.AccessKey))
		}

		// Save the new user record and access key linked to the user
		if err := userBucket.Put([]byte(strings.ToLower(user.Email)),
			userjson); err != nil {
			return err
		}
		if err := accessBucket.Put([]byte(user.AccessKey),
			[]byte(strings.ToLower(user.Email))); err != nil {
			return err
		}

		return nil
	})
}

func (db *boltimpl) GetUser(email string) (model.User, error) {
	var user model.User
	err := db.bdb.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(userBucketName))
		v := b.Get([]byte(strings.ToLower(email)))
		if v == nil {
			return ErrNotFound
		}
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

func (db *boltimpl) GetUsers() ([]model.User, error) {
	var users []model.User
	err := db.bdb.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(userBucketName))
		if err := b.ForEach(func(k, v []byte) error {
			var user model.User
			if err := json.Unmarshal(v, &user); err != nil {
				return err
			}
			users = append(users, user)
			return nil
		}); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return users, nil
}

func (db *boltimpl) GetUserForAccessKey(key string) (model.User, error) {
	var email string
	if err := db.bdb.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(accessKeyBucketName))
		emailbytes := b.Get([]byte(key))
		if emailbytes == nil {
			return ErrNotFound
		}
		email = string(emailbytes)
		return nil
	}); err != nil {
		return model.User{}, err
	}

	return db.GetUser(email)
}

// DeleteUser needs to keep user bucket and access key bucket in sync, so
// will delete from both.
func (db *boltimpl) DeleteUser(email string) error {
	return db.bdb.Update(func(tx *bolt.Tx) error {
		userBucket := tx.Bucket([]byte(userBucketName))
		accessBucket := tx.Bucket([]byte(accessKeyBucketName))

		olduserjson := userBucket.Get([]byte(strings.ToLower(email)))
		if olduserjson == nil {
			// no such user
			return nil
		}

		var olduser model.User
		if err := json.Unmarshal(olduserjson, &olduser); err != nil {
			return err
		}

		// Now we just delete access key and user
		accessBucket.Delete([]byte(olduser.AccessKey))
		userBucket.Delete([]byte(strings.ToLower(email)))

		return nil
	})
}
