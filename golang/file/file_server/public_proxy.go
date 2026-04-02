// public_proxy.go — gRPC proxy for remote-node public directory access.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/file/filepb"
	"github.com/globulario/services/golang/security"
)

// clusterDirCache holds the in-memory snapshot of the cluster-wide public dir registry.
type clusterDirCache struct {
	mu      sync.RWMutex
	entries []config.PublicDirEntry
}

func (c *clusterDirCache) set(entries []config.PublicDirEntry) {
	c.mu.Lock()
	c.entries = entries
	c.mu.Unlock()
}

func (c *clusterDirCache) get() []config.PublicDirEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]config.PublicDirEntry, len(c.entries))
	copy(out, c.entries)
	return out
}

// findRemoteDir returns the cluster entry if path belongs to a remote node's
// public dir. Returns nil if the path is local or not in the registry.
func (c *clusterDirCache) findRemoteDir(path, localNodeID string) *config.PublicDirEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for i := range c.entries {
		e := &c.entries[i]
		if e.NodeID == localNodeID {
			continue // skip our own entries
		}
		if e.Type == "minio" {
			continue // MinIO dirs are directly accessible
		}
		if strings.HasPrefix(path+"/", e.Path+"/") || path == e.Path {
			return e
		}
	}
	return nil
}

// startClusterDirWatcher loads the initial cluster dir set and watches for changes.
func (srv *server) startClusterDirWatcher(ctx context.Context) {
	// Initial load
	entries, err := config.GetAllPublicDirs()
	if err != nil {
		slog.Warn("failed to load initial cluster public dirs", "err", err)
	} else {
		srv.clusterDirs.set(entries)
		slog.Info("loaded cluster public dirs", "count", len(entries))
	}

	// Background watcher
	go func() {
		if err := config.WatchPublicDirs(ctx, func(entries []config.PublicDirEntry) {
			srv.clusterDirs.set(entries)
			slog.Debug("cluster public dirs updated", "count", len(entries))
		}); err != nil {
			slog.Warn("cluster public dirs watcher stopped", "err", err)
		}
	}()
}

// proxyReadFile proxies a ReadFile call to a remote node's file service.
func (srv *server) proxyReadFile(path, nodeAddress string, stream filepb.FileService_ReadFileServer) error {
	client, err := srv.GetFileClient(nodeAddress)
	if err != nil {
		return fmt.Errorf("connect to remote node %s: %w", nodeAddress, err)
	}

	token, _ := security.GetLocalToken(srv.Mac)
	data, err := client.ReadFile(token, path)
	if err != nil {
		return fmt.Errorf("remote ReadFile %s via %s: %w", path, nodeAddress, err)
	}

	// Stream data back in chunks
	const chunkSize = 5 * 1024
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		if err := stream.Send(&filepb.ReadFileResponse{Data: data[i:end]}); err != nil {
			return err
		}
	}
	return nil
}

// proxyReadDir proxies a ReadDir call to a remote node's file service.
func (srv *server) proxyReadDir(path, nodeAddress string, recursive bool, stream filepb.FileService_ReadDirServer) error {
	client, err := srv.GetFileClient(nodeAddress)
	if err != nil {
		return fmt.Errorf("connect to remote node %s: %w", nodeAddress, err)
	}

	infos, err := client.ReadDir(path, recursive, int32(-1), int32(-1))
	if err != nil {
		return fmt.Errorf("remote ReadDir %s via %s: %w", path, nodeAddress, err)
	}

	for _, info := range infos {
		if err := stream.Send(&filepb.ReadDirResponse{Info: info}); err != nil {
			return err
		}
	}
	return nil
}

// proxyGetFileInfo proxies a GetFileInfo call to a remote node's file service.
func (srv *server) proxyGetFileInfo(_ context.Context, path, nodeAddress string) (*filepb.GetFileInfoResponse, error) {
	client, err := srv.GetFileClient(nodeAddress)
	if err != nil {
		return nil, fmt.Errorf("connect to remote node %s: %w", nodeAddress, err)
	}

	token, _ := security.GetLocalToken(srv.Mac)
	info, err := client.GetFileInfo(token, path, false, int32(-1), int32(-1))
	if err != nil {
		return nil, fmt.Errorf("remote GetFileInfo %s via %s: %w", path, nodeAddress, err)
	}
	return &filepb.GetFileInfoResponse{Info: info}, nil
}
