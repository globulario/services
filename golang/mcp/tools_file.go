package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/globulario/services/golang/config"
	filepb "github.com/globulario/services/golang/file/filepb"
)

// ── File service endpoint ────────────────────────────────────────────────────

func fileEndpoint() string {
	if cfg, err := config.GetServiceConfigurationById("file.FileService"); err == nil {
		if port, ok := cfg["Port"].(float64); ok {
			addr := "localhost"
			if a, ok := cfg["Address"].(string); ok && a != "" {
				addr = a
			}
			return fmt.Sprintf("%s:%d", addr, int(port))
		}
	}
	return "localhost:10103"
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func normalizeFileInfo(info *filepb.FileInfo) map[string]interface{} {
	if info == nil {
		return nil
	}
	return map[string]interface{}{
		"name":        info.GetName(),
		"path":        info.GetPath(),
		"size":        info.GetSize(),
		"size_human":  fmtBytes(uint64(info.GetSize())),
		"mode_octal":  fmt.Sprintf("%04o", info.GetMode()),
		"modified_at": fmtTime(info.GetModeTime()),
		"is_dir":      info.GetIsDir(),
		"mime":        info.GetMime(),
		"checksum":    info.GetChecksum(),
	}
}

func normalizeFileInfoBrief(info *filepb.FileInfo) map[string]interface{} {
	if info == nil {
		return nil
	}
	return map[string]interface{}{
		"name":        info.GetName(),
		"path":        info.GetPath(),
		"size":        info.GetSize(),
		"size_human":  fmtBytes(uint64(info.GetSize())),
		"mode_octal":  fmt.Sprintf("%04o", info.GetMode()),
		"modified_at": fmtTime(info.GetModeTime()),
		"is_dir":      info.GetIsDir(),
		"mime":        info.GetMime(),
	}
}

// collectReadDir collects streaming ReadDir responses up to limit.
func collectReadDir(stream filepb.FileService_ReadDirClient, limit int) ([]map[string]interface{}, bool, error) {
	var entries []map[string]interface{}
	truncated := false
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, false, err
		}
		info := resp.GetInfo()
		if info == nil {
			continue
		}
		entries = append(entries, normalizeFileInfoBrief(info))
		if limit > 0 && len(entries) >= limit {
			truncated = true
			break
		}
	}
	return entries, truncated, nil
}

// getStrSlice extracts a string slice from a map value (accepts []interface{} or []string).
func getStrSlice(args map[string]interface{}, key string) []string {
	if args == nil {
		return nil
	}
	v, ok := args[key]
	if !ok {
		return nil
	}
	switch arr := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return arr
	default:
		return nil
	}
}

// fileClient creates a FileServiceClient from the pool.
func fileClient(ctx context.Context, pool *clientPool) (filepb.FileServiceClient, error) {
	conn, err := pool.get(ctx, fileEndpoint())
	if err != nil {
		return nil, err
	}
	return filepb.NewFileServiceClient(conn), nil
}

// ── Tool registration ────────────────────────────────────────────────────────

func registerFileTools(s *server) {

	// ── 1. file_get_info ────────────────────────────────────────────────────
	s.register(toolDef{
		Name:        "file_get_info",
		Description: "Get detailed information about a file or directory (size, permissions, checksum, MIME type).",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"path": {
					Type:        "string",
					Description: "Absolute path to the file or directory.",
				},
			},
			Required: []string{"path"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		rawPath := getStr(args, "path")
		if rawPath == "" {
			return nil, fmt.Errorf("missing required argument: path")
		}
		path, err := s.cfg.validateFilePath(rawPath)
		if err != nil {
			return nil, err
		}

		client, err := fileClient(ctx, s.clients)
		if err != nil {
			return nil, err
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.GetFileInfo(callCtx, &filepb.GetFileInfoRequest{Path: path})
		if err != nil {
			return nil, fmt.Errorf("GetFileInfo: %w", err)
		}

		info := resp.GetInfo()
		if info == nil {
			return map[string]interface{}{"error": "no file info returned"}, nil
		}

		result := normalizeFileInfo(info)
		result["mode"] = fmt.Sprintf("%04o", info.GetMode())
		return result, nil
	})

	// ── 2. file_get_metadata ────────────────────────────────────────────────
	s.register(toolDef{
		Name:        "file_get_metadata",
		Description: "Get metadata associated with a file (extended attributes, tags, etc.).",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"path": {
					Type:        "string",
					Description: "Absolute path to the file.",
				},
			},
			Required: []string{"path"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		rawPath := getStr(args, "path")
		if rawPath == "" {
			return nil, fmt.Errorf("missing required argument: path")
		}
		path, err := s.cfg.validateFilePath(rawPath)
		if err != nil {
			return nil, err
		}

		client, err := fileClient(ctx, s.clients)
		if err != nil {
			return nil, err
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.GetFileMetadata(callCtx, &filepb.GetFileMetadataRequest{Path: path})
		if err != nil {
			return nil, fmt.Errorf("GetFileMetadata: %w", err)
		}

		metadata := make(map[string]interface{})
		if s := resp.GetResult(); s != nil {
			metadata = s.AsMap()
		}

		return map[string]interface{}{
			"path":     path,
			"metadata": metadata,
		}, nil
	})

	// ── 3. file_list_directory ──────────────────────────────────────────────
	s.register(toolDef{
		Name:        "file_list_directory",
		Description: "List files and subdirectories in a directory (non-recursive).",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"path": {
					Type:        "string",
					Description: "Absolute path to the directory.",
				},
				"limit": {
					Type:        "number",
					Description: "Maximum number of entries to return (default 100).",
				},
			},
			Required: []string{"path"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		rawPath := getStr(args, "path")
		if rawPath == "" {
			return nil, fmt.Errorf("missing required argument: path")
		}
		path, err := s.cfg.validateFilePath(rawPath)
		if err != nil {
			return nil, err
		}
		limit := getInt(args, "limit", 100)

		client, err := fileClient(ctx, s.clients)
		if err != nil {
			return nil, err
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		stream, err := client.ReadDir(callCtx, &filepb.ReadDirRequest{Path: path, Recursive: false})
		if err != nil {
			return nil, fmt.Errorf("ReadDir: %w", err)
		}

		entries, truncated, err := collectReadDir(stream, limit)
		if err != nil {
			return nil, fmt.Errorf("ReadDir stream: %w", err)
		}

		return map[string]interface{}{
			"path":      path,
			"count":     len(entries),
			"entries":   entries,
			"truncated": truncated,
		}, nil
	})

	// ── 4. file_list_directory_recursive ────────────────────────────────────
	s.register(toolDef{
		Name:        "file_list_directory_recursive",
		Description: "Recursively list all files and directories under a path.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"path": {
					Type:        "string",
					Description: "Absolute path to the root directory.",
				},
				"limit": {
					Type:        "number",
					Description: "Maximum number of entries to return (default 500).",
				},
			},
			Required: []string{"path"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		rawPath := getStr(args, "path")
		if rawPath == "" {
			return nil, fmt.Errorf("missing required argument: path")
		}
		path, err := s.cfg.validateFilePath(rawPath)
		if err != nil {
			return nil, err
		}
		limit := getInt(args, "limit", 500)

		client, err := fileClient(ctx, s.clients)
		if err != nil {
			return nil, err
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 30*time.Second)
		defer cancel()

		stream, err := client.ReadDir(callCtx, &filepb.ReadDirRequest{Path: path, Recursive: true})
		if err != nil {
			return nil, fmt.Errorf("ReadDir: %w", err)
		}

		entries, truncated, err := collectReadDir(stream, limit)
		if err != nil {
			return nil, fmt.Errorf("ReadDir stream: %w", err)
		}

		return map[string]interface{}{
			"path":        path,
			"total_count": len(entries),
			"entries":     entries,
			"truncated":   truncated,
		}, nil
	})

	// ── 5. file_read_text_small ─────────────────────────────────────────────
	s.register(toolDef{
		Name:        "file_read_text_small",
		Description: "Read a small text file's content. Only works for UTF-8 text files, not binary.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"path": {
					Type:        "string",
					Description: "Absolute path to the text file.",
				},
				"max_bytes": {
					Type:        "number",
					Description: "Maximum bytes to read (default 32768).",
				},
			},
			Required: []string{"path"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		rawPath := getStr(args, "path")
		if rawPath == "" {
			return nil, fmt.Errorf("missing required argument: path")
		}
		path, err := s.cfg.validateFilePath(rawPath)
		if err != nil {
			return nil, err
		}
		maxBytes := getInt(args, "max_bytes", 32768)

		client, err := fileClient(ctx, s.clients)
		if err != nil {
			return nil, err
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		stream, err := client.ReadFile(callCtx, &filepb.ReadFileRequest{Path: path})
		if err != nil {
			return nil, fmt.Errorf("ReadFile: %w", err)
		}

		var buf bytes.Buffer
		truncated := false
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("ReadFile stream: %w", err)
			}
			buf.Write(resp.GetData())
			if buf.Len() > maxBytes {
				truncated = true
				break
			}
		}

		content := buf.Bytes()
		if maxBytes > 0 && len(content) > maxBytes {
			content = content[:maxBytes]
			truncated = true
		}

		if !utf8.Valid(content) {
			return nil, fmt.Errorf("file appears to be binary, not a valid UTF-8 text file")
		}

		return map[string]interface{}{
			"path":      path,
			"size":      len(content),
			"content":   string(content),
			"truncated": truncated,
		}, nil
	})

	// ── 6. file_get_public_dirs ─────────────────────────────────────────────
	s.register(toolDef{
		Name:        "file_get_public_dirs",
		Description: "List all directories configured as publicly accessible.",
		InputSchema: inputSchema{
			Type:       "object",
			Properties: map[string]propSchema{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		client, err := fileClient(ctx, s.clients)
		if err != nil {
			return nil, err
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.GetPublicDirs(callCtx, &filepb.GetPublicDirsRequest{})
		if err != nil {
			return nil, fmt.Errorf("GetPublicDirs: %w", err)
		}

		dirs := resp.GetDirs()
		if dirs == nil {
			dirs = []string{}
		}

		return map[string]interface{}{
			"count": len(dirs),
			"dirs":  dirs,
		}, nil
	})

	// ── 7. deploy_get_webroot_snapshot ───────────────────────────────────────
	s.register(toolDef{
		Name:        "deploy_get_webroot_snapshot",
		Description: "Get a snapshot of the webroot directory: file counts, sizes, types, and public directory configuration.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"webroot_path": {
					Type:        "string",
					Description: "Path to the webroot directory (default /var/lib/globular/webroot).",
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		webrootPath := getStr(args, "webroot_path")
		if webrootPath == "" {
			webrootPath = "/var/lib/globular/webroot"
		}
		webrootPath, err := s.cfg.validateFilePath(webrootPath)
		if err != nil {
			return nil, err
		}

		client, err := fileClient(ctx, s.clients)
		if err != nil {
			return nil, err
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 30*time.Second)
		defer cancel()

		// Get public dirs.
		publicDirs := []string{}
		if resp, err := client.GetPublicDirs(callCtx, &filepb.GetPublicDirsRequest{}); err == nil {
			if d := resp.GetDirs(); d != nil {
				publicDirs = d
			}
		}

		// Enumerate webroot recursively.
		stream, err := client.ReadDir(callCtx, &filepb.ReadDirRequest{Path: webrootPath, Recursive: true})
		if err != nil {
			return nil, fmt.Errorf("ReadDir(%s): %w", webrootPath, err)
		}

		var totalFiles int
		var totalSize int64
		fileTypes := make(map[string]int)
		topLevelDirs := make(map[string]bool)
		var warnings []string

		limit := 200
		truncated := false
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("stream error: %v", err))
				break
			}
			info := resp.GetInfo()
			if info == nil {
				continue
			}

			totalFiles++
			totalSize += info.GetSize()

			// Track file type by extension.
			if !info.GetIsDir() {
				ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(info.GetName()), "."))
				if ext == "" {
					ext = "other"
				}
				fileTypes[ext]++
			}

			// Track top-level directories relative to webroot.
			rel := strings.TrimPrefix(info.GetPath(), webrootPath)
			rel = strings.TrimPrefix(rel, "/")
			parts := strings.SplitN(rel, "/", 2)
			if len(parts) > 0 && parts[0] != "" && info.GetIsDir() && len(parts) == 1 {
				topLevelDirs[parts[0]] = true
			}

			if totalFiles >= limit {
				truncated = true
				break
			}
		}

		topDirs := make([]string, 0, len(topLevelDirs))
		for d := range topLevelDirs {
			topDirs = append(topDirs, d)
		}

		summary := fmt.Sprintf("Webroot has %d files (%s) across %d applications",
			totalFiles, fmtBytes(uint64(totalSize)), len(topDirs))
		if truncated {
			summary += fmt.Sprintf(" (truncated at %d entries)", limit)
		}

		return map[string]interface{}{
			"webroot_path":   webrootPath,
			"public_dirs":    publicDirs,
			"total_files":    totalFiles,
			"total_size":     fmtBytes(uint64(totalSize)),
			"file_types":     fileTypes,
			"top_level_dirs": topDirs,
			"warnings":       warnings,
			"summary":        summary,
		}, nil
	})

	// ── 8. deploy_check_app_presence ────────────────────────────────────────
	s.register(toolDef{
		Name:        "deploy_check_app_presence",
		Description: "Check if a specific application's files exist at the expected webroot location.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"app_name": {
					Type:        "string",
					Description: "Name of the application to check.",
				},
				"webroot_path": {
					Type:        "string",
					Description: "Path to the webroot directory (default /var/lib/globular/webroot).",
				},
			},
			Required: []string{"app_name"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		appName := getStr(args, "app_name")
		if appName == "" {
			return nil, fmt.Errorf("missing required argument: app_name")
		}
		webrootPath := getStr(args, "webroot_path")
		if webrootPath == "" {
			webrootPath = "/var/lib/globular/webroot"
		}

		appPath := filepath.Join(webrootPath, appName)
		appPath, err := s.cfg.validateFilePath(appPath)
		if err != nil {
			return nil, err
		}

		client, err := fileClient(ctx, s.clients)
		if err != nil {
			return nil, err
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 30*time.Second)
		defer cancel()

		// Check if the app directory exists.
		infoResp, err := client.GetFileInfo(callCtx, &filepb.GetFileInfoRequest{Path: appPath})
		if err != nil {
			return map[string]interface{}{
				"app_name":  appName,
				"path":      appPath,
				"exists":    false,
				"findings":  []string{fmt.Sprintf("Application directory does not exist: %s", appPath)},
				"summary":   fmt.Sprintf("Application '%s' is not present at %s", appName, appPath),
			}, nil
		}

		info := infoResp.GetInfo()
		if info == nil || !info.GetIsDir() {
			return map[string]interface{}{
				"app_name":     appName,
				"path":         appPath,
				"exists":       true,
				"is_directory": false,
				"findings":     []string{"Path exists but is not a directory"},
				"summary":      fmt.Sprintf("'%s' exists but is not a directory", appName),
			}, nil
		}

		// List contents.
		stream, err := client.ReadDir(callCtx, &filepb.ReadDirRequest{Path: appPath, Recursive: false})
		if err != nil {
			return nil, fmt.Errorf("ReadDir(%s): %w", appPath, err)
		}

		var fileCount int
		var totalSize int64
		var topFiles []string
		hasIndexHTML := false
		hasManifest := false
		var findings []string

		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("ReadDir stream: %w", err)
			}
			fi := resp.GetInfo()
			if fi == nil {
				continue
			}
			fileCount++
			totalSize += fi.GetSize()

			name := fi.GetName()
			if len(topFiles) < 20 {
				topFiles = append(topFiles, name)
			}

			switch strings.ToLower(name) {
			case "index.html":
				hasIndexHTML = true
			case "manifest.json":
				hasManifest = true
			}
		}

		if !hasIndexHTML {
			findings = append(findings, "Missing index.html")
		}
		if !hasManifest {
			findings = append(findings, "Missing manifest.json")
		}

		sizeStr := fmtBytes(uint64(totalSize))
		summary := fmt.Sprintf("Application '%s' is present with %d files (%s)", appName, fileCount, sizeStr)

		return map[string]interface{}{
			"app_name":      appName,
			"path":          appPath,
			"exists":        true,
			"is_directory":  true,
			"file_count":    fileCount,
			"total_size":    sizeStr,
			"has_index_html": hasIndexHTML,
			"has_manifest":  hasManifest,
			"top_files":     topFiles,
			"findings":      findings,
			"summary":       summary,
		}, nil
	})

	// ── 9. deploy_compare_expected_vs_actual_files ──────────────────────────
	s.register(toolDef{
		Name:        "deploy_compare_expected_vs_actual_files",
		Description: "Compare a list of expected files against what actually exists in a directory.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"path": {
					Type:        "string",
					Description: "Absolute path to the directory to check.",
				},
				"expected_files": {
					Type:        "array",
					Description: "List of expected file names (e.g. [\"index.html\", \"app.js\", \"style.css\"]).",
				},
			},
			Required: []string{"path", "expected_files"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		rawPath := getStr(args, "path")
		if rawPath == "" {
			return nil, fmt.Errorf("missing required argument: path")
		}
		path, err := s.cfg.validateFilePath(rawPath)
		if err != nil {
			return nil, err
		}
		expectedFiles := getStrSlice(args, "expected_files")
		if len(expectedFiles) == 0 {
			return nil, fmt.Errorf("missing required argument: expected_files")
		}

		client, err := fileClient(ctx, s.clients)
		if err != nil {
			return nil, err
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		// List actual files.
		stream, err := client.ReadDir(callCtx, &filepb.ReadDirRequest{Path: path, Recursive: false})
		if err != nil {
			return nil, fmt.Errorf("ReadDir(%s): %w", path, err)
		}

		actualSet := make(map[string]bool)
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("ReadDir stream: %w", err)
			}
			if fi := resp.GetInfo(); fi != nil {
				actualSet[fi.GetName()] = true
			}
		}

		// Compare.
		expectedSet := make(map[string]bool, len(expectedFiles))
		for _, f := range expectedFiles {
			expectedSet[f] = true
		}

		var matched, missing, extra []string
		for _, f := range expectedFiles {
			if actualSet[f] {
				matched = append(matched, f)
			} else {
				missing = append(missing, f)
			}
		}
		for f := range actualSet {
			if !expectedSet[f] {
				extra = append(extra, f)
			}
		}

		var findings []string
		for _, f := range missing {
			findings = append(findings, fmt.Sprintf("Missing expected file: %s", f))
		}
		if len(extra) > 0 {
			findings = append(findings, fmt.Sprintf("%d unexpected files found", len(extra)))
		}

		summary := fmt.Sprintf("%d/%d expected files present, %d missing, %d extra",
			len(matched), len(expectedFiles), len(missing), len(extra))

		return map[string]interface{}{
			"path":          path,
			"matched":       matched,
			"missing":       missing,
			"extra":         extra,
			"matched_count": len(matched),
			"missing_count": len(missing),
			"extra_count":   len(extra),
			"findings":      findings,
			"summary":       summary,
		}, nil
	})

	// ── 10. deploy_get_public_exposure_status ───────────────────────────────
	s.register(toolDef{
		Name:        "deploy_get_public_exposure_status",
		Description: "Check which directories are publicly exposed and whether they exist with content.",
		InputSchema: inputSchema{
			Type:       "object",
			Properties: map[string]propSchema{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		client, err := fileClient(ctx, s.clients)
		if err != nil {
			return nil, err
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 30*time.Second)
		defer cancel()

		resp, err := client.GetPublicDirs(callCtx, &filepb.GetPublicDirsRequest{})
		if err != nil {
			return nil, fmt.Errorf("GetPublicDirs: %w", err)
		}

		dirs := resp.GetDirs()
		if dirs == nil {
			dirs = []string{}
		}

		var dirInfos []map[string]interface{}
		var findings []string
		existCount := 0
		missingCount := 0

		for _, dir := range dirs {
			entry := map[string]interface{}{
				"path": dir,
			}

			infoResp, err := client.GetFileInfo(callCtx, &filepb.GetFileInfoRequest{Path: dir})
			if err != nil {
				entry["exists"] = false
				entry["warning"] = "directory does not exist"
				findings = append(findings, fmt.Sprintf("Public directory '%s' does not exist", dir))
				missingCount++
				dirInfos = append(dirInfos, entry)
				continue
			}

			info := infoResp.GetInfo()
			if info == nil {
				entry["exists"] = false
				entry["warning"] = "no info returned"
				missingCount++
				dirInfos = append(dirInfos, entry)
				continue
			}

			entry["exists"] = true
			existCount++

			// Count files in the directory.
			stream, err := client.ReadDir(callCtx, &filepb.ReadDirRequest{Path: dir, Recursive: true})
			if err == nil {
				var fileCount int
				var totalSize int64
				for {
					r, err := stream.Recv()
					if err == io.EOF {
						break
					}
					if err != nil {
						break
					}
					if fi := r.GetInfo(); fi != nil {
						fileCount++
						totalSize += fi.GetSize()
					}
					if fileCount >= 10000 {
						break
					}
				}
				entry["file_count"] = fileCount
				entry["total_size"] = fmtBytes(uint64(totalSize))

				if fileCount == 0 {
					entry["warning"] = "directory is empty"
					findings = append(findings, fmt.Sprintf("Public directory '%s' is empty", dir))
				}
				if totalSize > 500*1024*1024 {
					entry["warning"] = "directory is very large"
					findings = append(findings, fmt.Sprintf("Public directory '%s' is very large (%s)", dir, fmtBytes(uint64(totalSize))))
				}
			}

			dirInfos = append(dirInfos, entry)
		}

		summary := fmt.Sprintf("%d public directories configured, %d exists, %d missing",
			len(dirs), existCount, missingCount)

		return map[string]interface{}{
			"public_dirs":      dirInfos,
			"total_public_dirs": len(dirs),
			"findings":         findings,
			"summary":          summary,
		}, nil
	})
}
