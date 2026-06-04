package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestHealAuditStore_RotatesAtMaxBytes pins meta.diagnostic_output_must_be_bounded
// for the heal audit JSONL: when the live file would exceed maxBytes, it is
// rotated to .1 (older backups shift, oldest is dropped). Without this, a
// chronic invariant violation would grow the file linearly until disk fill.
func TestHealAuditStore_RotatesAtMaxBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "heal.jsonl")

	s := &HealAuditStore{
		path:       path,
		maxBytes:   512, // tight so a few records trigger rotation
		maxBackups: 2,
	}

	rec := HealAuditRecord{
		Timestamp:   time.Now(),
		InvariantID: "test.invariant",
		EntityRef:   "node-1/pkg-a",
		Disposition: HealAuto,
		Action:      "restart",
		Executed:    true,
	}

	// Write enough records to force two rotations.
	for i := 0; i < 40; i++ {
		rec.CycleID = fmt.Sprintf("cycle-%03d", i)
		if err := s.Append(rec); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	// Live file must remain under cap (the most recent append may push it
	// up to maxBytes+oneLine just before the next rotation).
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat live: %v", err)
	}
	if fi.Size() > s.maxBytes*2 {
		t.Fatalf("live file unbounded: %d bytes > 2×maxBytes (%d)", fi.Size(), s.maxBytes*2)
	}

	// At least one backup must exist after this many writes.
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("expected %s.1 after rotation: %v", path, err)
	}

	// Backups beyond maxBackups must be dropped.
	dropped := fmt.Sprintf("%s.%d", path, s.maxBackups+1)
	if _, err := os.Stat(dropped); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be absent (beyond maxBackups), got err=%v", dropped, err)
	}
}

// TestHealAuditStore_NoRotationWhenDisabled documents the maxBytes<=0 escape
// hatch — useful for tests and operators who want a single growing file.
func TestHealAuditStore_NoRotationWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "heal.jsonl")

	s := &HealAuditStore{
		path:       path,
		maxBytes:   0, // disabled
		maxBackups: 3,
	}

	rec := HealAuditRecord{
		Timestamp:   time.Now(),
		InvariantID: "test.invariant",
		Executed:    true,
	}
	for i := 0; i < 20; i++ {
		rec.CycleID = fmt.Sprintf("cycle-%03d", i)
		if err := s.Append(rec); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	if _, err := os.Stat(path + ".1"); !os.IsNotExist(err) {
		t.Fatalf("rotation should be disabled, but %s.1 exists", path)
	}
}
