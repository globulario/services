package mcp

import (
	"context"
	"os/exec"
	"strings"
)

// toolHandler executes a tool and returns a JSON-serializable result.
// On failure, return (nil, err) — the server converts this to isError:true.
type toolHandler func(ctx context.Context, args map[string]interface{}) (interface{}, error)

// toolDef is the MCP tool definition as sent to clients in tools/list.
type toolDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema inputSchema `json:"inputSchema"`
}

// inputSchema describes a tool's input as a JSON Schema object.
type inputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]propSchema  `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
}

// propSchema describes a single property in an inputSchema.
type propSchema struct {
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Default     interface{} `json:"default,omitempty"`
	Enum        []string    `json:"enum,omitempty"`
	Items       *propSchema `json:"items,omitempty"` // for array types
}

// registeredTool pairs a definition with its handler.
type registeredTool struct {
	def     toolDef
	handler toolHandler
}

// register adds a tool to the server.
func (s *Server) register(def toolDef, handler toolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[def.Name] = &registeredTool{def: def, handler: handler}
	s.order = append(s.order, def.Name)
}

// registerAllTools adds all awareness tools.
// promote-proposal is intentionally not registered — promotion is CLI-only.
func registerAllTools(s *Server) {
	registerPreflightTool(s)
	registerAgentContextTool(s)
	registerImpactFileTool(s)
	registerRuntimeTool(s)
	registerFixledgerTools(s)
	registerPackageTools(s)
	registerLearningTools(s)
	registerNodeContextTools(s)
}

// strArg extracts a string argument from an MCP args map.
func strArg(args map[string]interface{}, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// strSliceArg extracts a []string argument from an MCP args map.
func strSliceArg(args map[string]interface{}, key string) []string {
	if v, ok := args[key]; ok {
		switch vv := v.(type) {
		case []interface{}:
			out := make([]string, 0, len(vv))
			for _, item := range vv {
				if s, ok := item.(string); ok {
					out = append(out, s)
				}
			}
			return out
		case []string:
			return vv
		}
	}
	return nil
}

// boolArg extracts a bool argument from an MCP args map.
func boolArg(args map[string]interface{}, key string) bool {
	if v, ok := args[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// runGit runs a git command and returns its stdout output.
func runGit(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
