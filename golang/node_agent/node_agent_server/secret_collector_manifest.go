// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.pki
// @awareness file_role=secret_collector_manifest_types
// @awareness risk=medium
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/globulario/services/golang/node_agent/node_agentpb"
)

// perNodeManifest mirrors the structure written to
// <capsule>/payload/secrets/<node_id>/manifest.json.
//
// We use an explicit struct (rather than protojson) so the on-disk format
// is reviewable as plain JSON and stable across proto regeneration. The
// restore script reads this manifest with `jq` — no proto knowledge needed
// on the operator side.
type perNodeManifest struct {
	Version           int                       `json:"version"`
	NodeID            string                    `json:"node_id"`
	Hostname          string                    `json:"hostname"`
	PrimaryIP         string                    `json:"primary_ip"`
	NodeAgentVersion  string                    `json:"node_agent_version"`
	CollectedAtUnix   string                    `json:"collected_at_unix"`
	Entries           []perNodeManifestEntry    `json:"entries"`
	MissingRequired   []string                  `json:"missing_required"`
	MissingOptional   []string                  `json:"missing_optional"`
}

type perNodeManifestEntry struct {
	OriginalPath       string `json:"original_path"`
	CapsuleRelpath     string `json:"capsule_relpath"`
	ModeOctal          string `json:"mode_octal,omitempty"`
	Owner              string `json:"owner,omitempty"`
	Group              string `json:"group,omitempty"`
	SizeBytes          uint64 `json:"size_bytes"`
	Sha256             string `json:"sha256,omitempty"`
	Required           bool   `json:"required"`
	OptionalWhenAbsent bool   `json:"optional_when_absent"`
	Found              bool   `json:"found"`
	Reason             string `json:"reason,omitempty"`
	ProducedBy         string `json:"produced_by,omitempty"`
}

// writePerNodeManifest renders the per-node manifest to disk via tmp+rename.
// Mode 0640 so the backup_manager user (group=globular) can read it.
func writePerNodeManifest(path string, resp *node_agentpb.CollectBackupSecretsResponse) error {
	m := perNodeManifest{
		Version:          2,
		NodeID:           resp.NodeId,
		Hostname:         resp.Hostname,
		PrimaryIP:        resp.PrimaryIp,
		NodeAgentVersion: resp.NodeAgentVersion,
		CollectedAtUnix:  resp.CollectedAtUnix,
		MissingRequired:  resp.MissingRequired,
		MissingOptional:  resp.MissingOptional,
	}
	for _, e := range resp.Entries {
		m.Entries = append(m.Entries, perNodeManifestEntry{
			OriginalPath:       e.OriginalPath,
			CapsuleRelpath:     e.CapsuleRelpath,
			ModeOctal:          e.ModeOctal,
			Owner:              e.Owner,
			Group:              e.Group,
			SizeBytes:          e.SizeBytes,
			Sha256:             e.Sha256,
			Required:           e.Required,
			OptionalWhenAbsent: e.OptionalWhenAbsent,
			Found:              e.Found,
			Reason:             e.Reason,
			ProducedBy:         e.ProducedBy,
		})
	}
	buf, err := json.MarshalIndent(&m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o640); err != nil {
		return fmt.Errorf("write tmp manifest: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename manifest: %w", err)
	}
	// Sanity: ensure final path is exactly mode 0640 (umask may have downgraded).
	if err := os.Chmod(path, 0o640); err != nil {
		return fmt.Errorf("chmod manifest: %w", err)
	}
	_ = filepath.Clean(path) // doc: path is already cleaned by the caller
	return nil
}
