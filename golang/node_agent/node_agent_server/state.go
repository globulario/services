package main

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
)

// MigrateLegacyStatePathOnce relocates an existing state.json from the
// pre-Project-O `nodeagent/` (no separator) directory to the canonical
// `node-agent/` (hyphenated) directory. Idempotent.
//
// Rules (mirrors Project O.3 spec):
//   - If canonical exists  → keep canonical, leave legacy untouched, log warn
//     when both are present so the operator can resolve manually.
//   - If only legacy exists → create canonical's parent dir with safe perms,
//     rename legacy → canonical.
//   - Neither exists       → no-op.
//
// Failure to migrate is logged but NOT fatal — the load step decides whether
// the canonical path is loadable from whatever ended up there.
func MigrateLegacyStatePathOnce(canonical, legacy string) {
	if canonical == "" || legacy == "" || canonical == legacy {
		return
	}
	legacyExists := pathExists(legacy)
	canonicalExists := pathExists(canonical)
	if !legacyExists {
		return
	}
	if canonicalExists {
		log.Printf("state-migration: both %s and %s exist — canonical wins, legacy left in place for operator review", canonical, legacy)
		return
	}
	parent := filepath.Dir(canonical)
	if err := os.MkdirAll(parent, 0o750); err != nil {
		log.Printf("state-migration: WARN create parent %s: %v", parent, err)
		return
	}
	if err := os.Rename(legacy, canonical); err != nil {
		log.Printf("state-migration: WARN rename %s → %s: %v", legacy, canonical, err)
		return
	}
	log.Printf("state-migration: moved %s → %s", legacy, canonical)
}

func pathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

type nodeAgentState struct {
	ControllerEndpoint string `json:"controller_endpoint"`
	ControllerInsecure bool   `json:"controller_insecure"`
	RequestID          string `json:"request_id"`
	NodeID             string `json:"node_id"`
	JoinToken          string `json:"join_token"`
	NetworkGeneration  uint64 `json:"network_generation"`
	ClusterDomain      string `json:"cluster_domain"`
	Protocol           string `json:"protocol"`
	CertGeneration     uint64 `json:"cert_generation"`

	// DNS-first naming fields (PR1)
	NodeName      string `json:"node_name"`
	AdvertiseIP   string `json:"advertise_ip"`
	AdvertiseFQDN string `json:"advertise_fqdn"`

	// v2 join path: join_id is set by the installer via /join/authorize before
	// any cluster-affecting step. The node-agent uses it as the request_id for
	// GetJoinRequestStatus polling, bypassing the v1 RequestJoin call.
	JoinID string `json:"join_id,omitempty"`
	// JoinPlanJSON is the raw JoinPlan JSON written by the installer alongside
	// the join_id. The node-agent validates the plan before polling for admission.
	JoinPlanJSON []byte `json:"join_plan_json,omitempty"`
}

func newNodeAgentState() *nodeAgentState {
	return &nodeAgentState{}
}

func loadNodeAgentState(path string) (*nodeAgentState, error) {
	state := newNodeAgentState()
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state, nil
		}
		return nil, err
	}
	if len(b) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(b, state); err != nil {
		return nil, err
	}
	return state, nil
}

func (s *nodeAgentState) save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "nodeagent-state-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
