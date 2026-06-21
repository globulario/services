package opsknowledge

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// incident is the minimal, deterministic view of an incident Markdown file.
// Incidents are plain Markdown (no YAML front matter), so the parser stays
// conservative: it extracts the H1 title and a content hash only. Richer section
// extraction is intentionally NOT attempted (no free-form Markdown NLP).
type incident struct {
	Slug  string
	Title string
	Path  string // repo-relative
	Hash  string
}

// loadIncidents parses every *.md under dir (non-recursively) into incident
// records, sorted by path for deterministic ordering. A missing dir is not an
// error (the corpus may omit incidents).
func loadIncidents(dir, repoRel string) ([]incident, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []incident
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(data)
		out = append(out, incident{
			Slug:  fileSlug(e.Name()),
			Title: firstH1(string(data), e.Name()),
			Path:  filepath.ToSlash(filepath.Join(repoRel, "incidents", e.Name())),
			Hash:  hex.EncodeToString(sum[:]),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// firstH1 returns the first Markdown H1 ("# Title"), or a fallback derived from
// the filename if none is present.
func firstH1(content, fallbackName string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return strings.TrimSuffix(fallbackName, ".md")
}
