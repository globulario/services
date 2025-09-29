package storage_store

import (
	"bytes"
	"context"
	"strings"
)

// NewScylla_store constructs a ScyllaStore and starts its serialized action loop.
// Kept for backward compatibility with older call sites.
// Address/keyspace args are optional and only used if Open is invoked without JSON.
func NewScylla_store(address string, keySpace string, replicationFactor int) *ScyllaStore {
	s := &ScyllaStore{
		actions: make(chan action, 64),
	}
	// Start the loop.
	go s.Run(context.Background())
	return s
}

// Open opens the store using a JSON options document (preferred).
// The reader can be nil; in that case, address/keySpace act as fallbacks.
// --- NEW: public Open keeping your requested prototype ---
// Accepts a JSON options string, initializes a reader, and either posts to the
// action loop (if present) or opens directly.
func (store *ScyllaStore) Open(optionsStr string) error {
	// If no serialized loop is running, open directly.
	if store.actions == nil {
		return store.open(strings.NewReader(optionsStr), "", "", "")
	}
	// Otherwise, send through the loop; Run() will turn the string into a reader.
	errCh := make(chan error, 1)
	store.actions <- action{
		name: "open",
		args: []any{optionsStr}, // Run() handles string -> strings.NewReader(...)
		errCh: errCh,
	}
	return <-errCh
}
// Close closes the store.
func (store *ScyllaStore) Close() error {
	errCh := make(chan error, 1)
	store.actions <- action{name: "close", errCh: errCh}
	return <-errCh
}

// SetItem stores a value by key.
func (store *ScyllaStore) SetItem(key string, val []byte) error {
	errCh := make(chan error, 1)
	store.actions <- action{name: "setitem", args: []any{key, val}, errCh: errCh}
	return <-errCh
}

// SetItemWithTTL stores a value with a TTL in seconds.
func (store *ScyllaStore) SetItemWithTTL(key string, val []byte, ttlSeconds int) error {
	errCh := make(chan error, 1)
	store.actions <- action{name: "setitem", args: []any{key, val, ttlSeconds}, errCh: errCh}
	return <-errCh
}

// GetItem loads a value.
func (store *ScyllaStore) GetItem(key string) ([]byte, error) {
	resCh := make(chan any, 1)
	errCh := make(chan error, 1)
	store.actions <- action{name: "getitem", args: []any{key}, resCh: resCh, errCh: errCh}
	if err := <-errCh; err != nil {
		return nil, err
	}
	if v := <-resCh; v != nil {
		if b, ok := v.([]byte); ok {
			// return a copy for safety
			return bytes.Clone(b), nil
		}
	}
	return nil, nil
}

// RemoveItem deletes a key.
func (store *ScyllaStore) RemoveItem(key string) error {
	errCh := make(chan error, 1)
	store.actions <- action{name: "removeitem", args: []any{key}, errCh: errCh}
	return <-errCh
}

// Clear truncates the table.
func (store *ScyllaStore) Clear() error {
	errCh := make(chan error, 1)
	store.actions <- action{name: "clear", errCh: errCh}
	return <-errCh
}

// Drop removes the table.
func (store *ScyllaStore) Drop() error {
	errCh := make(chan error, 1)
	store.actions <- action{name: "drop", errCh: errCh}
	return <-errCh
}

// GetAllKeys retrieves all keys in the store.
func (store *ScyllaStore) GetAllKeys() ([]string, error) {
	resCh := make(chan any, 1)
	errCh := make(chan error, 1)
	store.actions <- action{name: "getallkeys", resCh: resCh, errCh: errCh}
	if err := <-errCh; err != nil {
		return nil, err
	}
	if v := <-resCh; v != nil {
		if keys, ok := v.([]string); ok {
			// return a copy for safety
			copiedKeys := make([]string, len(keys))
			copy(copiedKeys, keys)
			return copiedKeys, nil
		}
	}
	return nil, nil
}
