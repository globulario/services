// update_artifact_binary.go — Hand-written message and stream types for the
// UpdateArtifactBinary RPC. These bridge the gap until ./generateCode.sh
// regenerates repository.pb.go and repository_grpc.pb.go from the updated
// proto/repository.proto.
//
// DELETE THIS FILE after running ./generateCode.sh — the protoc-generated
// code will supersede it.

package repositorypb

import "google.golang.org/grpc"

// ── Message types ───────────────────────────────────────────────────────────

// UpdateArtifactBinaryHeader is the first message in the stream.
type UpdateArtifactBinaryHeader struct {
	Ref       *ArtifactRef `json:"ref,omitempty"`
	Checksum  string       `json:"checksum,omitempty"`
	SizeBytes int64        `json:"size_bytes,omitempty"`
}

// UpdateArtifactBinaryRequest wraps either a header or binary chunk.
type UpdateArtifactBinaryRequest struct {
	// Exactly one of these fields is set per message.
	header *UpdateArtifactBinaryHeader
	chunk  []byte
}

// GetHeader returns the header if set, nil otherwise.
func (r *UpdateArtifactBinaryRequest) GetHeader() *UpdateArtifactBinaryHeader {
	if r == nil {
		return nil
	}
	return r.header
}

// GetChunk returns the binary chunk data.
func (r *UpdateArtifactBinaryRequest) GetChunk() []byte {
	if r == nil {
		return nil
	}
	return r.chunk
}

// ProtoReflect satisfies proto.Message — stub for compilation.
func (r *UpdateArtifactBinaryRequest) ProtoReflect() {}
func (r *UpdateArtifactBinaryRequest) Reset()        {}
func (r *UpdateArtifactBinaryRequest) String() string { return "UpdateArtifactBinaryRequest" }

// UpdateArtifactBinaryResponse reports the result of a delta deploy.
type UpdateArtifactBinaryResponse struct {
	BuildNumber int64  `json:"build_number,omitempty"`
	Checksum    string `json:"checksum,omitempty"`
	Status      string `json:"status,omitempty"`
}

func (r *UpdateArtifactBinaryResponse) GetBuildNumber() int64 {
	if r == nil {
		return 0
	}
	return r.BuildNumber
}

func (r *UpdateArtifactBinaryResponse) GetChecksum() string {
	if r == nil {
		return ""
	}
	return r.Checksum
}

func (r *UpdateArtifactBinaryResponse) GetStatus() string {
	if r == nil {
		return ""
	}
	return r.Status
}

func (r *UpdateArtifactBinaryResponse) ProtoReflect() {}
func (r *UpdateArtifactBinaryResponse) Reset()        {}
func (r *UpdateArtifactBinaryResponse) String() string { return "UpdateArtifactBinaryResponse" }

// ── Stream type alias ───────────────────────────────────────────────────────

// PackageRepository_UpdateArtifactBinaryServer is the server-side stream type.
type PackageRepository_UpdateArtifactBinaryServer = grpc.ClientStreamingServer[UpdateArtifactBinaryRequest, UpdateArtifactBinaryResponse]

// GetRef returns the ref from the header.
func (h *UpdateArtifactBinaryHeader) GetRef() *ArtifactRef {
	if h == nil {
		return nil
	}
	return h.Ref
}

// GetChecksum returns the expected checksum.
func (h *UpdateArtifactBinaryHeader) GetChecksum() string {
	if h == nil {
		return ""
	}
	return h.Checksum
}

// GetSizeBytes returns the expected size.
func (h *UpdateArtifactBinaryHeader) GetSizeBytes() int64 {
	if h == nil {
		return 0
	}
	return h.SizeBytes
}
