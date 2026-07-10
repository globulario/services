package substrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// SchemaVersion of the dump file format.
	SchemaVersion = 1

	// Keyspace captured by a dump. Everything the cluster coordinates lives
	// under this prefix; the classification table governs what a restore
	// does with each key.
	Keyspace = "/globular/"

	// DefaultDumpDir is where node-local dumps are written. Dumps contain
	// operator secrets (classified RESTORE_AUTHORITATIVE), so files are 0600.
	DefaultDumpDir = "/var/lib/globular/backup/etcd"

	// ClusterIDKey mirrors config.ClusterMembershipIDKey without importing
	// the config package (this package must stay dependency-light).
	ClusterIDKey = "/globular/system/cluster/id"

	rangePageLimit = 1000
)

// Entry is one dumped key-value pair with its revision metadata.
type Entry struct {
	Key            string `json:"k"`
	Value          []byte `json:"v"`
	CreateRevision int64  `json:"create_rev"`
	ModRevision    int64  `json:"mod_rev"`
	Version        int64  `json:"version"`
	Lease          bool   `json:"lease,omitempty"`
}

// Manifest describes a dump. DesiredEpoch orders dumps for restore selection:
// until an owner-authorized epoch counter exists, it is the max ModRevision
// across keys classified RESTORE_AUTHORITATIVE or RESTORE_AS_UNVERIFIED —
// i.e. the desired-state surface. Heartbeats, leases, observations, and other
// DISCARD/REBUILD-class churn deliberately do NOT advance it.
type Manifest struct {
	SchemaVersion      int    `json:"schema_version"`
	ClusterUID         string `json:"cluster_uid"`
	DesiredEpoch       int64  `json:"desired_epoch"`
	CreatedAt          string `json:"created_at"`
	CreatedByNode      string `json:"created_by_node"`
	SourceEtcdRevision int64  `json:"source_etcd_revision"`
	Keyspace           string `json:"keyspace"`
	KeyCount           int    `json:"key_count"`
	PayloadSHA256      string `json:"payload_sha256"`
	// SerializableRead records that the dump was taken with local
	// (non-linearizable) reads — true for dumps taken from a quorum-less
	// member. The data is that member's last applied state.
	SerializableRead bool `json:"serializable_read"`
}

// Dump is the on-disk file: manifest plus full keyspace contents.
type Dump struct {
	Manifest Manifest `json:"manifest"`
	Entries  []Entry  `json:"entries"`
}

// TakeDump reads the full /globular keyspace at one consistent revision.
// The first page pins the revision; later pages read at that revision, so a
// dump is a point-in-time snapshot even while writers are active.
func TakeDump(ctx context.Context, kv KV, serializable bool) (*Dump, error) {
	var entries []Entry
	var rev int64
	start, end := Keyspace, rangeEnd(Keyspace)

	for {
		kvs, headRev, more, err := kv.Range(ctx, start, end, rev, rangePageLimit)
		if err != nil {
			return nil, fmt.Errorf("range %q: %w", start, err)
		}
		if rev == 0 {
			rev = headRev
		}
		for _, k := range kvs {
			entries = append(entries, Entry{
				Key:            string(k.Key),
				Value:          append([]byte(nil), k.Value...),
				CreateRevision: k.CreateRevision,
				ModRevision:    k.ModRevision,
				Version:        k.Version,
				Lease:          k.Lease != 0,
			})
		}
		if !more || len(kvs) == 0 {
			break
		}
		// Resume strictly after the last returned key.
		start = string(kvs[len(kvs)-1].Key) + "\x00"
	}

	clusterUID := ""
	epoch := int64(0)
	for i := range entries {
		if entries[i].Key == ClusterIDKey {
			clusterUID = strings.TrimSpace(string(entries[i].Value))
		}
		c := Classify(entries[i].Key)
		if c.Policy == RestoreAuthoritative || c.Policy == RestoreAsUnverified {
			if entries[i].ModRevision > epoch {
				epoch = entries[i].ModRevision
			}
		}
	}

	hostname, _ := os.Hostname()
	d := &Dump{
		Manifest: Manifest{
			SchemaVersion:      SchemaVersion,
			ClusterUID:         clusterUID,
			DesiredEpoch:       epoch,
			CreatedAt:          time.Now().UTC().Format(time.RFC3339),
			CreatedByNode:      hostname,
			SourceEtcdRevision: rev,
			Keyspace:           Keyspace,
			KeyCount:           len(entries),
			SerializableRead:   serializable,
		},
		Entries: entries,
	}
	sum, err := payloadSHA256(entries)
	if err != nil {
		return nil, err
	}
	d.Manifest.PayloadSHA256 = sum
	return d, nil
}

func payloadSHA256(entries []Entry) (string, error) {
	payload, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("marshal entries: %w", err)
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

// WriteFile persists the dump atomically (tmp + rename) with 0600 permissions
// — dumps contain authoritative secrets. It returns the final path.
func (d *Dump) WriteFile(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	created, err := time.Parse(time.RFC3339, d.Manifest.CreatedAt)
	if err != nil {
		created = time.Now().UTC()
	}
	name := fmt.Sprintf("globular-dump-%s-rev%d.json",
		created.UTC().Format("20060102T150405Z"), d.Manifest.SourceEtcdRevision)
	final := filepath.Join(dir, name)

	data, err := json.MarshalIndent(d, "", " ")
	if err != nil {
		return "", fmt.Errorf("marshal dump: %w", err)
	}
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, final); err != nil {
		return "", fmt.Errorf("rename: %w", err)
	}
	return final, nil
}

// ReadDumpFile loads and integrity-checks a dump file: schema version must be
// supported and the payload checksum must match the manifest. A dump that
// fails either check is unusable evidence and is rejected outright.
func ReadDumpFile(path string) (*Dump, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var d Dump
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if d.Manifest.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("%s: unsupported schema_version %d (supported: %d)",
			path, d.Manifest.SchemaVersion, SchemaVersion)
	}
	sum, err := payloadSHA256(d.Entries)
	if err != nil {
		return nil, err
	}
	if sum != d.Manifest.PayloadSHA256 {
		return nil, fmt.Errorf("%s: payload checksum mismatch (manifest %s, computed %s) — dump is corrupt or tampered",
			path, d.Manifest.PayloadSHA256, sum)
	}
	return &d, nil
}

// SelectLatestDump picks the restore candidate from a directory: matching
// cluster UID (when known), supported schema, valid checksum, then highest
// DesiredEpoch with SourceEtcdRevision as the tie-breaker. Timestamps are
// deliberately NOT the ordering key — "latest mtime wins" restores whichever
// node wrote last, not whichever dump carries the newest desired state.
func SelectLatestDump(dir, clusterUID string) (string, *Dump, error) {
	names, err := filepath.Glob(filepath.Join(dir, "globular-dump-*.json"))
	if err != nil {
		return "", nil, err
	}
	sort.Strings(names)

	var bestPath string
	var best *Dump
	var rejected []string
	for _, path := range names {
		d, err := ReadDumpFile(path)
		if err != nil {
			rejected = append(rejected, fmt.Sprintf("%s: %v", filepath.Base(path), err))
			continue
		}
		if clusterUID != "" && d.Manifest.ClusterUID != "" && d.Manifest.ClusterUID != clusterUID {
			rejected = append(rejected, fmt.Sprintf("%s: cluster_uid %s != %s", filepath.Base(path), d.Manifest.ClusterUID, clusterUID))
			continue
		}
		if best == nil ||
			d.Manifest.DesiredEpoch > best.Manifest.DesiredEpoch ||
			(d.Manifest.DesiredEpoch == best.Manifest.DesiredEpoch &&
				d.Manifest.SourceEtcdRevision > best.Manifest.SourceEtcdRevision) {
			best, bestPath = d, path
		}
	}
	if best == nil {
		if len(rejected) > 0 {
			return "", nil, fmt.Errorf("no usable dump in %s; rejected: %s", dir, strings.Join(rejected, "; "))
		}
		return "", nil, fmt.Errorf("no dumps found in %s", dir)
	}
	return bestPath, best, nil
}
