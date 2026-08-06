// Package raft provides CaimanDB's Raft integration: a Badger-backed
// raft.LogStore implementation used to persist the replicated log.
package raft

import (
	"encoding/json"
	"errors"
	"fmt"
	badger "github.com/dgraph-io/badger/v4"
	"github.com/hashicorp/raft"
	"os"
)

type BadgerLogStore struct {
	db *badger.DB
}

func NewBadgerLogStore(path string) (*BadgerLogStore, error) {
	if err := os.MkdirAll(path, 0750); err != nil {
		return nil, err
	}

	opts := badger.DefaultOptions(path).
		WithLogger(nil).
		WithCompression(2).
		WithValueLogFileSize(64 << 20).
		WithMemTableSize(64 << 20).
		WithNumMemtables(4).
		WithNumLevelZeroTables(8)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	return &BadgerLogStore{db: db}, nil
}

func (b *BadgerLogStore) FirstIndex() (uint64, error) {
	var idx uint64
	err := b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.PrefetchSize = 1
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte("log-")
		it.Seek(prefix)

		if !it.ValidForPrefix(prefix) {
			idx = 0
			return nil
		}

		item := it.Item()
		key := string(item.KeyCopy(nil))
		if _, err := fmt.Sscanf(key, "log-%d", &idx); err != nil {
			idx = 0
			return nil
		}
		return nil
	})
	return idx, err
}

func (b *BadgerLogStore) LastIndex() (uint64, error) {
	var idx uint64
	err := b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.PrefetchSize = 1
		opts.Reverse = true
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte("log-")
		it.Seek(prefix)

		if !it.ValidForPrefix(prefix) {
			idx = 0
			return nil
		}

		item := it.Item()
		key := string(item.KeyCopy(nil))
		if _, err := fmt.Sscanf(key, "log-%d", &idx); err != nil {
			idx = 0
			return nil
		}
		return nil
	})
	return idx, err
}

func (b *BadgerLogStore) GetLog(idx uint64, entry *raft.Log) error {
	key := fmt.Sprintf("log-%d", idx)
	return b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return raft.ErrLogNotFound
			}
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, entry)
		})
	})
}

func (b *BadgerLogStore) StoreLog(log *raft.Log) error {
	return b.StoreLogs([]*raft.Log{log})
}

func (b *BadgerLogStore) StoreLogs(logs []*raft.Log) error {
	if len(logs) == 0 {
		return nil
	}
	return b.db.Update(func(txn *badger.Txn) error {
		for _, log := range logs {
			key := fmt.Sprintf("log-%d", log.Index)
			data, err := json.Marshal(log)
			if err != nil {
				return err
			}
			if err := txn.Set([]byte(key), data); err != nil {
				return err
			}
		}
		return nil
	})
}

func (b *BadgerLogStore) DeleteRange(min, max uint64) error {
	return b.db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		var keys [][]byte
		prefix := []byte("log-")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := string(item.KeyCopy(nil))
			var idx uint64
			fmt.Sscanf(key, "log-%d", &idx)
			if idx >= min && idx <= max {
				keys = append(keys, item.KeyCopy(nil))
			}
		}

		for _, k := range keys {
			if err := txn.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}

func (b *BadgerLogStore) Close() error {
	return b.db.Close()
}
