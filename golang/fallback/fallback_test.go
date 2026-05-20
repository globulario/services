package fallback

// fallback_test.go — Phase 6 of the Diagnostic Honesty Refactor.
//
// Pins the contract of Enter / Exit / ExitMatching / Snapshot /
// SetNotifier. The point of the primitive is to guarantee:
//
//   1. Every fallback path is discoverable via Snapshot() — no silent
//      degraded mode hiding in a slog.Warn call.
//   2. Enter is idempotent: a single fallback condition that fires from
//      many goroutines must produce ONE registry entry with the original
//      Since timestamp preserved.
//   3. PrimaryError and AffectedPaths refresh on re-entry — operators
//      want the latest evidence, not the first.
//   4. Notifier fires only on real transitions, not on duplicate Enters
//      or on Exits with no record present.

import (
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"
)

type recordingNotifier struct {
	mu      sync.Mutex
	entered []Active
	exited  []Active
}

func (r *recordingNotifier) OnEnter(a Active) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entered = append(r.entered, a)
}

func (r *recordingNotifier) OnExit(a Active) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.exited = append(r.exited, a)
}

func (r *recordingNotifier) snapshotEntered() []Active {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Active, len(r.entered))
	copy(out, r.entered)
	return out
}

func (r *recordingNotifier) snapshotExited() []Active {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Active, len(r.exited))
	copy(out, r.exited)
	return out
}

func TestEnter_StoresWithSinceStamped(t *testing.T) {
	resetForTest()
	defer resetForTest()
	before := time.Now()
	out := Enter(Active{
		Service:    "repository",
		Dependency: "scylladb",
		Mode:       "minio_read",
	})
	if out.Since.IsZero() {
		t.Fatal("Enter must stamp Since when caller leaves it zero")
	}
	if out.Since.Before(before) {
		t.Errorf("Since=%v is before call start %v", out.Since, before)
	}
	if got := Snapshot(); len(got) != 1 || got[0].Service != "repository" {
		t.Errorf("Snapshot=%v want one repository entry", got)
	}
}

func TestEnter_MissingRequiredFields_NoOp(t *testing.T) {
	resetForTest()
	defer resetForTest()
	Enter(Active{Dependency: "scylladb", Mode: "minio_read"})  // no Service
	Enter(Active{Service: "repository", Mode: "minio_read"})   // no Dependency
	Enter(Active{Service: "repository", Dependency: "scylla"}) // no Mode
	if got := Snapshot(); len(got) != 0 {
		t.Errorf("Snapshot=%v want empty (each Enter missing required field)", got)
	}
}

func TestEnter_Idempotent_SincePreserved(t *testing.T) {
	resetForTest()
	defer resetForTest()
	first := Enter(Active{Service: "repository", Dependency: "scylladb", Mode: "minio_read"})
	time.Sleep(2 * time.Millisecond)
	second := Enter(Active{Service: "repository", Dependency: "scylladb", Mode: "minio_read"})
	if !second.Since.Equal(first.Since) {
		t.Errorf("re-entry reset Since: first=%v second=%v", first.Since, second.Since)
	}
	if got := Snapshot(); len(got) != 1 {
		t.Errorf("Snapshot=%v want single entry after duplicate Enter", got)
	}
}

func TestEnter_RefreshesPrimaryErrorAndAffectedPaths(t *testing.T) {
	resetForTest()
	defer resetForTest()
	Enter(Active{
		Service:       "repository",
		Dependency:    "scylladb",
		Mode:          "minio_read",
		PrimaryError:  "first error",
		AffectedPaths: []string{"a"},
	})
	out := Enter(Active{
		Service:       "repository",
		Dependency:    "scylladb",
		Mode:          "minio_read",
		PrimaryError:  "second error",
		AffectedPaths: []string{"b", "c"},
	})
	if out.PrimaryError != "second error" {
		t.Errorf("PrimaryError=%q want=%q (must refresh on re-entry)", out.PrimaryError, "second error")
	}
	if !reflect.DeepEqual(out.AffectedPaths, []string{"b", "c"}) {
		t.Errorf("AffectedPaths=%v want=[b c]", out.AffectedPaths)
	}
}

func TestEnter_NotifierFiresOnceOnFirstEntry(t *testing.T) {
	resetForTest()
	defer resetForTest()
	n := &recordingNotifier{}
	SetNotifier(n)
	Enter(Active{Service: "repository", Dependency: "scylladb", Mode: "minio_read"})
	Enter(Active{Service: "repository", Dependency: "scylladb", Mode: "minio_read"})
	Enter(Active{Service: "repository", Dependency: "scylladb", Mode: "minio_read"})
	got := n.snapshotEntered()
	if len(got) != 1 {
		t.Errorf("OnEnter fired %d times; want 1 (idempotent)", len(got))
	}
}

func TestExit_RemovesEntry(t *testing.T) {
	resetForTest()
	defer resetForTest()
	Enter(Active{Service: "repository", Dependency: "scylladb", Mode: "minio_read"})
	if !Exit("repository", "scylladb", "minio_read") {
		t.Error("Exit returned false; want true")
	}
	if got := Snapshot(); len(got) != 0 {
		t.Errorf("Snapshot=%v want empty after Exit", got)
	}
}

func TestExit_NoopWhenAbsent(t *testing.T) {
	resetForTest()
	defer resetForTest()
	if Exit("repository", "scylladb", "minio_read") {
		t.Error("Exit returned true for absent key; want false")
	}
}

func TestExit_NotifierFiresOnRemoval(t *testing.T) {
	resetForTest()
	defer resetForTest()
	n := &recordingNotifier{}
	SetNotifier(n)
	Enter(Active{Service: "repository", Dependency: "scylladb", Mode: "minio_read"})
	Exit("repository", "scylladb", "minio_read")
	if got := n.snapshotExited(); len(got) != 1 {
		t.Errorf("OnExit fired %d times; want 1", len(got))
	}
	// And a redundant Exit must NOT fire the notifier again.
	Exit("repository", "scylladb", "minio_read")
	if got := n.snapshotExited(); len(got) != 1 {
		t.Errorf("OnExit fired %d times after redundant Exit; want still 1", len(got))
	}
}

func TestExitMatching_ClearsByDependency(t *testing.T) {
	resetForTest()
	defer resetForTest()
	Enter(Active{Service: "repository", Dependency: "scylladb", Mode: "minio_read"})
	Enter(Active{Service: "repository", Dependency: "scylladb", Mode: "in_memory_cache"})
	Enter(Active{Service: "dns", Dependency: "scylladb", Mode: "in_memory_cache"})
	Enter(Active{Service: "file", Dependency: "minio", Mode: "local_disk"})
	cleared := ExitMatching(func(a Active) bool { return a.Dependency == "scylladb" })
	if len(cleared) != 3 {
		t.Errorf("ExitMatching returned %d cleared; want 3", len(cleared))
	}
	rest := Snapshot()
	if len(rest) != 1 || rest[0].Dependency != "minio" {
		t.Errorf("Snapshot=%v want only the minio fallback remaining", rest)
	}
}

func TestSnapshot_StableOrder(t *testing.T) {
	resetForTest()
	defer resetForTest()
	// Insert in non-alphabetical order; expect alphabetical (service,
	// then dependency, then mode) on the way out.
	Enter(Active{Service: "z", Dependency: "scylladb", Mode: "y"})
	Enter(Active{Service: "a", Dependency: "etcd", Mode: "x"})
	Enter(Active{Service: "a", Dependency: "scylladb", Mode: "y"})
	got := Snapshot()
	want := []string{"a|etcd|x", "a|scylladb|y", "z|scylladb|y"}
	gotKeys := make([]string, len(got))
	for i, a := range got {
		gotKeys[i] = keyFor(a.Service, a.Dependency, a.Mode)
	}
	if !reflect.DeepEqual(gotKeys, want) {
		t.Errorf("Snapshot order=%v want=%v", gotKeys, want)
	}
}

func TestKeyFor_CaseInsensitive(t *testing.T) {
	if keyFor("Repository", "ScyllaDB", "MinIO_Read") != keyFor("repository", "scylladb", "minio_read") {
		t.Errorf("keyFor must be case-insensitive — accidental case drift between callsites would fragment the directory")
	}
}

func TestEnter_ConcurrentDoesNotPanic(t *testing.T) {
	resetForTest()
	defer resetForTest()
	const goroutines = 8
	const iters = 200
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iters; j++ {
				Enter(Active{Service: "repository", Dependency: "scylladb", Mode: "minio_read"})
			}
		}()
	}
	wg.Wait()
	// Single entry regardless of concurrent Enters.
	if got := Snapshot(); len(got) != 1 {
		t.Errorf("Snapshot=%v want 1 after concurrent Enter blizzard", got)
	}
}

// Sanity: the FindingID constant must match the value documented in
// failure_modes.yaml so doctor / verifier code can rely on it.
func TestFindingID_PinsContract(t *testing.T) {
	if FindingID != "service.silent_fallback_active" {
		t.Errorf("FindingID=%q must be service.silent_fallback_active to match failure_modes.yaml", FindingID)
	}
}

func TestSnapshot_Empty_ReturnsNil(t *testing.T) {
	resetForTest()
	defer resetForTest()
	got := Snapshot()
	if len(got) != 0 {
		t.Errorf("Snapshot of empty registry = %v want empty", got)
	}
	// Stable: nil vs empty-slice doesn't matter for consumers, just pin
	// that range-over works without panic.
	for range got {
		t.Fatal("unreachable")
	}
	sort.Slice(got, func(i, j int) bool { return got[i].Service < got[j].Service }) // no panic
}
