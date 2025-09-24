package storage_client

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/storage/storagepb"
)

// ---------- env helpers ----------

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

var (
	// Point this to where your storage service is reachable; using a fixed address
	// reduces “sticky” routing issues compared to a domain LB.
	storageAddr = getenv("STORAGE_ADDR", "localhost")
	serviceID   = getenv("STORAGE_SERVICE_ID", "storage.StorageService")
)

// buildOptionsFileBased returns JSON: {"path": "...", "name": "..."} and ensures path exists.
func buildOptionsFileBased(t *testing.T, baseDir, name string) string {
	t.Helper()
	if baseDir == "" {
		baseDir = t.TempDir()
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("failed to create baseDir %q: %v", baseDir, err)
	}
	opts := map[string]any{
		"path": baseDir,
		"name": name,
	}
	b, _ := json.Marshal(opts)
	return string(b)
}

func buildOptionsBigCache() string {
	opts := map[string]any{"lifeWindowSec": 30}
	b, _ := json.Marshal(opts)
	return string(b)
}

// Scylla options (opt-in with env):
//   SCYLLA_HOSTS="10.0.0.63:9042,10.0.0.64:9042"
//   SCYLLA_KEYSPACE="storage_test"  (required if SCYLLA_HOSTS set)
//   SCYLLA_TABLE="kv"               (optional; default kv)
func buildOptionsScylla(t *testing.T, logicalName string) (string, bool) {
	hostsCSV := strings.TrimSpace(os.Getenv("SCYLLA_HOSTS"))
	if hostsCSV == "" {
		return "", false
	}
	hosts := strings.Split(hostsCSV, ",")
	for i := range hosts {
		hosts[i] = strings.TrimSpace(hosts[i])
	}
	keyspace := strings.TrimSpace(os.Getenv("SCYLLA_KEYSPACE"))
	if keyspace == "" {
		t.Skip("SCYLLA_HOSTS set but SCYLLA_KEYSPACE missing — skipping Scylla tests")
	}
	table := getenv("SCYLLA_TABLE", "kv")

	opts := map[string]any{
		"hosts":                        hosts,
		"keyspace":                     keyspace,
		"table":                        table,
		"replication_factor":           1,
		"connect_timeout_ms":           5000,
		"timeout_ms":                   5000,
		"disable_initial_host_lookup":  true,
		"path":                         "",
		"name":                         logicalName,
	}
	b, _ := json.Marshal(opts)
	return string(b), true
}

// ---------- helpers ----------

func openWithRetry(t *testing.T, c *Storage_Client, id, options string) error {
	t.Helper()
	var last error
	for i := 0; i < 10; i++ {
		if err := c.OpenConnection(id, options); err != nil {
			last = err
			// Retry on the specific “no connection found” transitional state.
			if strings.Contains(strings.ToLower(err.Error()), "no connection found") {
				time.Sleep(150 * time.Millisecond)
				continue
			}
			return err
		}
		return nil
	}
	return last
}

func kvRoundTrip(t *testing.T, c *Storage_Client, id string) {
	t.Helper()
	key := "alpha"
	val := []byte("bravo")

	if err := c.SetItem(id, key, val); err != nil {
		t.Fatalf("SetItem(%s) failed: %v", id, err)
	}
	got, err := c.GetItem(id, key)
	if err != nil {
		t.Fatalf("GetItem(%s) failed: %v", id, err)
	}
	if string(got) != string(val) {
		t.Fatalf("GetItem mismatch: got=%q want=%q", string(got), string(val))
	}
	exists, err := c.Exists(id, key)
	if err != nil {
		t.Fatalf("Exists(%s) failed: %v", id, err)
	}
	if !exists {
		t.Fatalf("Exists returned false for existing key")
	}
	if err := c.RemoveItem(id, key); err != nil {
		t.Fatalf("RemoveItem(%s) failed: %v", id, err)
	}
	exists, err = c.Exists(id, key)
	if err != nil {
		t.Fatalf("Exists(after remove) failed: %v", err)
	}
	if exists {
		t.Fatalf("Exists returned true after RemoveItem")
	}
}

// ---------- the suite (sequential) ----------

func TestStoreImplementations(t *testing.T) {
	client, err := NewStorageService_Client(storageAddr, serviceID)
	if err != nil {
		t.Fatalf("NewStorageService_Client(%s, %s) failed: %v", storageAddr, serviceID, err)
	}
	// Give each RPC a sensible deadline.
	client.SetTimeout(8 * time.Second)
	defer client.Close()

	type tc struct {
		name    string
		typ     storagepb.StoreType
		options string
	}

	// Prepare file-based roots isolated per backend
	levelRoot := filepath.Join(t.TempDir(), "leveldb")
	badgerRoot := filepath.Join(t.TempDir(), "badger")

	var cases []tc

	// LEVEL_DB
	cases = append(cases, tc{
		name:    "leveldb",
		typ:     storagepb.StoreType_LEVEL_DB,
		options: buildOptionsFileBased(t, levelRoot, "ts_leveldb"),
	})

	// BIG_CACHE (in-memory)
	cases = append(cases, tc{
		name:    "bigcache",
		typ:     storagepb.StoreType_BIG_CACHE,
		options: buildOptionsBigCache(),
	})

	// BADGER_DB
	cases = append(cases, tc{
		name:    "badger",
		typ:     storagepb.StoreType_BADGER_DB,
		options: buildOptionsFileBased(t, badgerRoot, "ts_badger"),
	})

	// Optional: SCYLLA_DB (only if SCYLLA_HOSTS provided)
	if scyllaOpts, ok := buildOptionsScylla(t, "ts_scylla"); ok {
		cases = append(cases, tc{
			name:    "scylla",
			typ:     storagepb.StoreType_SCYLLA_DB,
			options: scyllaOpts,
		})
	}

	for _, cse := range cases {
		t.Run(strings.ToUpper(cse.name), func(t *testing.T) {
			id := "conn_" + cse.name

			// Create
			if err := client.CreateConnectionWithType(id, "storage_test_"+cse.name, cse.typ); err != nil {
				t.Fatalf("CreateConnection(%s) type=%s failed: %v", id, cse.typ.String(), err)
			}
			// Clean up the connection record regardless of later failures.
			defer func() { _ = client.DeleteConnection(id) }()

			// Open (with retry if server isn’t ready yet for that id)
			if err := openWithRetry(t, client, id, cse.options); err != nil {
				t.Fatalf("OpenConnection(%s) failed with options=%s: %v", id, cse.options, err)
			}

			// CRUD roundtrip
			kvRoundTrip(t, client, id)

			// Clear
			if err := client.Clear(id); err != nil {
				t.Fatalf("Clear(%s) failed: %v", id, err)
			}


			// Ensure files/handles are released for file-backed stores
			if err := client.CloseConnection(id); err != nil {
				// Some stores implicitly close on Drop; ignore secondary errors.
				_ = err
			}
		})
	}
}

// A small error-behavior test that should not deadlock or hang.
func TestErrorPaths_AllStores(t *testing.T) {
	client, err := NewStorageService_Client(storageAddr, serviceID)
	if err != nil {
		t.Fatalf("NewStorageService_Client failed: %v", err)
	}
	client.SetTimeout(5 * time.Second)
	defer client.Close()

	// Unknown connection id
	const missing = "does-not-exist"
	if err := client.SetItem(missing, "k", []byte("v")); err == nil {
		t.Fatalf("SetItem on unknown id should fail")
	}
	if _, err := client.GetItem(missing, "k"); err == nil {
		t.Fatalf("GetItem on unknown id should fail")
	}
	_ = client.Clear(missing)
	_ = client.Drop(missing)

	// Bad options should fail quickly on Open
	badOpts := `{"this":"is","not":"expected"}`
	for _, typ := range []storagepb.StoreType{
		storagepb.StoreType_LEVEL_DB,
		storagepb.StoreType_BIG_CACHE,
		storagepb.StoreType_BADGER_DB,
	} {
		id := fmt.Sprintf("bad_%s", strings.ToLower(typ.String()))
		if err := client.CreateConnectionWithType(id, "badcase", typ); err != nil {
			t.Fatalf("CreateConnectionWithType failed: %v", err)
		}
		_ = client.OpenConnection(id, badOpts) // should return an error; don’t hang
		_ = client.Drop(id)
		_ = client.DeleteConnection(id)
	}
}
