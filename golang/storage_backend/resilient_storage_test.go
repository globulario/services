package storage_backend_test

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"testing"

	"github.com/globulario/services/golang/storage_backend"
)

// -----------------------------------------------------------------------------
// failStorage — a Storage implementation that always returns errors.
// Used to simulate mirror failure in tests.
// -----------------------------------------------------------------------------

type failStorage struct{}

var errFail = errors.New("storage: simulated failure")

func (f *failStorage) ReadFile(_ context.Context, _ string) ([]byte, error) { return nil, errFail }
func (f *failStorage) Open(_ context.Context, _ string) (io.ReadSeekCloser, error) {
	return nil, errFail
}
func (f *failStorage) Stat(_ context.Context, _ string) (fs.FileInfo, error)          { return nil, errFail }
func (f *failStorage) Exists(_ context.Context, _ string) bool                         { return false }
func (f *failStorage) ReadDir(_ context.Context, _ string) ([]fs.DirEntry, error)      { return nil, errFail }
func (f *failStorage) AtomicWriteFile(_ context.Context, _ string, _ []byte, _ fs.FileMode) error {
	return errFail
}
func (f *failStorage) WriteFile(_ context.Context, _ string, _ []byte, _ fs.FileMode) error {
	return errFail
}
func (f *failStorage) Create(_ context.Context, _ string) (io.WriteCloser, error) { return nil, errFail }
func (f *failStorage) MkdirAll(_ context.Context, _ string, _ fs.FileMode) error  { return errFail }
func (f *failStorage) RemoveAll(_ context.Context, _ string) error                 { return errFail }
func (f *failStorage) Remove(_ context.Context, _ string) error                    { return errFail }
func (f *failStorage) Rename(_ context.Context, _, _ string) error                 { return errFail }
func (f *failStorage) Ping(_ context.Context) error                                { return errFail }
func (f *failStorage) TempDir() string                                             { return "/tmp" }
func (f *failStorage) Getwd() (string, error)                                      { return "", errFail }

// -----------------------------------------------------------------------------
// Test A: WriteFile — local first, mirror failure is non-fatal
// -----------------------------------------------------------------------------

func TestResilientStorage_WriteLocalFirst(t *testing.T) {
	localDir := t.TempDir()
	local := storage_backend.NewOSStorage(localDir)
	mirror := &failStorage{} // mirror always fails

	rs := storage_backend.NewResilientStorage(local, mirror)
	ctx := context.Background()

	const path = "blobs/test.bin"
	payload := []byte("hello resilient")

	if err := rs.WriteFile(ctx, path, payload, 0o644); err != nil {
		t.Fatalf("WriteFile returned error despite local succeeding: %v", err)
	}

	// Verify the blob is readable from local directly.
	got, err := local.ReadFile(ctx, path)
	if err != nil {
		t.Fatalf("local ReadFile after write: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("local data mismatch: got %q want %q", got, payload)
	}
}

// -----------------------------------------------------------------------------
// Test B: ReadFile — local has blob, mirror is empty
// -----------------------------------------------------------------------------

func TestResilientStorage_ReadLocalFirst(t *testing.T) {
	localDir := t.TempDir()
	mirrorDir := t.TempDir()
	local := storage_backend.NewOSStorage(localDir)
	mirror := storage_backend.NewOSStorage(mirrorDir)

	ctx := context.Background()
	const path = "blobs/data.bin"
	payload := []byte("local data only")

	// Write only to local.
	if err := local.WriteFile(ctx, path, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	rs := storage_backend.NewResilientStorage(local, mirror)

	got, err := rs.ReadFile(ctx, path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("data mismatch: got %q want %q", got, payload)
	}
}

// -----------------------------------------------------------------------------
// Test C: ReadFile — blob only in mirror → fallback + cache-fill in local
// -----------------------------------------------------------------------------

func TestResilientStorage_ReadFallsBackToMirror(t *testing.T) {
	localDir := t.TempDir()
	mirrorDir := t.TempDir()
	local := storage_backend.NewOSStorage(localDir)
	mirror := storage_backend.NewOSStorage(mirrorDir)

	ctx := context.Background()
	const path = "blobs/mirror-only.bin"
	payload := []byte("mirror data")

	// Write only to mirror.
	if err := mirror.WriteFile(ctx, path, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	rs := storage_backend.NewResilientStorage(local, mirror)

	got, err := rs.ReadFile(ctx, path)
	if err != nil {
		t.Fatalf("ReadFile should fall back to mirror: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("data mismatch: got %q want %q", got, payload)
	}

	// Verify cache-fill: local should now have the file.
	localData, localErr := local.ReadFile(ctx, path)
	if localErr != nil {
		t.Fatalf("local should have been populated from mirror: %v", localErr)
	}
	if string(localData) != string(payload) {
		t.Errorf("local cache data mismatch: got %q want %q", localData, payload)
	}
}

// -----------------------------------------------------------------------------
// Test D: Ping — always succeeds even when mirror is down
// -----------------------------------------------------------------------------

func TestResilientStorage_PingAlwaysLocal(t *testing.T) {
	localDir := t.TempDir()
	local := storage_backend.NewOSStorage(localDir)
	mirror := &failStorage{} // mirror's Ping() returns an error

	rs := storage_backend.NewResilientStorage(local, mirror)
	ctx := context.Background()

	if err := rs.Ping(ctx); err != nil {
		t.Errorf("Ping should succeed (delegates to local): %v", err)
	}
}

// -----------------------------------------------------------------------------
// Test E: nil mirror — basic WriteFile/ReadFile work from local
// -----------------------------------------------------------------------------

func TestResilientStorage_NilMirror(t *testing.T) {
	localDir := t.TempDir()
	local := storage_backend.NewOSStorage(localDir)

	rs := storage_backend.NewResilientStorage(local, nil) // no mirror
	ctx := context.Background()

	const path = "blobs/no-mirror.bin"
	payload := []byte("local only, no mirror configured")

	if err := rs.WriteFile(ctx, path, payload, 0o644); err != nil {
		t.Fatalf("WriteFile (nil mirror): %v", err)
	}

	got, err := rs.ReadFile(ctx, path)
	if err != nil {
		t.Fatalf("ReadFile (nil mirror): %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("data mismatch: got %q want %q", got, payload)
	}

	if storage_backend.IsMirrorAvailable(rs) {
		t.Error("IsMirrorAvailable should return false for nil mirror")
	}
}

// -----------------------------------------------------------------------------
// Test F: IsMirrorAvailable — true when mirror is set
// -----------------------------------------------------------------------------

func TestResilientStorage_IsMirrorAvailable(t *testing.T) {
	localDir := t.TempDir()
	mirrorDir := t.TempDir()
	local := storage_backend.NewOSStorage(localDir)
	mirror := storage_backend.NewOSStorage(mirrorDir)

	rs := storage_backend.NewResilientStorage(local, mirror)
	if !storage_backend.IsMirrorAvailable(rs) {
		t.Error("IsMirrorAvailable should return true when mirror is set")
	}

	// A plain OSStorage (not ResilientStorage) should return false.
	plain := storage_backend.NewOSStorage(localDir)
	if storage_backend.IsMirrorAvailable(plain) {
		t.Error("IsMirrorAvailable should return false for non-ResilientStorage")
	}
}
