package generate

import (
	"encoding/json"
	"fmt"
	"strings"
)

// InspectResult holds the parsed proto inspection output.
type InspectResult struct {
	File     string        `json:"file"`
	Package  string        `json:"package"`
	GoPackage string       `json:"go_package,omitempty"`
	Services []InspectService `json:"services"`
	Messages []InspectMessage `json:"messages"`
	HasAuth  bool          `json:"has_auth"`
}

// InspectService describes a service found during inspection.
type InspectService struct {
	Name string       `json:"name"`
	RPCs []InspectRPC `json:"rpcs"`
}

// InspectRPC describes an RPC found during inspection.
type InspectRPC struct {
	Name             string `json:"name"`
	Input            string `json:"input"`
	Output           string `json:"output"`
	ServerStreaming   bool   `json:"server_streaming,omitempty"`
	ClientStreaming   bool   `json:"client_streaming,omitempty"`
	AuthzAction      string `json:"authz_action,omitempty"`
	AuthzPerm        string `json:"authz_permission,omitempty"`
	ResourceTemplate string `json:"resource_template,omitempty"`
	DefaultRoleHint  string `json:"default_role_hint,omitempty"`
}

// InspectMessage describes a message found during inspection.
type InspectMessage struct {
	Name   string         `json:"name"`
	Fields []InspectField `json:"fields"`
}

// InspectField describes a message field.
type InspectField struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Number   int32  `json:"number"`
	Repeated bool   `json:"repeated,omitempty"`
}

// InspectProto parses a proto file and returns a structured inspection result.
func InspectProto(protoFile string) (*InspectResult, error) {
	// Try protoc-based parsing first, fall back to regex
	data, err := ParseProto(protoFile)
	if err != nil {
		data, err = ParseProtoFallback(protoFile)
		if err != nil {
			return nil, fmt.Errorf("proto inspection failed: %w", err)
		}
	}

	// If protoc parsing worked but missed authz annotations, enrich with regex
	if !data.HasAuth {
		fallback, ferr := ParseProtoFallback(protoFile)
		if ferr == nil && fallback.HasAuth {
			// Merge authz data from fallback into protoc-parsed data
			for i := range data.RPCs {
				for _, frpc := range fallback.RPCs {
					if frpc.Name == data.RPCs[i].Name {
						data.RPCs[i].AuthzAction = frpc.AuthzAction
						data.RPCs[i].AuthzPerm = frpc.AuthzPerm
						data.RPCs[i].ResourceTemplate = frpc.ResourceTemplate
						data.RPCs[i].DefaultRoleHint = frpc.DefaultRoleHint
					}
				}
			}
			data.HasAuth = fallback.HasAuth
		}
	}

	result := &InspectResult{
		File:      protoFile,
		Package:   data.PackageName,
		GoPackage: data.GoPackage,
		HasAuth:   data.HasAuth,
	}

	svc := InspectService{Name: data.ServiceName}
	for _, rpc := range data.RPCs {
		svc.RPCs = append(svc.RPCs, InspectRPC{
			Name:             rpc.Name,
			Input:            rpc.InputType,
			Output:           rpc.OutputType,
			ServerStreaming:   rpc.IsServerStream,
			ClientStreaming:   rpc.IsClientStream,
			AuthzAction:      rpc.AuthzAction,
			AuthzPerm:        rpc.AuthzPerm,
			ResourceTemplate: rpc.ResourceTemplate,
			DefaultRoleHint:  rpc.DefaultRoleHint,
		})
	}
	result.Services = append(result.Services, svc)

	for _, msg := range data.Messages {
		im := InspectMessage{Name: msg.Name}
		for _, f := range msg.Fields {
			im.Fields = append(im.Fields, InspectField{
				Name:     f.Name,
				Type:     f.Type,
				Number:   f.Number,
				Repeated: f.Repeated,
			})
		}
		result.Messages = append(result.Messages, im)
	}

	return result, nil
}

// FormatInspectResult formats the inspection result as text or JSON.
func FormatInspectResult(result *InspectResult, asJSON bool) (string, error) {
	if asJSON {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "File:       %s\n", result.File)
	fmt.Fprintf(&b, "Package:    %s\n", result.Package)
	if result.GoPackage != "" {
		fmt.Fprintf(&b, "Go Package: %s\n", result.GoPackage)
	}
	fmt.Fprintf(&b, "Has Auth:   %v\n", result.HasAuth)
	b.WriteString("\n")

	for _, svc := range result.Services {
		fmt.Fprintf(&b, "Service: %s\n", svc.Name)
		for _, rpc := range svc.RPCs {
			streaming := ""
			if rpc.ServerStreaming {
				streaming = " [server-stream]"
			}
			if rpc.ClientStreaming {
				streaming = " [client-stream]"
			}
			if rpc.ServerStreaming && rpc.ClientStreaming {
				streaming = " [bidi-stream]"
			}
			fmt.Fprintf(&b, "  rpc %s(%s) returns (%s)%s\n", rpc.Name, rpc.Input, rpc.Output, streaming)
			if rpc.AuthzAction != "" {
				fmt.Fprintf(&b, "      authz: action=%s perm=%s resource=%s role=%s\n",
					rpc.AuthzAction, rpc.AuthzPerm, rpc.ResourceTemplate, rpc.DefaultRoleHint)
			}
		}
		b.WriteString("\n")
	}

	for _, msg := range result.Messages {
		fmt.Fprintf(&b, "Message: %s\n", msg.Name)
		for _, f := range msg.Fields {
			rep := ""
			if f.Repeated {
				rep = "repeated "
			}
			fmt.Fprintf(&b, "  %s%s %s = %d\n", rep, f.Type, f.Name, f.Number)
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}
