package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	"github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/minio/minio-go/v7"
)

// runWebrootSync mirrors the MinIO webroot bucket to the local filesystem.
// This provides a resilience fallback: when MinIO is unreachable, the gateway
// serves static content from the local copy in /var/lib/globular/webroot/.
//
// The sync is incremental: only files that differ in size or modification time
// are downloaded. Files deleted from MinIO are removed locally.
func (srv *NodeAgentServer) runWebrootSync(ctx context.Context, req *node_agentpb.RunWorkflowRequest) (*node_agentpb.RunWorkflowResponse, error) {
	start := time.Now()

	// Load MinIO config from the objectstore contract.
	minioCfg, _, err := actions.LoadMinioConfigPublic(actions.ResolveContractPathPublic(), false)
	if err != nil {
		return probeFail(start, fmt.Sprintf("load minio config: %v", err)), nil
	}

	client, err := actions.BuildMinioClientPublic(minioCfg)
	if err != nil {
		return probeFail(start, fmt.Sprintf("connect minio: %v", err)), nil
	}

	// Derive bucket and prefix from the layout.
	domain, _ := config.GetDomain()
	layout, err := actions.DeriveMinioLayoutPublic(minioCfg, domain)
	if err != nil {
		return probeFail(start, fmt.Sprintf("derive layout: %v", err)), nil
	}

	bucket := layout.WebrootBucket
	prefix := layout.WebrootPrefix
	if bucket == "" {
		return probeFail(start, "webroot bucket not configured"), nil
	}

	localRoot := config.GetWebRootDir()
	if err := os.MkdirAll(localRoot, 0o755); err != nil {
		return probeFail(start, fmt.Sprintf("create webroot dir: %v", err)), nil
	}

	// Track remote objects for cleanup of deleted files.
	remoteKeys := make(map[string]struct{})
	var synced, skipped, removed int

	// List and sync objects from MinIO.
	listCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	objectCh := client.ListObjects(listCtx, bucket, minio.ListObjectsOptions{
		Prefix:    prefix + "/",
		Recursive: true,
	})

	for obj := range objectCh {
		if obj.Err != nil {
			return probeFail(start, fmt.Sprintf("list objects: %v", obj.Err)), nil
		}

		// Strip prefix to get relative path.
		relPath := strings.TrimPrefix(obj.Key, prefix+"/")
		if relPath == "" || strings.HasSuffix(relPath, "/") {
			continue
		}

		localPath := filepath.Join(localRoot, filepath.FromSlash(relPath))
		remoteKeys[relPath] = struct{}{}

		// Check if local file is up-to-date.
		if fi, err := os.Stat(localPath); err == nil {
			if fi.Size() == obj.Size && !obj.LastModified.After(fi.ModTime().Add(1*time.Second)) {
				skipped++
				continue
			}
		}

		// Download the object.
		if err := downloadMinioObject(ctx, client, bucket, obj.Key, localPath); err != nil {
			log.Printf("webroot-sync: download %s failed: %v", relPath, err)
			continue
		}
		synced++
	}

	// Remove local files that no longer exist in MinIO.
	_ = filepath.Walk(localRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(localRoot, path)
		rel = filepath.ToSlash(rel)
		if _, exists := remoteKeys[rel]; !exists {
			if os.Remove(path) == nil {
				removed++
			}
		}
		return nil
	})

	// Clean up empty directories.
	cleanEmptyDirs(localRoot)

	msg := fmt.Sprintf("synced=%d skipped=%d removed=%d (%.1fs)", synced, skipped, removed, time.Since(start).Seconds())
	log.Printf("webroot-sync: %s", msg)

	return &node_agentpb.RunWorkflowResponse{
		Status:         "SUCCEEDED",
		StepsTotal:     int32(synced + skipped),
		StepsSucceeded: int32(synced + skipped),
		DurationMs:     time.Since(start).Milliseconds(),
	}, nil
}

func downloadMinioObject(ctx context.Context, client *minio.Client, bucket, key, localPath string) error {
	dlCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	obj, err := client.GetObject(dlCtx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return err
	}
	defer obj.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}

	tmp := localPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, obj); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()

	return os.Rename(tmp, localPath)
}

func cleanEmptyDirs(root string) {
	// Walk bottom-up by collecting dirs first.
	var dirs []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() && path != root {
			dirs = append(dirs, path)
		}
		return nil
	})
	// Remove from deepest to shallowest.
	for i := len(dirs) - 1; i >= 0; i-- {
		os.Remove(dirs[i]) // only succeeds if empty
	}
}
