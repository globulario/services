// Package manual loads hand-authored awareness truth files into the graph.
// Files are optional: if a file does not exist, the loader skips it silently.
package manual

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/globulario/services/golang/awareness/graph"
)

// LoadAll loads all manual truth files from docsDir into g.
// Missing files are skipped without error.
// docsDir should be the docs/awareness/ directory in the repo.
func LoadAll(ctx context.Context, g *graph.Graph, docsDir string) error {
	loaders := []struct {
		file string
		fn   func(context.Context, *graph.Graph, string) error
	}{
		{"invariants.yaml", LoadInvariants},
		{"convergence_rules.yaml", LoadInvariants},
		{"failure_modes.yaml", LoadFailureModes},
		{"forbidden_fixes.yaml", LoadForbiddenFixes},
		{"services.yaml", LoadServices},
		{"patterns.yaml", LoadPatterns},
		{"design_patterns.yaml", LoadDesignPatterns},
	}

	for _, l := range loaders {
		path := filepath.Join(docsDir, l.file)
		if err := l.fn(ctx, g, path); err != nil {
			return fmt.Errorf("manual loader %s: %w", l.file, err)
		}
	}
	return nil
}
