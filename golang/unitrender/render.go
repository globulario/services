package unitrender

import (
	"regexp"
	"strings"

	"github.com/globulario/services/golang/systemdutil"
)

// Inputs is the closed set of explicit template variables allowed to affect
// rendered package assets. Unit rendering must depend only on these values and
// the package artifact content, never on staging paths, cwd, or on-disk units.
type Inputs struct {
	StateDir     string
	Prefix       string
	BinDir       string
	LogDir       string
	MinioDataDir string
	NodeIP       string
}

var simpleTemplateVarRE = regexp.MustCompile(`\{\{\s*\.\s*([A-Za-z0-9_]+)\s*\}\}`)

// RenderBytes replaces the supported {{.Var}} placeholders using explicit
// render inputs. Unknown variables are preserved as-is.
func RenderBytes(content []byte, in Inputs) []byte {
	return []byte(simpleTemplateVarRE.ReplaceAllStringFunc(string(content), func(match string) string {
		sub := simpleTemplateVarRE.FindStringSubmatch(match)
		if len(sub) != 2 {
			return match
		}
		switch strings.ToLower(strings.TrimSpace(sub[1])) {
		case "statedir":
			return in.StateDir
		case "prefix":
			return in.Prefix
		case "bindir":
			return in.BinDir
		case "logdir":
			return in.LogDir
		case "miniodatadir":
			return in.MinioDataDir
		case "nodeip":
			return in.NodeIP
		default:
			return match
		}
	}))
}

// RenderSystemdUnitBytes applies the explicit render inputs and then performs
// the canonical WorkingDirectory normalization used by runtime install paths.
func RenderSystemdUnitBytes(content []byte, in Inputs) []byte {
	return systemdutil.NormalizeUnitWorkingDirectory(RenderBytes(content, in))
}
