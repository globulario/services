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
	"github.com/globulario/services/golang/testutil"
)

// ---------- CONSTANTS ----------
// These can be overridden via environment variables.

const (
	// Service ID (as registered in Globular)
	serviceID = "storage.StorageService"

	// Scylla test defaults - can be overridden via environment
	defaultScyllaKeyspace = "storage_test"
	defaultScyllaTable    = "kv"
)

// getStorageAddr returns the storage service address from environment or default.
func getStorageAddr() string {
	return testutil.GetAddress()
}

// getScyllaConfig returns Scylla configuration from environment variables.
func getScyllaConfig() (hosts, keyspace, table string, useTLS bool, caFile, certFile, keyFile string, insecure bool) {
	addr := getStorageAddr()
	hosts = testutil.GetEnvOrDefault("SCYLLA_HOSTS", addr+":9142")
	keyspace = testutil.GetEnvOrDefault("SCYLLA_KEYSPACE", defaultScyllaKeyspace)
	table = testutil.GetEnvOrDefault("SCYLLA_TABLE", defaultScyllaTable)
	useTLS = testutil.GetEnvOrDefault("SCYLLA_USE_TLS", "true") == "true"
	caFile, certFile, _ = testutil.GetTLSPaths()
	keyFile = os.Getenv(testutil.EnvTLSKeyPath)
	insecure = testutil.GetEnvOrDefault("SCYLLA_INSECURE", "false") == "true"
	return
}

// ---------- option builders ----------

func buildOptionsFileBased(t *testing.T, baseDir, name string) string {
	t.Helper()
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", baseDir, err)
	}
	opts := map[string]any{"path": baseDir, "name": name}
	b, _ := json.Marshal(opts)
	return string(b)
}

func buildOptionsBigCache() string {
	b, _ := json.Marshal(map[string]any{"lifeWindowSec": 30})
	return string(b)
}

func buildOptionsScylla(t *testing.T, logicalName string) (string, bool) {
	scyllaHosts, scyllaKeyspace, scyllaTable, scyllaUseTLS, scyllaCAFile, scyllaCertFile, scyllaKeyFile, scyllaInsecure := getScyllaConfig()
	if strings.TrimSpace(scyllaHosts) == "" || strings.TrimSpace(scyllaKeyspace) == "" {
		return "", false
	}
	hosts := strings.Split(scyllaHosts, ",")
	for i := range hosts {
		hosts[i] = strings.TrimSpace(hosts[i])
	}
	opts := map[string]any{
		"hosts":                       hosts,
		"keyspace":                    scyllaKeyspace,
		"table":                       scyllaTable,
		"replication_factor":          1,
		"connect_timeout_ms":          5000,
		"timeout_ms":                  5000,
		"disable_initial_host_lookup": true,
		"tls":                  scyllaUseTLS,
		"ca_file":              scyllaCAFile,
		"cert_file":            scyllaCertFile,
		"key_file":             scyllaKeyFile,
		"insecure_skip_verify": scyllaInsecure,
		"name":                 logicalName,
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
		t.Fatalf("SetItem(%s): %v", id, err)
	}
	got, err := c.GetItem(id, key)
	if err != nil {
		t.Fatalf("GetItem(%s): %v", id, err)
	}
	if string(got) != string(val) {
		t.Fatalf("GetItem mismatch: got=%q want=%q", string(got), string(val))
	}
	if err := c.RemoveItem(id, key); err != nil {
		t.Fatalf("RemoveItem(%s): %v", id, err)
	}
	if ex, _ := c.Exists(id, key); ex {
		t.Fatalf("Exists true after RemoveItem")
	}
}

// ---------- the suite ----------

func TestStoreImplementations(t *testing.T) {
	testutil.SkipIfNoExternalServices(t)

	storageAddr := getStorageAddr()
	client, err := NewStorageService_Client(storageAddr, serviceID)
	if err != nil {
		t.Fatalf("NewStorageService_Client: %v", err)
	}
	client.SetTimeout(10 * time.Second)
	defer client.Close()

	// Put test data under the configured Globular config dir to keep things tidy.
	base := filepath.Join(os.TempDir(), "testdata", "storage", t.Name())
	levelRoot := filepath.Join(base, "leveldb")
	badgerRoot := filepath.Join(base, "badger")

	type tc struct {
		name    string
		typ     storagepb.StoreType
		options string
	}
	cases := []tc{
		{
			name:    "leveldb",
			typ:     storagepb.StoreType_LEVEL_DB,
			options: buildOptionsFileBased(t, levelRoot, "ts_leveldb"),
		},
		{
			name:    "bigcache",
			typ:     storagepb.StoreType_BIG_CACHE,
			options: buildOptionsBigCache(),
		},
		{
			name:    "badger",
			typ:     storagepb.StoreType_BADGER_DB,
			options: buildOptionsFileBased(t, badgerRoot, "ts_badger"),
		},
	}

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

			if err := client.CreateConnectionWithType(id, "storage_test_"+cse.name, cse.typ); err != nil {
				t.Fatalf("CreateConnectionWithType: %v", err)
			}
			
			t.Cleanup(func() { _ = client.DeleteConnection(id) })

			if err := openWithRetry(t, client, id, cse.options); err != nil {
				t.Fatalf("OpenConnection(%s): %v", cse.name, err)
			}

			kvRoundTrip(t, client, id)

			// Sanity ops & release handles
			_ = client.Clear(id)
			_ = client.CloseConnection(id)
			_ = client.Drop(id)

			// Reopen to verify files/locks are freed (esp. Badger)
			if err := openWithRetry(t, client, id, cse.options); err != nil {
				t.Fatalf("Reopen after Drop(%s): %v", cse.name, err)
			}
			_ = client.CloseConnection(id)
		})
	}
}

// Quick error-surface check: should fail fast and not hang.
func TestErrorPaths_AllStores(t *testing.T) {
	testutil.SkipIfNoExternalServices(t)

	storageAddr := getStorageAddr()
	client, err := NewStorageService_Client(storageAddr, serviceID)
	if err != nil {
		t.Fatalf("NewStorageService_Client: %v", err)
	}
	client.SetTimeout(5 * time.Second)
	defer client.Close()

	const missing = "does-not-exist"
	if err := client.SetItem(missing, "k", []byte("v")); err == nil {
		t.Fatalf("SetItem on unknown id should fail")
	}
	if _, err := client.GetItem(missing, "k"); err == nil {
		t.Fatalf("GetItem on unknown id should fail")
	}
	_ = client.Clear(missing)
	_ = client.Drop(missing)

	badOpts := `{"this":"is","not":"expected"}`
	for _, typ := range []storagepb.StoreType{
		storagepb.StoreType_LEVEL_DB,
		storagepb.StoreType_BIG_CACHE,
		storagepb.StoreType_BADGER_DB,
	} {
		id := fmt.Sprintf("bad_%s", strings.ToLower(typ.String()))
		if err := client.CreateConnectionWithType(id, "badcase", typ); err != nil {
			t.Fatalf("CreateConnectionWithType: %v", err)
		}
		_ = client.OpenConnection(id, badOpts) // return error; don’t hang
		_ = client.Drop(id)
		_ = client.DeleteConnection(id)
	}
}
