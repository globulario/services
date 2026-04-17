package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RunningBinary represents a Globular binary that is actually executing.
type RunningBinary struct {
	ServiceName string // canonical service name (e.g. "workflow", "dns")
	BinaryPath  string // absolute path to the binary
	Checksum    string // sha256 of the binary file
	PID         int
}

// checksumCache caches binary checksums keyed by path.
// Entries are invalidated when the file's modification time changes.
var checksumCache = struct {
	sync.Mutex
	entries map[string]cachedChecksum
}{entries: make(map[string]cachedChecksum)}

type cachedChecksum struct {
	checksum string
	modTime  time.Time
	size     int64
}

// DiscoverRunningBinaries scans /proc for processes whose binary lives in
// /usr/lib/globular/bin. This captures both:
//   - services running as the "globular" user
//   - node-agent running as root
//
// Checksums are cached and only recomputed when the binary changes on disk.
// Returns a map of canonical service name → RunningBinary.
func DiscoverRunningBinaries() map[string]RunningBinary {
	result := make(map[string]RunningBinary)

	entries, err := os.ReadDir("/proc")
	if err != nil {
		log.Printf("process-fingerprint: cannot read /proc: %v", err)
		return result
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		exePath, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
		if err != nil {
			continue
		}

		if !strings.HasPrefix(exePath, globularBinDir+"/") {
			continue
		}

		binName := filepath.Base(exePath)
		svcName := binaryToServiceName(binName)
		if svcName == "" {
			continue
		}

		if _, exists := result[svcName]; exists {
			continue
		}

		checksum, err := cachedSha256(exePath)
		if err != nil {
			continue
		}

		result[svcName] = RunningBinary{
			ServiceName: svcName,
			BinaryPath:  exePath,
			Checksum:    checksum,
			PID:         pid,
		}
	}

	return result
}

// cachedSha256 returns the SHA256 of a file, using a cache keyed by
// (path, modTime, size). Only recomputes when the file changes.
func cachedSha256(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	checksumCache.Lock()
	if cached, ok := checksumCache.entries[path]; ok {
		if cached.modTime.Equal(info.ModTime()) && cached.size == info.Size() {
			checksumCache.Unlock()
			return cached.checksum, nil
		}
	}
	checksumCache.Unlock()

	// Cache miss or stale — compute.
	checksum, err := sha256File(path)
	if err != nil {
		return "", err
	}

	checksumCache.Lock()
	checksumCache.entries[path] = cachedChecksum{
		checksum: checksum,
		modTime:  info.ModTime(),
		size:     info.Size(),
	}
	checksumCache.Unlock()

	return checksum, nil
}

// binaryToServiceName converts a binary filename to a canonical service name.
func binaryToServiceName(binName string) string {
	name := strings.TrimSuffix(binName, "_server")
	name = strings.TrimSuffix(name, "_service")
	if name == binName {
		return ""
	}
	name = strings.ReplaceAll(name, "_", "-")
	return name
}

// sha256File computes the SHA256 hash of a file.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
