// Package proto extracts proto_service, rpc_method, and proto_message nodes
// from .proto files using simple line-based parsing.
package proto

import (
	"bufio"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/awareness/graph"
)

// Extract walks walkDir for .proto files and extracts service/rpc/message nodes.
// Paths are stored relative to pathRoot (typically the repo root).
func Extract(ctx context.Context, g *graph.Graph, walkDir, pathRoot string) error {
	return filepath.WalkDir(walkDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".proto") {
			return nil
		}
		rel, err := filepath.Rel(pathRoot, path)
		if err != nil {
			return err
		}
		return extractProtoFile(ctx, g, path, rel)
	})
}

func extractProtoFile(ctx context.Context, g *graph.Graph, absPath, relPath string) error {
	f, err := os.Open(absPath)
	if err != nil {
		return nil // skip unreadable files
	}
	defer f.Close()

	fileID := "source_file:" + relPath
	_ = g.AddNode(ctx, graph.Node{
		ID:   fileID,
		Type: graph.NodeTypeSourceFile,
		Name: filepath.Base(relPath),
		Path: relPath,
	})

	var currentService string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		switch {
		case strings.HasPrefix(line, "service "):
			// service Foo {
			name := extractProtoName(line)
			if name == "" {
				continue
			}
			currentService = name
			svcID := "proto_service:" + name
			_ = g.AddNode(ctx, graph.Node{
				ID:   svcID,
				Type: graph.NodeTypeProtoService,
				Name: name,
				Path: relPath,
			})
			_ = g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeDefines, Dst: svcID})

		case strings.HasPrefix(line, "rpc "):
			// rpc MethodName (Request) returns (Response) {}
			methodName := extractRPCName(line)
			if methodName == "" || currentService == "" {
				continue
			}
			methodID := "rpc_method:" + currentService + "." + methodName
			_ = g.AddNode(ctx, graph.Node{
				ID:   methodID,
				Type: graph.NodeTypeRPCMethod,
				Name: methodName,
				Path: relPath,
			})
			svcID := "proto_service:" + currentService
			_ = g.AddEdge(ctx, graph.Edge{Src: svcID, Kind: graph.EdgeOwns, Dst: methodID})

			// Link request/response message types.
			req, resp := extractRPCMessages(line)
			for _, msg := range []string{req, resp} {
				if msg == "" {
					continue
				}
				msgID := "proto_message:" + msg
				_ = g.AddNode(ctx, graph.Node{
					ID:   msgID,
					Type: graph.NodeTypeProtoMessage,
					Name: msg,
					Path: relPath,
				})
				_ = g.AddEdge(ctx, graph.Edge{Src: methodID, Kind: graph.EdgeRequires, Dst: msgID})
			}

		case strings.HasPrefix(line, "message "):
			// message Foo {
			name := extractProtoName(line)
			if name == "" {
				continue
			}
			msgID := "proto_message:" + name
			_ = g.AddNode(ctx, graph.Node{
				ID:   msgID,
				Type: graph.NodeTypeProtoMessage,
				Name: name,
				Path: relPath,
			})
			_ = g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeDefines, Dst: msgID})

		case line == "}" && currentService != "":
			// Rough heuristic: closing brace ends the current service context.
			// This is imprecise but sufficient for V1 awareness.
			currentService = ""
		}
	}
	return scanner.Err()
}

// extractProtoName extracts the identifier from "service Foo {" or "message Foo {".
func extractProtoName(line string) string {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSuffix(parts[1], "{")
}

// extractRPCName extracts the method name from "rpc MethodName (...) returns (...) {}".
func extractRPCName(line string) string {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// extractRPCMessages extracts request and response message types.
func extractRPCMessages(line string) (req, resp string) {
	// rpc Foo (ReqType) returns (RespType) {}
	open := strings.Index(line, "(")
	close := strings.Index(line, ")")
	if open < 0 || close < 0 || close <= open {
		return "", ""
	}
	req = strings.TrimSpace(line[open+1 : close])

	rest := line[close+1:]
	retIdx := strings.Index(rest, "returns")
	if retIdx < 0 {
		return req, ""
	}
	rest = rest[retIdx+len("returns"):]
	open2 := strings.Index(rest, "(")
	close2 := strings.Index(rest, ")")
	if open2 < 0 || close2 < 0 || close2 <= open2 {
		return req, ""
	}
	resp = strings.TrimSpace(rest[open2+1 : close2])
	return req, resp
}
