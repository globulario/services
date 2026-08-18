//go:build unix

package evolution

import (
	"fmt"
	"os"
	"syscall"
)

// withEnvelopeLock serializes read-modify-write on one ChangeEnvelope file.
//
// Scenario invocations for the same change can overlap, and a simulation runs
// for a long time between loading the envelope and persisting a result. Without
// this, an invocation that finished later would write back the snapshot it read
// before its run, silently undoing whatever another invocation decided in the
// meantime — including a withdrawal, which would resurrect a proof that had
// been invalidated.
//
// The lock is an OS advisory lock on a sidecar file, so it is released when the
// descriptor closes or the process dies. There are no stale locks to reap.
func withEnvelopeLock(envelopePath string, fn func() error) error {
	lockPath := envelopePath + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("open change envelope lock: %w", err)
	}
	defer func() { _ = f.Close() }()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("acquire change envelope lock: %w", err)
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()
	return fn()
}
