package main

// trusted_publisher.go — data model for trusted CI publishing relationships.
//
// A trusted publisher relationship links a namespace (or specific package) to a
// CI identity, enabling automated publishing beyond simple RBAC grants. These
// relationships are stored as JSON files under:
//
//   artifacts/.trusted-publishers/{publisherID}/{relationship-id}.json
//
// Each relationship records:
//   - The CI provider (e.g., "github-actions", "gitlab-ci")
//   - The repository or workflow identity allowed to publish
//   - Optional constraints (branch, tag pattern, environment)
//   - Audit metadata (who created it, when)

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
)

const trustedPublishersDir = "artifacts/.trusted-publishers"

// TrustedPublisher represents a trusted CI publishing relationship.
type TrustedPublisher struct {
	// ID is a unique identifier for this relationship.
	ID string `json:"id"`

	// PublisherID is the namespace this relationship applies to.
	PublisherID string `json:"publisher_id"`

	// PackageName optionally scopes the relationship to a specific package.
	// Empty means the relationship covers the entire namespace.
	PackageName string `json:"package_name,omitempty"`

	// Provider identifies the CI system (e.g., "github-actions", "gitlab-ci", "jenkins").
	Provider string `json:"provider"`

	// RepositoryOwner is the owner of the source repository (e.g., GitHub org/user).
	RepositoryOwner string `json:"repository_owner"`

	// RepositoryName is the name of the source repository.
	RepositoryName string `json:"repository_name"`

	// WorkflowRef optionally constrains to a specific workflow file or job.
	WorkflowRef string `json:"workflow_ref,omitempty"`

	// BranchPattern optionally constrains publishing to branches matching this pattern.
	// Supports glob patterns (e.g., "main", "release/*").
	BranchPattern string `json:"branch_pattern,omitempty"`

	// TagPattern optionally constrains publishing to tags matching this pattern.
	TagPattern string `json:"tag_pattern,omitempty"`

	// Environment optionally constrains to a specific deployment environment.
	Environment string `json:"environment,omitempty"`

	// CreatedBy records who established this trust relationship.
	CreatedBy string `json:"created_by"`

	// CreatedAt records when this relationship was established.
	CreatedAt string `json:"created_at"`
}

// trustedPublisherStorageKey returns the storage path for a trusted publisher relationship.
func trustedPublisherStorageKey(publisherID, relationshipID string) string {
	return trustedPublishersDir + "/" + publisherID + "/" + relationshipID + ".json"
}

// trustedPublisherDirKey returns the storage directory for a namespace's trusted publishers.
func trustedPublisherDirKey(publisherID string) string {
	return trustedPublishersDir + "/" + publisherID
}

// writeTrustedPublisher persists a trusted publisher relationship.
func (srv *server) writeTrustedPublisher(ctx context.Context, tp *TrustedPublisher) error {
	if tp.ID == "" {
		tp.ID = Utility.RandomUUID()
	}
	if tp.CreatedAt == "" {
		tp.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	// Extract creator from auth context.
	if tp.CreatedBy == "" {
		if authCtx := security.FromContext(ctx); authCtx != nil {
			tp.CreatedBy = authCtx.Subject
		}
	}

	data, err := json.MarshalIndent(tp, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal trusted publisher: %w", err)
	}

	dirKey := trustedPublisherDirKey(tp.PublisherID)
	if err := srv.Storage().MkdirAll(ctx, dirKey, 0o755); err != nil {
		return fmt.Errorf("create trusted publishers dir: %w", err)
	}

	storageKey := trustedPublisherStorageKey(tp.PublisherID, tp.ID)
	if err := srv.Storage().WriteFile(ctx, storageKey, data, 0o644); err != nil {
		return fmt.Errorf("write trusted publisher %q: %w", storageKey, err)
	}

	slog.Info("trusted publisher relationship created",
		"id", tp.ID,
		"publisher", tp.PublisherID,
		"provider", tp.Provider,
		"repo", tp.RepositoryOwner+"/"+tp.RepositoryName,
		"created_by", tp.CreatedBy,
	)

	srv.publishAuditEvent(ctx, "trusted_publisher.created", map[string]any{
		"id":               tp.ID,
		"publisher_id":     tp.PublisherID,
		"provider":         tp.Provider,
		"repository_owner": tp.RepositoryOwner,
		"repository_name":  tp.RepositoryName,
	})

	return nil
}

// readTrustedPublisher reads a single trusted publisher relationship.
func (srv *server) readTrustedPublisher(ctx context.Context, publisherID, relationshipID string) (*TrustedPublisher, error) {
	storageKey := trustedPublisherStorageKey(publisherID, relationshipID)
	data, err := srv.Storage().ReadFile(ctx, storageKey)
	if err != nil {
		return nil, fmt.Errorf("read trusted publisher %q: %w", storageKey, err)
	}

	tp := &TrustedPublisher{}
	if err := json.Unmarshal(data, tp); err != nil {
		return nil, fmt.Errorf("unmarshal trusted publisher: %w", err)
	}
	return tp, nil
}

// listTrustedPublishers returns all trusted publisher relationships for a namespace.
func (srv *server) listTrustedPublishers(ctx context.Context, publisherID string) ([]*TrustedPublisher, error) {
	dirKey := trustedPublisherDirKey(publisherID)
	entries, err := srv.Storage().ReadDir(ctx, dirKey)
	if err != nil {
		return nil, nil // no relationships
	}

	var result []*TrustedPublisher
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		relID := strings.TrimSuffix(name, ".json")
		tp, err := srv.readTrustedPublisher(ctx, publisherID, relID)
		if err != nil {
			slog.Warn("skip unreadable trusted publisher", "publisher", publisherID, "id", relID, "err", err)
			continue
		}
		result = append(result, tp)
	}
	return result, nil
}

// deleteTrustedPublisher removes a trusted publisher relationship.
func (srv *server) deleteTrustedPublisher(ctx context.Context, publisherID, relationshipID string) error {
	storageKey := trustedPublisherStorageKey(publisherID, relationshipID)
	if err := srv.Storage().Remove(ctx, storageKey); err != nil {
		return fmt.Errorf("delete trusted publisher %q: %w", storageKey, err)
	}

	slog.Info("trusted publisher relationship deleted",
		"id", relationshipID,
		"publisher", publisherID,
	)

	srv.publishAuditEvent(ctx, "trusted_publisher.deleted", map[string]any{
		"id":           relationshipID,
		"publisher_id": publisherID,
	})

	return nil
}

// matchesTrustedPublisherBySubject checks if an APPLICATION principal has a matching
// trusted publisher relationship for the given namespace (and optionally package).
// Matches the subject against RepositoryOwner or RepositoryName fields of each relationship.
func (srv *server) matchesTrustedPublisherBySubject(ctx context.Context, publisherID, packageName, subject string) bool {
	publishers, err := srv.listTrustedPublishers(ctx, publisherID)
	if err != nil || len(publishers) == 0 {
		return false
	}
	for _, tp := range publishers {
		// Match subject against repository owner (CI identity).
		if !strings.EqualFold(tp.RepositoryOwner, subject) && !strings.EqualFold(tp.RepositoryName, subject) {
			continue
		}
		// If relationship is scoped to a specific package, check it.
		if tp.PackageName != "" && !strings.EqualFold(tp.PackageName, packageName) {
			continue
		}
		return true
	}
	return false
}

// matchesTrustedPublisher checks if a given CI identity matches any trusted publisher
// relationship for the namespace. Returns true if a match is found.
func (srv *server) matchesTrustedPublisher(ctx context.Context, publisherID, provider, repoOwner, repoName string) bool {
	publishers, err := srv.listTrustedPublishers(ctx, publisherID)
	if err != nil || len(publishers) == 0 {
		return false
	}

	for _, tp := range publishers {
		if !strings.EqualFold(tp.Provider, provider) {
			continue
		}
		if !strings.EqualFold(tp.RepositoryOwner, repoOwner) {
			continue
		}
		if !strings.EqualFold(tp.RepositoryName, repoName) {
			continue
		}
		return true
	}
	return false
}
