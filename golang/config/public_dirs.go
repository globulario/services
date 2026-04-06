package config

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// PublicDirEntry represents a single public directory registered in the cluster.
//go:schemalint:ignore — schema owned by marker type in schema_annotations.go
type PublicDirEntry struct {
	Path        string `json:"path"`
	Type        string `json:"type"` // "local", "minio", "external"
	NodeID      string `json:"node_id"`
	NodeAddress string `json:"node_address"`
	LocalPath   string `json:"local_path"`
}

const publicDirsPrefix = "/globular/cluster/public-dirs/"

// pathHash returns a short hex hash of the path for use as an etcd key suffix.
func pathHash(path string) string {
	h := sha256.Sum256([]byte(path))
	return fmt.Sprintf("%x", h[:8])
}

// PutPublicDir writes a single public dir entry to the cluster registry.
func PutPublicDir(entry PublicDirEntry) error {
	cli, err := GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}
	key := publicDirsPrefix + entry.NodeID + "/" + pathHash(entry.Path)
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal public dir entry: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = cli.Put(ctx, key, string(data))
	return err
}

// DeletePublicDir removes a single public dir entry from the cluster registry.
func DeletePublicDir(nodeID, path string) error {
	cli, err := GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}
	key := publicDirsPrefix + nodeID + "/" + pathHash(path)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = cli.Delete(ctx, key)
	return err
}

// GetAllPublicDirs reads all public dir entries from all nodes in the cluster.
func GetAllPublicDirs() ([]PublicDirEntry, error) {
	cli, err := GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("etcd client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := cli.Get(ctx, publicDirsPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("etcd get: %w", err)
	}
	var entries []PublicDirEntry
	for _, kv := range resp.Kvs {
		var e PublicDirEntry
		if err := json.Unmarshal(kv.Value, &e); err != nil {
			continue // skip corrupt entries
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// WatchPublicDirs watches the cluster public-dirs prefix and calls callback
// with the full set of entries whenever a change occurs. Blocks until ctx is done.
func WatchPublicDirs(ctx context.Context, callback func([]PublicDirEntry)) error {
	cli, err := GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}
	ch := cli.Watch(ctx, publicDirsPrefix, clientv3.WithPrefix())
	for resp := range ch {
		if resp.Err() != nil {
			continue
		}
		entries, err := GetAllPublicDirs()
		if err != nil {
			continue
		}
		callback(entries)
	}
	return nil
}

// PublicDirTypeString converts a string type to a canonical lowercase form.
func PublicDirTypeString(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	switch t {
	case "local", "minio", "external":
		return t
	default:
		return "local"
	}
}
