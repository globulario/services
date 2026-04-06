// schema-lint verifies that every etcd-backed type in the codebase has a
// +globular:schema:key pragma. Exits non-zero if undocumented types are found.
//
// Usage:
//
//	go run ./tools/schema-lint -root ./
//
// Detects types that reference etcd key patterns ("/globular/") in their
// vicinity without a schema pragma. This prevents new etcd-backed state
// from being added without ownership documentation.
//
// Opt-out: add //go:schemalint:ignore on the line before the type for
// justified exceptions (e.g., test-only types, internal helpers).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	root := flag.String("root", "./", "root directory to scan")
	flag.Parse()

	violations := lint(*root)
	if len(violations) > 0 {
		fmt.Fprintf(os.Stderr, "schema-lint: %d etcd-backed type(s) missing +globular:schema:key pragma:\n", len(violations))
		for _, v := range violations {
			fmt.Fprintf(os.Stderr, "  %s:%d — type %s (near etcd key reference)\n", v.File, v.Line, v.TypeName)
		}
		fmt.Fprintf(os.Stderr, "\nFix by adding schema pragmas or //go:schemalint:ignore with justification.\n")
		os.Exit(1)
	}
	fmt.Printf("schema-lint: all etcd-backed types have schema pragmas (%s)\n", *root)
}

type violation struct {
	File     string
	Line     int
	TypeName string
}

// etcdKeyIndicators are string patterns that suggest a nearby type is
// stored in etcd. We look for these within 10 lines of a type declaration.
var etcdKeyIndicators = []string{
	`"/globular/`,
	`EtcdKey`,
	`etcdPrefix`,
	`etcdKey(`,
}

// skipPatterns — files/dirs to skip entirely.
var skipDirs = map[string]bool{
	"vendor":           true,
	"testdata":         true,
	".git":             true,
	"schema_reference": true, // the reference package itself
	"tools":            true, // extractor/lint tools
}

func lint(root string) []violation {
	var violations []violation

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if strings.HasSuffix(path, ".pb.go") {
			return nil
		}

		vs := lintFile(path)
		violations = append(violations, vs...)
		return nil
	})

	return violations
}

func lintFile(path string) []violation {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	var violations []violation

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Look for type declarations.
		if !strings.HasPrefix(trimmed, "type ") {
			continue
		}
		// Extract type name.
		parts := strings.Fields(trimmed)
		if len(parts) < 2 {
			continue
		}
		typeName := parts[1]

		// Check if this type already has a schema pragma (look back 10 lines).
		hasPragma := false
		hasIgnore := false
		start := i - 10
		if start < 0 {
			start = 0
		}
		for j := start; j < i; j++ {
			if strings.Contains(lines[j], "+globular:schema:key=") {
				hasPragma = true
				break
			}
			if strings.Contains(lines[j], "go:schemalint:ignore") {
				hasIgnore = true
				break
			}
		}
		if hasPragma || hasIgnore {
			continue
		}

		// Check if there's an etcd key indicator near this type (±15 lines).
		hasEtcdRef := false
		searchStart := i - 15
		if searchStart < 0 {
			searchStart = 0
		}
		searchEnd := i + 15
		if searchEnd > len(lines) {
			searchEnd = len(lines)
		}
		for j := searchStart; j < searchEnd; j++ {
			for _, indicator := range etcdKeyIndicators {
				if strings.Contains(lines[j], indicator) {
					hasEtcdRef = true
					break
				}
			}
			if hasEtcdRef {
				break
			}
		}

		if hasEtcdRef {
			violations = append(violations, violation{
				File:     path,
				Line:     i + 1,
				TypeName: typeName,
			})
		}
	}

	return violations
}
