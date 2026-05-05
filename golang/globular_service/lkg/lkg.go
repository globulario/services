// Package lkg provides a shared last-known-good (LKG) persistence contract for
// critical runtime consumers. LKG entries are written atomically, versioned by
// generation, and protected by checksum so consumers can detect corruption
// before applying stale or invalid state.
//
// Invariant (Case 04): if desired state cannot be read, use last-known-good;
// mark node degraded; do not destructively change runtime.
package lkg

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var baseDir = "/var/lib/globular"

// OverrideBaseDir redirects where LKG files are written. Intended for tests only.
func OverrideBaseDir(dir string) { baseDir = dir }

// BaseDir returns the current LKG base directory.
func BaseDir() string { return baseDir }

// Entry is the on-disk envelope written by Store and read by Load.
type Entry struct {
	Subsystem     string          `json:"subsystem"`
	Key           string          `json:"key"`
	Generation    int64           `json:"generation"`
	WrittenAtUnix int64           `json:"written_at_unix"`
	Checksum      string          `json:"checksum"`
	Data          json.RawMessage `json:"data"`
}

// Store atomically writes data as the LKG entry for the given subsystem and key.
// The generation must be strictly greater than the stored generation; if a valid
// LKG with the same or higher generation already exists the write is skipped and
// nil is returned (idempotent).
func Store(subsystem, key string, generation int64, data json.RawMessage) error {
	path := lkgPath(subsystem, key)
	if existing, err := Load(subsystem, key); err == nil && existing.Generation >= generation {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("lkg mkdir: %w", err)
	}
	entry := Entry{
		Subsystem:     subsystem,
		Key:           key,
		Generation:    generation,
		WrittenAtUnix: time.Now().Unix(),
		Data:          data,
	}
	entry.Checksum = computeChecksum(entry)
	b, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("lkg marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o640); err != nil {
		return fmt.Errorf("lkg write tmp: %w", err)
	}
	return os.Rename(tmp, path)
}

// StoreRaw stores raw bytes as the LKG entry data (wraps the bytes in a JSON
// string for the Data field). Convenience wrapper for text/JSON blobs.
func StoreRaw(subsystem, key string, generation int64, raw []byte) error {
	data, err := json.Marshal(string(raw))
	if err != nil {
		return err
	}
	return Store(subsystem, key, generation, json.RawMessage(data))
}

// Load reads and validates the LKG entry for the given subsystem and key.
// Returns ErrNotFound if no LKG exists. Returns ErrCorrupt if the stored
// checksum does not match the content.
func Load(subsystem, key string) (*Entry, error) {
	path := lkgPath(subsystem, key)
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lkg read: %w", err)
	}
	var entry Entry
	if err := json.Unmarshal(b, &entry); err != nil {
		return nil, ErrCorrupt
	}
	stored := entry.Checksum
	entry.Checksum = ""
	if computeChecksum(entry) != stored {
		return nil, ErrCorrupt
	}
	entry.Checksum = stored
	return &entry, nil
}

// LoadRaw reads the LKG entry and returns the raw bytes stored in Data (unquotes
// the JSON string). Convenience wrapper that mirrors StoreRaw.
func LoadRaw(subsystem, key string) ([]byte, error) {
	entry, err := Load(subsystem, key)
	if err != nil {
		return nil, err
	}
	var s string
	if err := json.Unmarshal(entry.Data, &s); err != nil {
		// Data was stored directly as a JSON object — return raw bytes.
		return entry.Data, nil
	}
	return []byte(s), nil
}

// Remove deletes the LKG file. Safe to call when no LKG exists.
func Remove(subsystem, key string) error {
	err := os.Remove(lkgPath(subsystem, key))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// ErrNotFound is returned when no LKG file exists for the requested key.
var ErrNotFound = fmt.Errorf("lkg: no last-known-good found")

// ErrCorrupt is returned when the LKG file exists but fails checksum validation.
var ErrCorrupt = fmt.Errorf("lkg: last-known-good is corrupt (checksum mismatch)")

func lkgPath(subsystem, key string) string {
	return filepath.Join(baseDir, subsystem, key+"-last-known-good.json")
}

func computeChecksum(e Entry) string {
	e.Checksum = ""
	b, _ := json.Marshal(e)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
