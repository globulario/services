package main

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Files or directories to ignore (non-production or expected occurrences).
var tlsScanAllowPrefixes = []string{
	"golang/nodeagent/nodeagent_server/internal/certs/etcd_kv.go",
	"golang/nodeagent/nodeagent_server/tls_scan_test.go",
}

var forbiddenPatterns = []*regexp.Regexp{
	// templated domain folders
	regexp.MustCompile(`tls/\{\{\s*\.\s*(Domain|Host)\s*\}\}`),
	regexp.MustCompile(`pki/\{\{\s*\.\s*(Domain|Host)\s*\}\}`),
	// string concat foldering
	regexp.MustCompile(`"tls/"\s*\+\s*domain`),
	regexp.MustCompile(`"pki/"\s*\+\s*domain`),
	regexp.MustCompile(`GetConfigDir\(\)\s*\+\s*"/tls/"\s*\+\s*domain`),
	regexp.MustCompile(`GetConfigDir\(\)\s*\+\s*"/pki/"\s*\+\s*domain`),
	// filepath.Join with domain/host foldering
	regexp.MustCompile(`filepath\.Join\([^)]*,\s*"tls"\s*,\s*[^)]*domain[^)]*\)`),
	regexp.MustCompile(`filepath\.Join\([^)]*,\s*"pki"\s*,\s*[^)]*domain[^)]*\)`),
	regexp.MustCompile(`filepath\.Join\([^)]*tlsDir[^)]*domain[^)]*\)`),
	regexp.MustCompile(`filepath\.Join\([^)]*GetRuntimeTLSDir\(\)[^)]*domain[^)]*\)`),
	// host/domain folder construction
	regexp.MustCompile(`"tls"\s*,\s*name\s*\+\s*"\."\s*\+\s*dom`),
	regexp.MustCompile(`"tls"\s*,\s*hn\s*\+\s*"\."\s*\+\s*dom`),
}

func TestCanonicalTLSPatterns(t *testing.T) {
	root := workspaceRoot(t)
	var offenders []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if strings.HasPrefix(path, filepath.Join(root, ".git")) ||
				strings.Contains(path, "vendor") ||
				strings.Contains(path, "third_party") ||
				strings.Contains(path, "generated") ||
				strings.Contains(path, "node_modules") ||
				strings.Contains(path, "dist") ||
				strings.Contains(path, "build") ||
				strings.Contains(path, "out") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if isAllowed(rel) {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".go", ".tmpl", ".yaml", ".yml", ".json", ".conf", ".cs", ".cpp", ".c", ".cc", ".cxx", ".h", ".hpp", ".sh":
		default:
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		line := 1
		for scanner.Scan() {
			txt := scanner.Text()
			for _, re := range forbiddenPatterns {
				if re.MatchString(txt) {
					offenders = append(offenders, rel+":"+intToStr(line)+": "+re.String()+" -> "+strings.TrimSpace(txt))
				}
			}
			line++
		}
		return nil
	})
	if len(offenders) > 0 {
		t.Fatalf("found forbidden TLS/pki patterns:\n%s", strings.Join(offenders, "\n"))
	}
}

func isAllowed(rel string) bool {
	for _, p := range tlsScanAllowPrefixes {
		if strings.HasPrefix(rel, p) {
			return true
		}
	}
	return false
}

func moduleRoot(t *testing.T) string {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cwd: %v", err)
	}
	start := dir
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod from %s", start)
		}
		dir = parent
	}
}

func workspaceRoot(t *testing.T) string {
	dir := moduleRoot(t)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}

func intToStr(i int) string {
	return strconv.Itoa(i)
}
