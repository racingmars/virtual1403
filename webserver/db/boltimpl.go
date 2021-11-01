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
	"bytes"
	"encoding/binary"
	"encoding/json"
	"strings"
	"time"

	"github.com/boltdb/bolt"

	"github.com/racingmars/virtual1403/webserver/model"
)

type boltimpl struct {
	bdb *bolt.DB
}

const (
	userBucketName      = "users"
	accessKeyBucketName = "access_keys"
	jobLogBucketName    = "job_log"
	jobLogUserIndexName = "job_log_user_index"
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
		if _, err := tx.CreateBucketIfNotExists(
			[]byte(jobLogBucketName)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(
			[]byte(jobLogUserIndexName)); err != nil {
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

		// TODO: should we delete job log entries for this user too?

		return nil
	})
}

func (db *boltimpl) LogJob(email string, pages int) error {
	err := db.bdb.Update(func(tx *bolt.Tx) error {
		userBucket := tx.Bucket([]byte(userBucketName))
		logBucket := tx.Bucket([]byte(jobLogBucketName))
		logIdxBucket := tx.Bucket([]byte(jobLogUserIndexName))

		userjson := userBucket.Get([]byte(strings.ToLower(email)))
		if userjson == nil {
			// no such user
			return ErrNotFound
		}

		var user model.User
		if err := json.Unmarshal(userjson, &user); err != nil {
			return err
		}

		user.JobCount++
		user.PageCount += pages
		user.LastJob = time.Now().UTC()

		// Save user back to DB
		userjson, err := json.Marshal(&user)
		if err != nil {
			return err
		}
		if err := userBucket.Put([]byte(strings.ToLower(email)),
			userjson); err != nil {
			return err
		}

		// Log the job
		nextID, err := logBucket.NextSequence()
		if err != nil {
			return err
		}
		logentry := model.JobLogEntry{
			ID:    nextID,
			Email: user.Email,
			Pages: pages,
			Time:  user.LastJob,
		}
		logentryjson, err := json.Marshal(&logentry)
		if err != nil {
			return err
		}
		logID := make([]byte, 64/8) // 64-bit uint
		binary.PutUvarint(logID, nextID)
		logBucket.Put(logID, logentryjson)

		// Also maintain an index into the job log by user. The key is the
		// lowercase username (email), followed by a null byte (0), followed
		// by the 64-bit job log ID. There is no value, since the key itself
		// serves as a pointer to the job log entry.
		var indexEntry []byte
		indexEntry = append(indexEntry, []byte(strings.ToLower(user.Email))...)
		indexEntry = append(indexEntry, 0)
		indexEntry = append(indexEntry, logID...)
		err = logIdxBucket.Put(indexEntry, nil)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func (db *boltimpl) GetUserJobLog(email string, size int) (
	[]model.JobLogEntry, error) {

	results := make([]model.JobLogEntry, 0, size)

	err := db.bdb.View(func(tx *bolt.Tx) error {
		logBucket := tx.Bucket([]byte(jobLogBucketName))
		logIdxBucket := tx.Bucket([]byte(jobLogUserIndexName))

		// The log index key is the lowercase username followed by null byte
		// (0) followed by the 64-bit log job id.

		// We want to get job log entries in reverse (newest first) order, so
		// we will seek just past where the last entry for this user should be
		// (instead of email + null byte, it will be email + byte value 1),
		// then work backwards. If we get a null key as the first key after
		// the user, then there are no entries following the entries for this
		// user, so we'll seek to the end of the index and work backward.

		c := logIdxBucket.Cursor()
		id := []byte(strings.ToLower(email))
		id = append(id, 1)
		k, _ := c.Seek(id)
		if k == nil {
			k, _ = c.Last()
		} else {
			k, _ = c.Prev()
		}

		// *if* any index entries exist for this user, the cursor is now
		// positioned on the last of them and k contains the key
		id = []byte(strings.ToLower(email))
		id = append(id, 0)

		// id now contains the *actual* key prefix for index entries for this
		// user. We'll iterate backwards through the index as long as we are
		// still on a key that begins with the user's ID.
		for k != nil && bytes.HasPrefix(k, id) {

			// Only return as many rows as requested
			if len(results) == size {
				break
			}

			entryid := bytes.TrimPrefix(k, id)
			logentryjson := logBucket.Get(entryid)
			var logentry model.JobLogEntry
			if err := json.Unmarshal(logentryjson, &logentry); err != nil {
				return err
			}
			results = append(results, logentry)

			// Move back one more row
			k, _ = c.Prev()
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return results, nil
}

func (db *boltimpl) GetJobLog(size int) ([]model.JobLogEntry, error) {
	results := make([]model.JobLogEntry, 0, size)
	err := db.bdb.View(func(tx *bolt.Tx) error {
		logBucket := tx.Bucket([]byte(jobLogBucketName))
		c := logBucket.Cursor()

		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			var job model.JobLogEntry
			if err := json.Unmarshal(v, &job); err != nil {
				return err
			}
			results = append(results, job)
			if len(results) == size {
				break
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return results, nil
}
