package main

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	resourcepb "github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
)

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
	return string(Utility.CreateDataChecksum(data))
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

	// Compute id and file path.
	id := bundleID(bundle.PackageDescriptor, rqst.Platform)
	repoPath := srv.Root + "/packages-repository"
	filePath := repoPath + "/" + id + ".tar.gz"

	// Read the archived binaries.
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read bundle file %q: %w", filePath, err)
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

	// Compute bundle id & repository path.
	id := bundleID(bundle.PackageDescriptor, bundle.Plaform)
	repoPath := srv.Root + "/packages-repository"
	if err := Utility.CreateDirIfNotExist(repoPath); err != nil {
		_ = stream.SendAndClose(&repopb.UploadBundleResponse{}) // best-effort close
		return fmt.Errorf("ensure repo dir: %w", err)
	}
	filePath := repoPath + "/" + id + ".tar.gz"

	// Persist archive.
	if err := os.WriteFile(filePath, bundle.Binairies, 0o644); err != nil {
		_ = stream.SendAndClose(&repopb.UploadBundleResponse{}) // best-effort close
		return fmt.Errorf("write bundle file %q: %w", filePath, err)
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
		"path", filePath,
	)
	return nil
}
