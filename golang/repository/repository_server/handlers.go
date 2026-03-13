package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/globulario/services/golang/plan/versionutil"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	resourcepb "github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
)

// handlers.go
//
// Repository RPC handlers - pure business logic for package management.
//
// This file contains:
// - DownloadBundle: Stream gob-encoded package bundles to clients
// - UploadBundle: Receive and persist package bundles with metadata
// - Helper functions: bundleID, descriptorID, encoding, streaming utilities
//
// These handlers are pure functions with no side effects on server configuration.
// Package metadata is persisted via resource service client (business logic),
// NOT via srv.Save() (which would be a config side effect).
//
// Phase 1 Step 2: Renamed from repository.go for clarity.

const chunkSize = 5 * 1024 // 5 KiB per message chunk

// bundleID returns a deterministic bundle identifier from descriptor and platform.
func bundleID(d *resourcepb.PackageDescriptor, platform string) string {
	// Keep the exact concatenation logic used elsewhere for compatibility.
	base := d.PublisherID + "%" + d.Name + "%" + d.Version + "%" + d.Id + "%" + platform
	return Utility.GenerateUUID(base)
}

// descriptorID ensures the PackageDescriptor.Id is set deterministically.
func descriptorID(d *resourcepb.PackageDescriptor) string {
	base := d.PublisherID + "%" + d.Name + "%" + d.Version
	return Utility.GenerateUUID(base)
}

// encodeBundle gob-encodes a PackageBundle into a buffer.
func encodeBundle(b *resourcepb.PackageBundle) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(b); err != nil {
		return nil, fmt.Errorf("encode bundle: %w", err)
	}
	return &buf, nil
}

// sendBufferChunks streams a buffer in fixed-size chunks over DownloadBundleServer.
func sendBufferChunks(buf *bytes.Buffer, stream repopb.PackageRepository_DownloadBundleServer) error {
	for {
		if buf.Len() == 0 {
			return nil
		}
		n := chunkSize
		if buf.Len() < n {
			n = buf.Len()
		}
		chunk := make([]byte, n)
		if _, err := io.ReadFull(buf, chunk); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read chunk: %w", err)
		}
		if err := stream.Send(&repopb.DownloadBundleResponse{Data: chunk}); err != nil {
			return fmt.Errorf("send chunk: %w", err)
		}
	}
}

// readUploadStream accumulates bytes from UploadBundleServer into a buffer.
func readUploadStream(stream repopb.PackageRepository_UploadBundleServer) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	for {
		msg, err := stream.Recv()
		switch {
		case err == io.EOF || (msg != nil && len(msg.Data) == 0):
			return &buf, nil
		case err != nil:
			return nil, fmt.Errorf("recv upload: %w", err)
		case msg == nil:
			return nil, errors.New("recv upload: message is nil")
		default:
			if _, werr := buf.Write(msg.Data); werr != nil {
				return nil, fmt.Errorf("buffer write: %w", werr)
			}
		}
	}
}

// checksumBytes returns the hex string checksum of data.
func checksumBytes(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", h[:])
}

// ----------------------------------------------------------------------------
// Public API
// ----------------------------------------------------------------------------

// DownloadBundle streams a gob-encoded PackageBundle (.tar.gz bytes inside) to the client.
// It verifies the bundle checksum before sending and returns chunked responses for efficiency.
func (srv *server) DownloadBundle(
	rqst *repopb.DownloadBundleRequest,
	stream repopb.PackageRepository_DownloadBundleServer,
) error {
	if rqst == nil || rqst.Descriptor_ == nil {
		return errors.New("invalid request: missing descriptor")
	}

	// Build the bundle skeleton.
	bundle := &resourcepb.PackageBundle{
		Plaform:           rqst.Platform,     // NOTE: field name comes from proto, keep as-is
		PackageDescriptor: rqst.Descriptor_,  // incoming descriptor
	}

	// Build artifact ref and read from the artifacts/ directory.
	id := bundleID(bundle.PackageDescriptor, rqst.Platform)
	aRef := &repopb.ArtifactRef{
		Name:        bundle.PackageDescriptor.GetName(),
		Version:     bundle.PackageDescriptor.GetVersion(),
		Platform:    rqst.Platform,
		PublisherId: bundle.PackageDescriptor.GetPublisherID(),
	}
	aKey := artifactKeyWithBuild(aRef, 0)
	data, err := srv.Storage().ReadFile(stream.Context(), binaryStorageKey(aKey))
	if err != nil {
		return fmt.Errorf("read bundle artifact %q: %w", aKey, err)
	}
	bundle.Binairies = data

	// Verify checksum matches stored metadata.
	expected, err := srv.getPackageBundleChecksum(id)
	if err != nil {
		return fmt.Errorf("get checksum for id %q: %w", id, err)
	}
	actual := checksumBytes(bundle.Binairies)
	if actual != expected {
		return fmt.Errorf("invalid bundle checksum: got %s, expected %s", actual, expected)
	}

	// Encode and stream in chunks.
	buf, err := encodeBundle(bundle)
	if err != nil {
		return err
	}
	if err := sendBufferChunks(buf, stream); err != nil {
		return err
	}

	slog.Info("bundle downloaded",
		"id", id,
		"platform", rqst.Platform,
		"descriptor_name", bundle.PackageDescriptor.GetName(),
		"size", len(bundle.Binairies),
	)
	return nil
}

// UploadBundle receives a gob-encoded PackageBundle from the client,
// validates and persists the archive (.tar.gz) and its metadata (checksum/size/modified).
// On success, it closes the stream with an empty UploadBundleResponse.
func (srv *server) UploadBundle(stream repopb.PackageRepository_UploadBundleServer) error {
	// Read raw gob bytes from stream.
	buf, err := readUploadStream(stream)
	if err != nil {
		_ = stream.SendAndClose(&repopb.UploadBundleResponse{}) // best-effort close
		return err
	}

	// Decode bundle.
	var bundle resourcepb.PackageBundle
	if err := gob.NewDecoder(buf).Decode(&bundle); err != nil {
		_ = stream.SendAndClose(&repopb.UploadBundleResponse{}) // best-effort close
		return fmt.Errorf("decode bundle: %w", err)
	}

	// Ensure descriptor ID is set (deterministic).
	if bundle.PackageDescriptor == nil {
		_ = stream.SendAndClose(&repopb.UploadBundleResponse{}) // best-effort close
		return errors.New("missing package descriptor")
	}
	bundle.PackageDescriptor.Id = descriptorID(bundle.PackageDescriptor)

	// Normalize version to canonical semver (no v-prefix).
	if cv, err := versionutil.Canonical(bundle.PackageDescriptor.Version); err == nil {
		bundle.PackageDescriptor.Version = cv
	}

	// Compute artifact key and write to the artifacts/ directory.
	id := bundleID(bundle.PackageDescriptor, bundle.Plaform)
	d := bundle.PackageDescriptor
	aRef := &repopb.ArtifactRef{
		Name:        d.Name,
		Version:     d.Version,
		Platform:    bundle.Plaform,
		PublisherId: d.PublisherID,
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	aKey := artifactKeyWithBuild(aRef, 0)

	_ = srv.Storage().MkdirAll(stream.Context(), artifactsDir, 0o755)
	if err := srv.Storage().WriteFile(stream.Context(), binaryStorageKey(aKey), bundle.Binairies, 0o644); err != nil {
		_ = stream.SendAndClose(&repopb.UploadBundleResponse{}) // best-effort close
		return fmt.Errorf("write artifact %q: %w", aKey, err)
	}

	// Fill metadata and persist it via existing server method.
	bundle.Checksum = checksumBytes(bundle.Binairies)
	bundle.Size = int32(len(bundle.Binairies))
	bundle.Modified = time.Now().Unix()

	if err := srv.setPackageBundle(
		bundle.Checksum,
		bundle.Plaform,
		bundle.Size,
		bundle.Modified,
		bundle.PackageDescriptor,
	); err != nil {
		_ = stream.SendAndClose(&repopb.UploadBundleResponse{}) // best-effort close
		return fmt.Errorf("persist bundle metadata: %w", err)
	}

	// Write artifact manifest with PUBLISHED state.
	manifest := &repopb.ArtifactManifest{
		Ref:           aRef,
		Checksum:      bundle.Checksum,
		SizeBytes:     int64(bundle.Size),
		ModifiedUnix:  bundle.Modified,
		PublishedUnix: time.Now().Unix(),
	}

	// Enrich manifest from embedded manifest.json in the .tgz archive.
	enrichManifestFromArchive(manifest, bundle.Binairies)

	// Also populate from PackageDescriptor fields if manifest.json was absent.
	if manifest.Description == "" {
		manifest.Description = d.Description
	}
	if len(manifest.Keywords) == 0 {
		manifest.Keywords = d.Keywords
	}

	// Marshal with PUBLISHED state since UploadBundle is a complete publish.
	if mjson, merr := marshalManifestWithState(manifest, repopb.PublishState_PUBLISHED); merr == nil {
		_ = srv.Storage().WriteFile(stream.Context(), manifestStorageKey(aKey), mjson, 0o644)
	}
	slog.Info("artifact uploaded via UploadBundle", "key", aKey, "publish_state", "PUBLISHED")

	// Close stream, report success.
	if err := stream.SendAndClose(&repopb.UploadBundleResponse{}); err != nil {
		return fmt.Errorf("send close response: %w", err)
	}

	slog.Info("bundle uploaded",
		"id", id,
		"platform", bundle.Plaform,
		"descriptor_name", bundle.PackageDescriptor.GetName(),
		"size", bundle.Size,
		"modified", bundle.Modified,
		"key", aKey,
	)
	return nil
}

// enrichManifestFromArchive attempts to extract a manifest.json from a .tgz
// archive and populate the ArtifactManifest's enriched fields (description,
// keywords, icon, alias, license, etc.). If the archive doesn't contain a
// manifest.json or parsing fails, the manifest is left unchanged.
func enrichManifestFromArchive(m *repopb.ArtifactManifest, tgzData []byte) {
	gr, err := gzip.NewReader(bytes.NewReader(tgzData))
	if err != nil {
		return
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			return
		}
		// Look for manifest.json at any nesting level.
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := hdr.Name
		// Match "manifest.json" or "*/manifest.json"
		if name != "manifest.json" && !isManifestJSON(name) {
			continue
		}

		data, err := io.ReadAll(tr)
		if err != nil {
			return
		}

		// Parse as a loose JSON map — we only extract known fields.
		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			return
		}

		if v, ok := raw["description"].(string); ok && v != "" {
			m.Description = v
		}
		if v, ok := raw["alias"].(string); ok && v != "" {
			m.Alias = v
		}
		if v, ok := raw["icon"].(string); ok && v != "" {
			m.Icon = v
		}
		if v, ok := raw["license"].(string); ok && v != "" {
			m.License = v
		}
		if v, ok := raw["min_globular_version"].(string); ok && v != "" {
			m.MinGlobularVersion = v
		}
		if kws, ok := raw["keywords"].([]interface{}); ok && len(kws) > 0 {
			var keywords []string
			for _, kw := range kws {
				if s, ok := kw.(string); ok {
					keywords = append(keywords, s)
				}
			}
			if len(keywords) > 0 {
				m.Keywords = keywords
			}
		}
		return // done after first manifest.json
	}
}

// isManifestJSON returns true if the tar entry name ends with "/manifest.json".
func isManifestJSON(name string) bool {
	i := len(name) - len("/manifest.json")
	return i > 0 && name[i:] == "/manifest.json"
}
