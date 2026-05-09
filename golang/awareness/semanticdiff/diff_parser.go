package semanticdiff

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// ParseDiff parses a unified diff into a ParsedDiff.
func ParseDiff(diffText string) (*ParsedDiff, error) {
	pd := &ParsedDiff{
		Fingerprint: DiffFingerprint(diffText),
	}
	var curFile *DiffFile
	var curHunk *DiffHunk

	lines := strings.Split(diffText, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		switch {
		case strings.HasPrefix(line, "--- "):
			// start of a file block
			curHunk = nil
			oldPath := strings.TrimPrefix(line, "--- ")
			oldPath = strings.TrimPrefix(oldPath, "a/")
			curFile = &DiffFile{OldPath: strings.TrimSpace(oldPath)}
			pd.Files = append(pd.Files, curFile)
		case strings.HasPrefix(line, "+++ "):
			if curFile != nil {
				newPath := strings.TrimPrefix(line, "+++ ")
				newPath = strings.TrimPrefix(newPath, "b/")
				curFile.Path = strings.TrimSpace(newPath)
			}
		case strings.HasPrefix(line, "@@ "):
			if curFile == nil {
				continue
			}
			curHunk = &DiffHunk{Symbol: parseHunkSymbol(line)}
			curFile.Hunks = append(curFile.Hunks, curHunk)
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			if curHunk != nil {
				curHunk.AddedLines = append(curHunk.AddedLines, line[1:])
			}
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			if curHunk != nil {
				curHunk.RemovedLines = append(curHunk.RemovedLines, line[1:])
			}
		}
	}
	return pd, nil
}

// parseHunkSymbol extracts the optional function/method name from a hunk header.
// @@ -10,5 +10,7 @@ func Reconcile(ctx context.Context)  →  "Reconcile"
func parseHunkSymbol(header string) string {
	// Find second @@
	idx := strings.Index(header[2:], "@@")
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(header[idx+4:])
	if rest == "" {
		return ""
	}
	// Extract the first identifier (function/method name)
	// common Go pattern: "func FuncName(" or "func (r *T) Method("
	if strings.HasPrefix(rest, "func ") {
		name := strings.TrimPrefix(rest, "func ")
		// handle receivers: (r *T) Method
		if strings.HasPrefix(name, "(") {
			if close := strings.Index(name, ")"); close >= 0 {
				name = strings.TrimSpace(name[close+1:])
			}
		}
		// take up to first non-identifier char
		for j, c := range name {
			if c == '(' || c == ' ' || c == '\t' {
				return name[:j]
			}
		}
		return name
	}
	// generic: take first word
	fields := strings.Fields(rest)
	if len(fields) > 0 {
		return fields[0]
	}
	return ""
}

// DiffFingerprint returns sha256:<hex> of normalized diff text.
func DiffFingerprint(diffText string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(diffText)))
	return fmt.Sprintf("sha256:%x", h)
}
