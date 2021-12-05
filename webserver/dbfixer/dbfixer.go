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
	"bytes"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/boltdb/bolt"
)

const (
	userBucketName      = "users"
	accessKeyBucketName = "access_keys"
	jobLogBucketName    = "job_log"
	jobLogUserIndexName = "job_log_user_index"
	configBucketName    = "config"
	autocertBucketName  = "autocert"
	deleteLogBucketName = "delete_log"
	pdfBucketName       = "pdfs"
)

func main() {
	// Just a wrapper around realmain so we can easily return exit codes but
	// allow any defer()s in realmain to run.
	result := realmain()
	if result != 0 {
		os.Exit(result)
	}
}

func realmain() int {
	if len(os.Args) != 3 {
		fmt.Println("Must use two arguments: infile outfile")
		return 1
	}

	oldName := os.Args[1]
	newName := os.Args[2]

	fmt.Printf("Converting %s to %s\n", oldName, newName)

	old, err := bolt.Open(oldName, 0600, nil)
	if err != nil {
		fmt.Printf("Error opening %s: %v\n", oldName, err)
		return 1
	}
	defer old.Close()

	new, err := bolt.Open(newName, 0600, nil)
	if err != nil {
		fmt.Printf("Error opening %s: %v\n", newName, err)
		return 1
	}
	defer new.Close()

	if err := createBuckets(new); err != nil {
		fmt.Printf("Error creating buckets in new database: %v\n", err)
		return 1
	}

	// Copy the tables that require no conversion
	buckets := []string{userBucketName, accessKeyBucketName,
		configBucketName, autocertBucketName}
	for _, b := range buckets {
		fmt.Printf("Copying bucket %s\n", b)
		if err := copyBucket(old, new, b); err != nil {
			fmt.Printf("FAILED: %v\n", err)
			return 1
		} else {
			fmt.Printf("Done copying bucket %s\n", b)
		}
	}

	// Fix job log
	fmt.Printf("Fixing bucket %s\n", jobLogBucketName)
	if err := fixSimple(old, new, jobLogBucketName); err != nil {
		fmt.Printf("FAILED: %v\n", err)
		return 1
	}

	// Fix user job index
	fmt.Printf("Fixing bucket %s\n", jobLogUserIndexName)
	if err := fixUserJobIndex(old, new); err != nil {
		fmt.Printf("FAILED: %v\n", err)
		return 1
	}

	// Fix delete log
	fmt.Printf("Fixing bucket %s\n", deleteLogBucketName)
	if err := fixSimple(old, new, deleteLogBucketName); err != nil {
		fmt.Printf("FAILED: %v\n", err)
		return 1
	}

	// Fix pdf bucket
	fmt.Printf("Fixing bucket %s\n", pdfBucketName)
	if err := fixSimple(old, new, pdfBucketName); err != nil {
		fmt.Printf("FAILED: %v\n", err)
		return 1
	}

	// Set the sequence number for each table
	allTables := []string{userBucketName, accessKeyBucketName,
		jobLogBucketName, jobLogUserIndexName, configBucketName,
		autocertBucketName, deleteLogBucketName, pdfBucketName}
	for _, b := range allTables {
		fmt.Printf("Setting sequence for bucket %s\n", b)
		if err := copySequence(old, new, b); err != nil {
			fmt.Printf("FAILED: %v\n", err)
			return 1
		}
	}

	fmt.Println("Done!")

	return 0
}

func createBuckets(db *bolt.DB) error {
	// Ensure existence of all buckets
	return db.Update(func(tx *bolt.Tx) error {
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
		if _, err := tx.CreateBucketIfNotExists(
			[]byte(configBucketName)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(
			[]byte(autocertBucketName)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(
			[]byte(deleteLogBucketName)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(
			[]byte(pdfBucketName)); err != nil {
			return err
		}
		return nil
	})
}

func put(db *bolt.DB, bucket string, key []byte, value []byte) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		return b.Put(key, value)
	})
}

func copySequence(old, new *bolt.DB, bucket string) error {
	return old.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		seq := b.Sequence()
		return setSequence(new, bucket, seq)
	})
}

func setSequence(db *bolt.DB, bucket string, seq uint64) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		return b.SetSequence(seq)
	})
}

func copyBucket(old, new *bolt.DB, bucket string) error {
	return old.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		return b.ForEach(func(k, v []byte) error {
			return put(new, bucket, k, v)
		})
	})
}

// fix a bucket where the key is a uint64
func fixSimple(old, new *bolt.DB, bucket string) error {
	return old.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		return b.ForEach(func(k, v []byte) error {
			id, err := bytesToUint64LE(k)
			if err != nil {
				return err
			}
			newid := uint64ToBytesBE(id)
			return put(new, bucket, newid, v)
		})
	})
}

func fixUserJobIndex(old, new *bolt.DB) error {
	return old.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(jobLogUserIndexName))
		return b.ForEach(func(k, v []byte) error {
			// The key is the lowercase username (email), followed by a null
			// byte (0), followed by the 64-bit job log ID. There is no value,
			// since the key itself serves as a pointer to the job log entry.

			parts := bytes.SplitN(k, []byte{0}, 2)
			if len(parts) != 2 {
				return fmt.Errorf("got %d parts when splitting key",
					len(parts))
			}

			id, err := bytesToUint64LE(parts[1])
			if err != nil {
				return err
			}
			newid := uint64ToBytesBE(id)
			indexEntry := parts[0]
			indexEntry = append(indexEntry, 0)
			indexEntry = append(indexEntry, newid...)

			return put(new, jobLogUserIndexName, indexEntry, v)
		})
	})
}

func uint64ToBytesBE(in uint64) []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, &in)
	return buf.Bytes()
}

func bytesToUint64LE(in []byte) (uint64, error) {
	inrdr := bytes.NewReader(in)
	var out uint64
	err := binary.Read(inrdr, binary.LittleEndian, &out)
	return out, err
}
