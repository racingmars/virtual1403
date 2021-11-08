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
	"context"

	"github.com/boltdb/bolt"
	"golang.org/x/crypto/acme/autocert"
)

// In addition to our application's DB interface, our BoltDB implementation
// will implement the interface from golang.org/x/crypto/acme/autocert#Cache
// so that we can store SSL certificate info for autocert.

// Get returns a certificate data for the specified key. If there's no such
// key, Get returns ErrCacheMiss.
func (db *boltimpl) Get(_ context.Context, key string) ([]byte, error) {
	var result []byte
	if err := db.bdb.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(autocertBucketName))
		v := bucket.Get([]byte(key))
		if v == nil {
			return autocert.ErrCacheMiss
		}
		result = make([]byte, len(v))
		copy(result, v)
		return nil
	}); err != nil {
		return nil, err
	}
	return result, nil
}

// Put stores the data in the cache under the specified key. Underlying
// implementations may use any data storage format, as long as the reverse
// operation, Get, results in the original data.
func (db *boltimpl) Put(_ context.Context, key string, data []byte) error {
	if err := db.bdb.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(autocertBucketName))
		if err := bucket.Put([]byte(key), data); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// Delete removes a certificate data from the cache under the specified key.
// If there's no such key in the cache, Delete returns nil.
func (db *boltimpl) Delete(_ context.Context, key string) error {
	if err := db.bdb.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(autocertBucketName))
		if err := bucket.Delete([]byte(key)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}
