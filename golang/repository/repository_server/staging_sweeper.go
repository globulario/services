package main

// staging_sweeper.go — D5a: reap crash-orphaned atomic-write temp files.
//
// WriteFileAtomic / AtomicWriteFile commit by writing "<target>.tmp.<uuidv4>",
// fsyncing, then renaming into place. On any in-process failure a deferred cleanup
// removes the temp. But if the process crashes between create and rename, the temp
// survives with no owner. This janitor removes those orphans.
//
// Safety (must NEVER delete in-use content): a candidate must BOTH
//   1. match storage_backend.IsAtomicTempName (a committed blob ".bin" or manifest
//      ".manifest.json" never matches the ".tmp.<uuid>" shape), AND
//   2. be older than orphanTempMaxAge — far beyond the upload reservation TTL, so
//      an in-flight atomic write or an active upload (younger than the gate) is
//      never a candidate.
// No artifact, blob, or manifest is ever eligible, so
// repository.purge_must_not_delete_active_desired_builds is not implicated.

import (
	"context"
	"time"

	"github.com/globulario/services/golang/storage_backend"
)

// orphanTempMaxAge is the minimum age before an atomic-write temp file is treated
// as a crash orphan. It is deliberately much larger than the upload reservation
// TTL (5m, allocate_upload.go) so the sweeper can never race an in-flight write:
// an atomic write completes in seconds, and a slow upload is bounded by the TTL.
const orphanTempMaxAge = 30 * time.Minute

// sweepOrphanTempBlobs removes orphaned ".tmp.<uuid>" files older than
// orphanTempMaxAge from the local POSIX artifacts directory. Best-effort: it logs
// and returns the count removed; individual errors are skipped, never fatal.
func (srv *server) sweepOrphanTempBlobs(ctx context.Context, now time.Time) (removed int) {
	if srv.localStorage == nil {
		return 0
	}
	entries, err := srv.localStorage.ReadDir(ctx, artifactsDir)
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !storage_backend.IsAtomicTempName(name) {
			continue // committed .bin / .manifest.json never match — only atomic temps
		}
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		age := now.Sub(info.ModTime())
		if age < orphanTempMaxAge {
			continue // too recent — could be an in-flight or just-completed write
		}
		if rerr := srv.localStorage.Remove(ctx, artifactsDir+"/"+name); rerr == nil {
			removed++
			logger.Info("staging-sweeper: removed orphan atomic-write temp",
				"name", name, "age", age.Round(time.Second))
		}
	}
	if removed > 0 {
		logger.Info("staging-sweeper: swept orphan temp files", "removed", removed)
	}
	return removed
}
