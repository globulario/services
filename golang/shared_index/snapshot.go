package shared_index

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	snapshotBucket = "globular-search-index"
)

// Manifest describes which segment files make up a Bleve index snapshot.
type Manifest struct {
	Version   int64             `json:"version"`
	Segments  []string          `json:"segments"`
	Checksums map[string]string `json:"checksums"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// snapshotSync handles uploading/downloading Bleve index snapshots via MinIO.
type snapshotSync struct {
	group  string // "search", "title", "blog"
	logger *slog.Logger
}

func newSnapshotSync(group string, logger *slog.Logger) *snapshotSync {
	return &snapshotSync{group: group, logger: logger}
}

func (s *snapshotSync) minioClient() (*minio.Client, error) {
	cfg := config.GetMinIOConfig()
	return minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.Secure,
	})
}

// EnsureBucket creates the snapshot bucket if it doesn't exist.
func (s *snapshotSync) EnsureBucket() error {
	client, err := s.minioClient()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	exists, err := client.BucketExists(ctx, snapshotBucket)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, snapshotBucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}
		s.logger.Info("created snapshot bucket", "bucket", snapshotBucket)
	}
	return nil
}

// manifestKey returns the MinIO key for a given index's manifest.
func (s *snapshotSync) manifestKey(indexName string) string {
	return fmt.Sprintf("%s/%s/manifest.json", s.group, indexName)
}

// segmentKey returns the MinIO key for a segment file.
func (s *snapshotSync) segmentKey(indexName, filename string) string {
	return fmt.Sprintf("%s/%s/store/%s", s.group, indexName, filename)
}

// GetManifest downloads the current manifest for an index.
// Returns nil if no manifest exists yet.
func (s *snapshotSync) GetManifest(indexName string) (*Manifest, error) {
	client, err := s.minioClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	obj, err := client.GetObject(ctx, snapshotBucket, s.manifestKey(indexName), minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()

	data, err := io.ReadAll(obj)
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "does not exist") {
			return nil, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, nil
}

// UploadSnapshot scans the local Bleve index directory and uploads any new or
// changed segment files to MinIO, then updates the manifest.
func (s *snapshotSync) UploadSnapshot(indexName, localDir string, currentVersion int64) (int64, error) {
	client, err := s.minioClient()
	if err != nil {
		return currentVersion, err
	}

	// List all files in the local index store directory.
	storeDir := filepath.Join(localDir, "store")
	entries, err := os.ReadDir(storeDir)
	if err != nil {
		// If no store dir, check for root-level index files.
		entries, err = os.ReadDir(localDir)
		if err != nil {
			return currentVersion, fmt.Errorf("read index dir: %w", err)
		}
		storeDir = localDir
	}

	// Get current manifest to compare checksums.
	existing, _ := s.GetManifest(indexName)
	existingChecksums := map[string]string{}
	if existing != nil {
		existingChecksums = existing.Checksums
	}

	newManifest := &Manifest{
		Version:   currentVersion + 1,
		Segments:  make([]string, 0),
		Checksums: make(map[string]string),
		UpdatedAt: time.Now().UTC(),
	}

	ctx := context.Background()
	uploaded := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "index_meta.json" || strings.HasSuffix(name, ".json") {
			// Upload metadata files too.
		}

		localPath := filepath.Join(storeDir, name)
		checksum, err := fileChecksum(localPath)
		if err != nil {
			s.logger.Warn("checksum failed, skipping", "file", name, "err", err)
			continue
		}

		newManifest.Segments = append(newManifest.Segments, name)
		newManifest.Checksums[name] = checksum

		// Skip upload if unchanged.
		if existingChecksums[name] == checksum {
			continue
		}

		// Upload file.
		f, err := os.Open(localPath)
		if err != nil {
			return currentVersion, fmt.Errorf("open %s: %w", name, err)
		}
		info, _ := f.Stat()
		_, err = client.PutObject(ctx, snapshotBucket, s.segmentKey(indexName, name),
			f, info.Size(), minio.PutObjectOptions{})
		f.Close()
		if err != nil {
			return currentVersion, fmt.Errorf("upload %s: %w", name, err)
		}
		uploaded++
	}

	// Also upload the root index_meta.json if it exists.
	metaPath := filepath.Join(localDir, "index_meta.json")
	if data, err := os.ReadFile(metaPath); err == nil {
		_, err = client.PutObject(ctx, snapshotBucket,
			fmt.Sprintf("%s/%s/index_meta.json", s.group, indexName),
			bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{})
		if err != nil {
			return currentVersion, fmt.Errorf("upload index_meta: %w", err)
		}
	}

	// Upload manifest.
	mData, err := json.MarshalIndent(newManifest, "", "  ")
	if err != nil {
		return currentVersion, err
	}
	_, err = client.PutObject(ctx, snapshotBucket, s.manifestKey(indexName),
		bytes.NewReader(mData), int64(len(mData)),
		minio.PutObjectOptions{ContentType: "application/json"})
	if err != nil {
		return currentVersion, fmt.Errorf("upload manifest: %w", err)
	}

	s.logger.Info("snapshot uploaded", "index", indexName,
		"version", newManifest.Version, "uploaded", uploaded, "total", len(newManifest.Segments))
	return newManifest.Version, nil
}

// DownloadSnapshot checks if the remote manifest is newer than localVersion,
// and if so downloads any missing segment files to localDir.
// Returns the new version and true if anything was downloaded.
func (s *snapshotSync) DownloadSnapshot(indexName, localDir string, localVersion int64) (int64, bool, error) {
	manifest, err := s.GetManifest(indexName)
	if err != nil || manifest == nil {
		return localVersion, false, err
	}
	if manifest.Version <= localVersion {
		return localVersion, false, nil
	}

	client, err := s.minioClient()
	if err != nil {
		return localVersion, false, err
	}

	// Ensure local store directory exists.
	storeDir := filepath.Join(localDir, "store")
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		return localVersion, false, fmt.Errorf("mkdir store: %w", err)
	}

	ctx := context.Background()
	downloaded := 0

	for _, seg := range manifest.Segments {
		localPath := filepath.Join(storeDir, seg)

		// Skip if we already have the right version.
		if localChecksum, err := fileChecksum(localPath); err == nil {
			if localChecksum == manifest.Checksums[seg] {
				continue
			}
		}

		// Download from MinIO.
		obj, err := client.GetObject(ctx, snapshotBucket, s.segmentKey(indexName, seg), minio.GetObjectOptions{})
		if err != nil {
			return localVersion, false, fmt.Errorf("get %s: %w", seg, err)
		}

		f, err := os.Create(localPath)
		if err != nil {
			obj.Close()
			return localVersion, false, fmt.Errorf("create %s: %w", seg, err)
		}
		_, err = io.Copy(f, obj)
		f.Close()
		obj.Close()
		if err != nil {
			return localVersion, false, fmt.Errorf("download %s: %w", seg, err)
		}
		downloaded++
	}

	// Download index_meta.json.
	metaKey := fmt.Sprintf("%s/%s/index_meta.json", s.group, indexName)
	if obj, err := client.GetObject(ctx, snapshotBucket, metaKey, minio.GetObjectOptions{}); err == nil {
		metaPath := filepath.Join(localDir, "index_meta.json")
		if f, err := os.Create(metaPath); err == nil {
			io.Copy(f, obj)
			f.Close()
		}
		obj.Close()
	}

	s.logger.Info("snapshot downloaded", "index", indexName,
		"version", manifest.Version, "downloaded", downloaded)
	return manifest.Version, downloaded > 0, nil
}

func fileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
