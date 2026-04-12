// minio_staging.go handles content-addressed input/output staging via MinIO.
//
// Inputs declared as ObjectRefs in the job spec are fetched from MinIO during
// StageComputeUnit and materialized into the execution directory. Outputs are
// uploaded to MinIO during CommitComputeOutput with computed checksums.
//
// etcd stores only metadata (ObjectRef pointers) — never blobs.
package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/globulario/services/golang/compute/computepb"
	"github.com/globulario/services/golang/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var (
	minioOnce   sync.Once
	minioClient *minio.Client
	minioErr    error
	minioBucket string
	minioPrefix string
)

// ensureMinioCompute lazily initializes the MinIO client from etcd config.
func ensureMinioCompute() (*minio.Client, string, string, error) {
	minioOnce.Do(func() {
		cfg, err := config.LoadMinIOConfig()
		if err != nil {
			minioErr = fmt.Errorf("compute minio: %w", err)
			return
		}

		opts := &minio.Options{
			Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
			Secure: cfg.Secure,
		}

		transport := &http.Transport{DialContext: config.ClusterDialContext}
		if cfg.Secure {
			tlsCfg := &tls.Config{}
			caPath := config.GetLocalCACertificate()
			if caPath != "" {
				caPEM, err := os.ReadFile(caPath)
				if err == nil {
					pool := x509.NewCertPool()
					pool.AppendCertsFromPEM(caPEM)
					tlsCfg.RootCAs = pool
				}
			}
			transport.TLSClientConfig = tlsCfg
		}
		opts.Transport = transport

		client, err := minio.New(cfg.Endpoint, opts)
		if err != nil {
			minioErr = fmt.Errorf("compute minio client: %w", err)
			return
		}
		minioClient = client
		minioBucket = cfg.Bucket
		minioPrefix = cfg.Prefix
		slog.Info("compute: minio client initialized",
			"endpoint", cfg.Endpoint, "bucket", cfg.Bucket)
	})
	return minioClient, minioBucket, minioPrefix, minioErr
}

// fetchInputRefs downloads input ObjectRefs from MinIO into the staging directory.
// Each input is written to stagingDir/{filename-from-key}. Checksums are verified
// when the ObjectRef specifies a sha256.
func fetchInputRefs(ctx context.Context, stagingDir string, refs []*computepb.ObjectRef) error {
	if len(refs) == 0 {
		return nil
	}
	client, bucket, prefix, err := ensureMinioCompute()
	if err != nil {
		return fmt.Errorf("fetch inputs: %w", err)
	}

	inputDir := filepath.Join(stagingDir, "input")
	if err := os.MkdirAll(inputDir, 0750); err != nil {
		return fmt.Errorf("create input dir: %w", err)
	}

	for i, ref := range refs {
		if ref == nil || ref.Uri == "" {
			continue
		}

		objectKey := resolveObjectKey(ref.Uri, prefix)
		localName := filenameFromKey(objectKey, i)
		localPath := filepath.Join(inputDir, localName)

		slog.Info("compute staging: fetching input",
			"key", objectKey, "bucket", bucket, "local", localPath)

		obj, err := client.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{})
		if err != nil {
			return fmt.Errorf("fetch input %q: %w", objectKey, err)
		}

		f, err := os.Create(localPath)
		if err != nil {
			obj.Close()
			return fmt.Errorf("create input file %q: %w", localPath, err)
		}

		hasher := sha256.New()
		w := io.MultiWriter(f, hasher)
		if _, err := io.Copy(w, obj); err != nil {
			f.Close()
			obj.Close()
			return fmt.Errorf("download input %q: %w", objectKey, err)
		}
		f.Close()
		obj.Close()

		// Verify checksum if declared.
		if ref.Sha256 != "" {
			actual := hex.EncodeToString(hasher.Sum(nil))
			if actual != ref.Sha256 {
				return fmt.Errorf("input %q checksum mismatch: expected %s, got %s",
					objectKey, ref.Sha256, actual)
			}
			slog.Info("compute staging: input checksum verified", "key", objectKey)
		}
	}
	return nil
}

// uploadOutput uploads the output directory contents to MinIO and returns
// the ObjectRef with the computed checksum. For v1, all files in the output
// directory are tarred into a single object.
func uploadOutput(ctx context.Context, stagingDir, jobID, unitID string) (*computepb.ObjectRef, error) {
	client, bucket, prefix, err := ensureMinioCompute()
	if err != nil {
		return nil, fmt.Errorf("upload output: %w", err)
	}

	outputDir := filepath.Join(stagingDir, "output")
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		// No output directory — return nil ref (not an error).
		return nil, nil
	}

	// Walk the output directory and upload each file individually.
	// For v1 single-unit jobs, this keeps things simple and debuggable.
	var totalSize uint64
	var lastRef *computepb.ObjectRef

	err = filepath.Walk(outputDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(outputDir, path)
		objectKey := computeOutputKey(prefix, jobID, unitID, relPath)

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open output %q: %w", path, err)
		}
		defer f.Close()

		// Compute checksum while uploading.
		hasher := sha256.New()
		tee := io.TeeReader(f, hasher)

		uploadInfo, err := client.PutObject(ctx, bucket, objectKey, tee, info.Size(),
			minio.PutObjectOptions{ContentType: "application/octet-stream"})
		if err != nil {
			return fmt.Errorf("upload output %q: %w", objectKey, err)
		}

		checksum := hex.EncodeToString(hasher.Sum(nil))
		totalSize += uint64(uploadInfo.Size)

		slog.Info("compute staging: output uploaded",
			"key", objectKey, "size", uploadInfo.Size, "sha256", checksum)

		lastRef = &computepb.ObjectRef{
			Uri:       fmt.Sprintf("minio://%s/%s", bucket, objectKey),
			Sha256:    checksum,
			SizeBytes: uint64(uploadInfo.Size),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return lastRef, nil
}

// resolveObjectKey extracts the MinIO object key from an ObjectRef URI.
// Supports formats:
//   - "minio://bucket/key" → "key"
//   - "key" (plain) → prefix + key
func resolveObjectKey(uri, prefix string) string {
	// Strip minio:// scheme and bucket.
	if strings.HasPrefix(uri, "minio://") {
		parts := strings.SplitN(strings.TrimPrefix(uri, "minio://"), "/", 2)
		if len(parts) == 2 {
			return parts[1] // key after bucket
		}
		return uri
	}
	// Plain key — prepend prefix if present.
	if prefix != "" && !strings.HasPrefix(uri, prefix) {
		return strings.TrimSuffix(prefix, "/") + "/" + uri
	}
	return uri
}

// filenameFromKey extracts a local filename from a MinIO object key.
func filenameFromKey(key string, index int) string {
	base := filepath.Base(key)
	if base == "" || base == "." || base == "/" {
		return fmt.Sprintf("input_%d", index)
	}
	return base
}

// computeOutputKey builds the MinIO key for a compute output file.
func computeOutputKey(prefix, jobID, unitID, relPath string) string {
	base := "compute/outputs/" + jobID + "/" + unitID + "/" + relPath
	if prefix != "" {
		return strings.TrimSuffix(prefix, "/") + "/" + base
	}
	return base
}
