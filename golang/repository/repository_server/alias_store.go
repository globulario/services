package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

type releaseBuildAliasRecord struct {
	PublisherID      string `json:"publisher_id"`
	Name             string `json:"name"`
	Version          string `json:"version"`
	Platform         string `json:"platform"`
	Kind             string `json:"kind,omitempty"`
	Source           string `json:"source,omitempty"`
	ReleaseTag       string `json:"release_tag"`
	BuildNumber      int64  `json:"build_number"`
	UpstreamBuildID  string `json:"upstream_build_id,omitempty"`
	CanonicalBuildID string `json:"canonical_build_id"`
	ArtifactSha256   string `json:"artifact_sha256,omitempty"`
	OriginRelease    string `json:"origin_release,omitempty"`
	CreatedAt        string `json:"created_at"`
}

func aliasStorageKey(ref *repopb.ArtifactRef, releaseTag string, buildNumber int64) string {
	return path.Join(
		artifactsDir,
		"aliases",
		sanitizeAliasPathSegment(ref.GetPublisherId()),
		sanitizeAliasPathSegment(ref.GetName()),
		sanitizeAliasPathSegment(ref.GetVersion()),
		sanitizeAliasPathSegment(ref.GetPlatform()),
		sanitizeAliasPathSegment(releaseTag),
		strconv.FormatInt(buildNumber, 10)+".json",
	)
}

func sanitizeAliasPathSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "_"
	}
	repl := strings.NewReplacer("/", "_", "\\", "_", "..", "_")
	return repl.Replace(s)
}

func (srv *server) ensureReleaseBuildAlias(
	ctx context.Context,
	ref *repopb.ArtifactRef,
	releaseTag string,
	buildNumber int64,
	upstreamBuildID string,
	canonicalBuildID string,
	artifactSHA256 string,
	originRelease string,
	source string,
) error {
	if ref == nil || strings.TrimSpace(releaseTag) == "" || buildNumber <= 0 || strings.TrimSpace(canonicalBuildID) == "" {
		return nil
	}
	storageKey := aliasStorageKey(ref, releaseTag, buildNumber)
	dir := path.Dir(storageKey)
	if err := srv.Storage().MkdirAll(ctx, dir, 0o755); err != nil {
		return fmt.Errorf("alias mkdir %q: %w", dir, err)
	}

	if existing, err := srv.Storage().ReadFile(ctx, storageKey); err == nil && len(existing) > 0 {
		var prev releaseBuildAliasRecord
		if json.Unmarshal(existing, &prev) == nil {
			if strings.TrimSpace(prev.CanonicalBuildID) != "" && prev.CanonicalBuildID != canonicalBuildID {
				return fmt.Errorf("alias conflict at %s: canonical_build_id=%s existing=%s",
					storageKey, canonicalBuildID, prev.CanonicalBuildID)
			}
		}
	}

	record := releaseBuildAliasRecord{
		PublisherID:      ref.GetPublisherId(),
		Name:             ref.GetName(),
		Version:          ref.GetVersion(),
		Platform:         ref.GetPlatform(),
		Kind:             ref.GetKind().String(),
		Source:           source,
		ReleaseTag:       releaseTag,
		BuildNumber:      buildNumber,
		UpstreamBuildID:  strings.TrimSpace(upstreamBuildID),
		CanonicalBuildID: canonicalBuildID,
		ArtifactSha256:   strings.TrimSpace(artifactSHA256),
		OriginRelease:    strings.TrimSpace(originRelease),
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("alias marshal %q: %w", storageKey, err)
	}
	if err := srv.Storage().WriteFile(ctx, storageKey, raw, 0o644); err != nil {
		return fmt.Errorf("alias write %q: %w", storageKey, err)
	}
	return nil
}
