package schema_reference

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ExtractResult is what the schema-extractor tool emits. Entries are
// sorted by KeyPattern so the generated JSON and markdown are stable
// across runs (clause 10: deterministic output).
type ExtractResult struct {
	// Source is always "schema-extractor"; recorded so MCP/CLI callers
	// know where the entries came from without asking (clause 4).
	Source string `json:"source"`
	// GeneratedAtUnix is when the extractor ran, unix seconds.
	GeneratedAtUnix int64 `json:"generated_at_unix"`
	// Entries are the parsed pragma blocks, sorted by KeyPattern.
	Entries []Entry `json:"entries"`
}

// A pragma line looks like:
//
//	// +globular:schema:key="/globular/.../{name}"
//
// The regex captures the field name (after the second colon) and the
// quoted value. We accept either straight quotes or backticks; spaces
// between `//` and `+globular` are tolerated.
var pragmaRe = regexp.MustCompile(`^\s*//\s*\+globular:schema:([a-z_]+)\s*=\s*(?:"([^"]*)"|` + "`" + `([^` + "`" + `]*)` + "`" + `)\s*$`)

// typeLineRe matches the `type Foo …` line that pragma blocks precede.
// We use it only to capture the Go type name — the key/writer/etc.
// come from the pragmas themselves.
var typeLineRe = regexp.MustCompile(`^\s*type\s+(\w+)\s+`)

// ExtractFile parses a single Go file and returns any pragma blocks it
// finds. Errors from file IO are returned; malformed pragma lines are
// reported as part of the returned error slice but do not abort parsing
// — the extractor tool aggregates errors and fails the build at the end.
func ExtractFile(path string) ([]Entry, []error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, []error{fmt.Errorf("open %s: %w", path, err)}
	}
	defer f.Close()

	var entries []Entry
	var errs []error

	// Collect pragma lines into a block; close the block when we see a
	// non-pragma / non-comment line. If the closing line is a `type T…`
	// declaration, the block is attached to that type. Otherwise it is
	// dropped with an error (orphan pragma).
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	type draft struct {
		firstLine int
		fields    map[string]string
	}
	var cur *draft
	lineNo := 0

	flush := func(typeName string, lastLine int) {
		if cur == nil {
			return
		}
		defer func() { cur = nil }()
		// Drop orphans: a block with no type declaration following it.
		if typeName == "" {
			errs = append(errs, fmt.Errorf("%s:%d: orphan schema pragma block (no `type` declaration follows)", path, cur.firstLine))
			return
		}
		e, perr := draftToEntry(cur.fields, cur.firstLine)
		if perr != nil {
			errs = append(errs, fmt.Errorf("%s:%d: %w", path, cur.firstLine, perr))
			return
		}
		e.TypeName = typeName
		e.SourceFile = path
		_ = lastLine // currently unused but kept as an explicit hook
		entries = append(entries, e)
	}

	for scanner.Scan() {
		lineNo++
		line := scanner.Text()

		if m := pragmaRe.FindStringSubmatch(line); m != nil {
			if cur == nil {
				cur = &draft{firstLine: lineNo, fields: map[string]string{}}
			}
			key := m[1]
			val := m[2]
			if val == "" {
				val = m[3]
			}
			// Allow repeated pragma fields by joining with "; " — this
			// is the explicit extension path for invariants that need
			// more than one sentence.
			if prior, ok := cur.fields[key]; ok {
				cur.fields[key] = prior + "; " + val
			} else {
				cur.fields[key] = val
			}
			continue
		}

		// A plain comment line inside a pragma block is ignored (lets
		// operators keep humans-eyes notes in the same paragraph).
		if strings.HasPrefix(strings.TrimSpace(line), "//") && cur != nil {
			continue
		}

		// Blank line between pragmas and type: keep the block open —
		// Go style sometimes puts blank lines before `type`.
		if strings.TrimSpace(line) == "" && cur != nil {
			continue
		}

		// Closing line: attach to `type X …` if we can.
		if cur != nil {
			if tm := typeLineRe.FindStringSubmatch(line); tm != nil {
				flush(tm[1], lineNo)
			} else {
				flush("", lineNo)
			}
			continue
		}
	}
	if err := scanner.Err(); err != nil {
		errs = append(errs, fmt.Errorf("scan %s: %w", path, err))
	}
	return entries, errs
}

// draftToEntry converts a parsed pragma block into an Entry and
// validates the required fields. Returns an error when a pragma block
// is missing a mandatory key (key_pattern / writer).
func draftToEntry(fields map[string]string, firstLine int) (Entry, error) {
	key := strings.TrimSpace(fields["key"])
	if key == "" {
		return Entry{}, fmt.Errorf("missing required pragma: +globular:schema:key")
	}
	writer := strings.TrimSpace(fields["writer"])
	if writer == "" {
		return Entry{}, fmt.Errorf("missing required pragma: +globular:schema:writer")
	}
	e := Entry{
		KeyPattern:   key,
		Writer:       writer,
		Description:  strings.TrimSpace(fields["description"]),
		Invariants:   strings.TrimSpace(fields["invariants"]),
		SinceVersion: strings.TrimSpace(fields["since_version"]),
		SourceLine:   firstLine,
	}
	if r := strings.TrimSpace(fields["readers"]); r != "" {
		for _, v := range strings.Split(r, ",") {
			v = strings.TrimSpace(v)
			if v != "" {
				e.Readers = append(e.Readers, v)
			}
		}
	}
	return e, nil
}

// ExtractTree walks `root` recursively, parsing every `.go` file under
// it (skipping testdata, vendor, generated .pb.go, and _test.go). The
// returned entries are sorted by KeyPattern so output is deterministic.
// Errors are collected and returned — the extractor tool decides
// whether to fail the build.
func ExtractTree(root string) (ExtractResult, []error) {
	var all []Entry
	var allErrs []error

	skipDirs := map[string]bool{
		"vendor":    true,
		"testdata":  true,
		"generated": true,
	}

	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			allErrs = append(allErrs, err)
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") {
			return nil
		}
		if strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, ".pb.go") {
			return nil
		}
		entries, errs := ExtractFile(path)
		all = append(all, entries...)
		allErrs = append(allErrs, errs...)
		return nil
	})

	// Detect duplicate key patterns — a pragma collision means two
	// types claim ownership of the same etcd key, which breaks the
	// single-writer invariant the schema reference exists to enforce.
	seen := map[string]Entry{}
	for _, e := range all {
		if prior, dup := seen[e.KeyPattern]; dup {
			allErrs = append(allErrs, fmt.Errorf(
				"duplicate key_pattern %q: %s:%d (%s) and %s:%d (%s)",
				e.KeyPattern,
				prior.SourceFile, prior.SourceLine, prior.TypeName,
				e.SourceFile, e.SourceLine, e.TypeName,
			))
			continue
		}
		seen[e.KeyPattern] = e
	}

	sort.Slice(all, func(i, j int) bool { return all[i].KeyPattern < all[j].KeyPattern })
	return ExtractResult{Source: "schema-extractor", Entries: all}, allErrs
}
