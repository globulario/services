package main

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/file/filepb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IndexFile re-indexes files (PDFs, text files) for full-text search.
// When the path points to a single file it indexes that file and streams one response.
// When the path points to a directory it walks the directory (optionally recursively)
// and streams a progress response for each file encountered.
//
// Uses the storage abstraction (storageForPath) so it works with both local
// filesystem and MinIO-backed paths.
func (srv *server) IndexFile(rqst *filepb.IndexFileRequest, strm filepb.FileService_IndexFileServer) error {
	path := srv.formatPath(rqst.GetPath())
	if path == "" {
		return status.Errorf(codes.InvalidArgument, "path is required")
	}
	force := rqst.GetForce()
	ctx := strm.Context()
	store := srv.storageForPath(path)

	info, err := store.Stat(ctx, path)
	if err != nil {
		return status.Errorf(codes.NotFound, "path not found: %s", path)
	}

	// Single file
	if !info.IsDir() {
		err := srv.indexFile(path, force)
		st := "indexed"
		msg := "OK"
		if err != nil {
			st = "error"
			msg = err.Error()
		}
		return strm.Send(&filepb.IndexFileResponse{
			Path: path, Status: st, Message: msg, Indexed: 1, Total: 1,
		})
	}

	// Directory walk
	var indexed, total int32
	recursive := rqst.GetRecursive()

	var walkDir func(ctx context.Context, dirPath string)
	walkDir = func(ctx context.Context, dirPath string) {
		entries, err := store.ReadDir(ctx, dirPath)
		if err != nil {
			return
		}
		for _, e := range entries {
			entryPath := filepath.Join(dirPath, e.Name())

			// Skip .hidden directories
			if strings.Contains(entryPath, "/.hidden/") || e.Name() == ".hidden" {
				continue
			}

			if e.IsDir() {
				if recursive {
					walkDir(ctx, entryPath)
				}
				continue
			}

			total++
			err := srv.indexFile(entryPath, force)
			st := "indexed"
			msg := "OK"
			if err != nil {
				if strings.Contains(err.Error(), "no indexer") {
					st = "skipped"
				} else {
					st = "error"
				}
				msg = err.Error()
			} else {
				indexed++
			}
			_ = strm.Send(&filepb.IndexFileResponse{
				Path: entryPath, Status: st, Message: msg,
				Indexed: indexed, Total: total,
			})
		}
	}

	walkDir(ctx, path)
	return nil
}

// FindIndexes discovers all __index_db__ paths under a directory.
// When recursive is true, it walks subdirectories looking for .hidden/*/__index_db__/.
// This replaces many client-side readDir round-trips with a single server-side walk.
func (srv *server) FindIndexes(ctx context.Context, rqst *filepb.FindIndexesRequest) (*filepb.FindIndexesResponse, error) {
	path := srv.formatPath(rqst.GetPath())
	if path == "" {
		return nil, status.Errorf(codes.InvalidArgument, "path is required")
	}

	store := srv.storageForPath(path)
	var indexPaths []string

	// findInHidden looks for __index_db__ entries inside a .hidden directory.
	findInHidden := func(hiddenDir string) {
		entries, err := store.ReadDir(ctx, hiddenDir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			indexDir := filepath.Join(hiddenDir, e.Name(), "__index_db__")
			if store.Exists(ctx, indexDir) {
				indexPaths = append(indexPaths, indexDir)
			}
		}
	}

	// Check .hidden/ in the root path
	findInHidden(filepath.Join(path, ".hidden"))

	// If recursive, walk subdirectories
	if rqst.GetRecursive() {
		var walkSubdirs func(dirPath string)
		walkSubdirs = func(dirPath string) {
			entries, err := store.ReadDir(ctx, dirPath)
			if err != nil {
				return
			}
			for _, e := range entries {
				if !e.IsDir() || e.Name() == ".hidden" {
					continue
				}
				subPath := filepath.Join(dirPath, e.Name())
				// Check this subdir's .hidden/
				findInHidden(filepath.Join(subPath, ".hidden"))
				// Recurse deeper
				walkSubdirs(subPath)
			}
		}
		walkSubdirs(path)
	}

	return &filepb.FindIndexesResponse{Paths: indexPaths}, nil
}
