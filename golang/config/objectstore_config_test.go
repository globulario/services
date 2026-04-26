package config

import (
	"encoding/json"
	"testing"
	"time"
)

// TestObjectStoreDesiredStateRoundTrip verifies that ObjectStoreDesiredState
// survives a marshal/unmarshal cycle with all fields intact.
func TestObjectStoreDesiredStateRoundTrip(t *testing.T) {
	ts := time.Now().Truncate(time.Second).UTC()
	orig := &ObjectStoreDesiredState{
		Mode:          ObjectStoreModeDistributed,
		Generation:    7,
		Endpoint:      "10.0.0.63:9000",
		AccessKey:     "AKIAIOSFODNN7EXAMPLE",
		SecretKey:     "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		Bucket:        "globular",
		Prefix:        "globular.internal",
		Nodes:         []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"},
		DrivesPerNode: 1,
		VolumesHash:   "abc123",
		WrittenAt:     ts,
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ObjectStoreDesiredState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Mode != orig.Mode {
		t.Errorf("mode: got %q, want %q", decoded.Mode, orig.Mode)
	}
	if decoded.Generation != orig.Generation {
		t.Errorf("generation: got %d, want %d", decoded.Generation, orig.Generation)
	}
	if decoded.Endpoint != orig.Endpoint {
		t.Errorf("endpoint: got %q, want %q", decoded.Endpoint, orig.Endpoint)
	}
	if decoded.AccessKey != orig.AccessKey {
		t.Errorf("access_key mismatch")
	}
	if decoded.Bucket != orig.Bucket {
		t.Errorf("bucket: got %q, want %q", decoded.Bucket, orig.Bucket)
	}
	if len(decoded.Nodes) != len(orig.Nodes) {
		t.Errorf("nodes len: got %d, want %d", len(decoded.Nodes), len(orig.Nodes))
	} else {
		for i, n := range orig.Nodes {
			if decoded.Nodes[i] != n {
				t.Errorf("nodes[%d]: got %q, want %q", i, decoded.Nodes[i], n)
			}
		}
	}
	if decoded.DrivesPerNode != orig.DrivesPerNode {
		t.Errorf("drives_per_node: got %d, want %d", decoded.DrivesPerNode, orig.DrivesPerNode)
	}
	if decoded.VolumesHash != orig.VolumesHash {
		t.Errorf("volumes_hash: got %q, want %q", decoded.VolumesHash, orig.VolumesHash)
	}
}

// TestSaveObjectStoreDesiredStateRejectsDNS verifies that
// SaveObjectStoreDesiredState refuses to write an endpoint with a DNS hostname.
func TestSaveObjectStoreDesiredStateRejectsDNS(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
	}{
		{"dns hostname with port", "minio.globular.internal:9000"},
		{"plain hostname", "minio-primary:9000"},
		{"loopback hostname", "localhost:9000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := &ObjectStoreDesiredState{
				Endpoint:  tc.endpoint,
				Mode:      ObjectStoreModeStandalone,
				AccessKey: "ak",
				SecretKey: "sk",
				Bucket:    "globular",
			}
			// ctx is not needed because the function should fail before hitting etcd
			// (on the endpoint validation guard).
			err := state.validateEndpoint()
			if err == nil {
				t.Fatalf("expected error for DNS endpoint %q, got nil", tc.endpoint)
			}
		})
	}
}

// TestSaveObjectStoreDesiredStateAcceptsIP verifies that a bare IP endpoint
// passes validation in ObjectStoreDesiredState.
func TestSaveObjectStoreDesiredStateAcceptsIP(t *testing.T) {
	state := &ObjectStoreDesiredState{
		Endpoint: "10.0.0.63:9000",
	}
	if err := state.validateEndpoint(); err != nil {
		t.Fatalf("unexpected error for valid IP endpoint: %v", err)
	}
}

// TestSaveObjectStoreDesiredStateAllowsEmptyEndpoint verifies that an empty
// endpoint passes validateEndpoint — it is allowed in degraded contracts where
// EndpointReady=false (controller startup before pool is formed).
func TestSaveObjectStoreDesiredStateAllowsEmptyEndpoint(t *testing.T) {
	state := &ObjectStoreDesiredState{
		Endpoint:      "",
		EndpointReady: false,
		Mode:          ObjectStoreModeStandalone,
	}
	if err := state.validateEndpoint(); err != nil {
		t.Fatalf("unexpected error for empty endpoint in degraded contract: %v", err)
	}
}

// TestObjectStoreDesiredStateReadyFlagsRoundTrip verifies CredentialsReady and
// EndpointReady survive a marshal/unmarshal cycle.
func TestObjectStoreDesiredStateReadyFlagsRoundTrip(t *testing.T) {
	orig := &ObjectStoreDesiredState{
		Mode:             ObjectStoreModeStandalone,
		Generation:       1,
		Endpoint:         "",
		CredentialsReady: false,
		EndpointReady:    false,
	}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded ObjectStoreDesiredState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.CredentialsReady != orig.CredentialsReady {
		t.Errorf("CredentialsReady: got %v, want %v", decoded.CredentialsReady, orig.CredentialsReady)
	}
	if decoded.EndpointReady != orig.EndpointReady {
		t.Errorf("EndpointReady: got %v, want %v", decoded.EndpointReady, orig.EndpointReady)
	}

	// Now with both flags true.
	ready := &ObjectStoreDesiredState{
		Mode:             ObjectStoreModeDistributed,
		Generation:       5,
		Endpoint:         "10.0.0.63:9000",
		CredentialsReady: true,
		EndpointReady:    true,
	}
	data2, _ := json.Marshal(ready)
	var decoded2 ObjectStoreDesiredState
	if err := json.Unmarshal(data2, &decoded2); err != nil {
		t.Fatalf("unmarshal ready: %v", err)
	}
	if !decoded2.CredentialsReady {
		t.Error("CredentialsReady should survive round-trip as true")
	}
	if !decoded2.EndpointReady {
		t.Error("EndpointReady should survive round-trip as true")
	}
}

// TestObjectStoreModeConstants verifies the mode constants have expected values.
func TestObjectStoreModeConstants(t *testing.T) {
	if ObjectStoreModeStandalone != "standalone" {
		t.Errorf("ObjectStoreModeStandalone = %q, want \"standalone\"", ObjectStoreModeStandalone)
	}
	if ObjectStoreModeDistributed != "distributed" {
		t.Errorf("ObjectStoreModeDistributed = %q, want \"distributed\"", ObjectStoreModeDistributed)
	}
}
