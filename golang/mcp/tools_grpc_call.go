package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func registerGrpcCallTools(s *server) {

	// ── grpc_call ───────────────────────────────────────────────────────────
	s.register(toolDef{
		Name: "grpc_call",
		Description: `Invoke any gRPC method on any running Globular service using server reflection.

Supports both unary and server-streaming RPCs. The request is specified as a JSON object
matching the proto message fields (use proto_describe to discover field names).

Examples:
- grpc_call(service="title.TitleService", method="SearchTitles", request={"query":"Coluche","indexPath":"/search/videos","size":5})
- grpc_call(service="file.FileService", method="GetFileInfo", request={"path":"/users/sa"})
- grpc_call(service="media.MediaService", method="ListMediaFiles", request={})
- grpc_call(service="title.TitleService", method="RebuildIndexFromStore", request={})

Note: streaming RPCs collect all responses into an array (max 100 messages).
Requires read_only=false for write operations.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"service":  {Type: "string", Description: "Full gRPC service name (e.g. 'title.TitleService')"},
				"method":   {Type: "string", Description: "Method name (e.g. 'SearchTitles', 'GetFileInfo')"},
				"request":  {Type: "string", Description: "JSON request body as a string (e.g. '{\"query\":\"test\"}')"},
				"endpoint": {Type: "string", Description: "Optional: direct endpoint (host:port). Default: gateway"},
				"timeout":  {Type: "number", Description: "Timeout in seconds (default: 30)"},
			},
			Required: []string{"service", "method"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		service := getStr(args, "service")
		method := getStr(args, "method")
		reqJSON := getStr(args, "request")
		endpoint := getStr(args, "endpoint")
		timeout := getInt(args, "timeout", 30)

		if service == "" || method == "" {
			return nil, fmt.Errorf("service and method are required")
		}
		if endpoint == "" {
			// Resolve direct service endpoint from etcd — reflection doesn't
			// work through the Envoy gateway (no route for reflection API).
			if ep, err := resolveServiceEndpoint(service); err == nil && ep != "" {
				endpoint = ep
			} else {
				endpoint = gatewayEndpoint()
			}
		}
		if reqJSON == "" {
			reqJSON = "{}"
		}

		// Also accept request as a raw object (not just string)
		if reqJSON == "{}" {
			if raw, ok := args["request"]; ok && raw != nil {
				switch v := raw.(type) {
				case map[string]interface{}:
					b, _ := json.Marshal(v)
					reqJSON = string(b)
				case string:
					reqJSON = v
				}
			}
		}

		conn, err := s.clients.get(ctx, endpoint)
		if err != nil {
			return nil, fmt.Errorf("connect to %s: %w", endpoint, err)
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), time.Duration(timeout)*time.Second)
		defer cancel()

		// Step 1: Get file descriptor via reflection
		refClient := grpc_reflection_v1alpha.NewServerReflectionClient(conn)
		stream, err := refClient.ServerReflectionInfo(callCtx)
		if err != nil {
			if isConnError(err) {
				s.clients.invalidate(endpoint)
			}
			return nil, fmt.Errorf("reflection: %w", err)
		}

		if err := stream.Send(&grpc_reflection_v1alpha.ServerReflectionRequest{
			MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: service,
			},
		}); err != nil {
			return nil, fmt.Errorf("reflection send: %w", err)
		}

		resp, err := stream.Recv()
		if err != nil {
			return nil, fmt.Errorf("reflection recv: %w", err)
		}
		stream.CloseSend()

		fdResp := resp.GetFileDescriptorResponse()
		if fdResp == nil {
			if errResp := resp.GetErrorResponse(); errResp != nil {
				return nil, fmt.Errorf("reflection error: %s", errResp.GetErrorMessage())
			}
			return nil, fmt.Errorf("unexpected reflection response")
		}

		// Step 2: Build file descriptors
		fdSet := &descriptorpb.FileDescriptorSet{}
		seen := make(map[string]bool)
		for _, raw := range fdResp.GetFileDescriptorProto() {
			fd := &descriptorpb.FileDescriptorProto{}
			if err := proto.Unmarshal(raw, fd); err != nil {
				continue
			}
			if !seen[fd.GetName()] {
				seen[fd.GetName()] = true
				fdSet.File = append(fdSet.File, fd)
			}
		}

		// Resolve dependencies — fetch any missing imports
		if err := resolveDependencies(callCtx, refClient, fdSet, seen); err != nil {
			// Non-fatal: some methods may still work
			_ = err
		}

		files, err := protodesc.NewFiles(fdSet)
		if err != nil {
			return nil, fmt.Errorf("build descriptors: %w", err)
		}

		// Step 3: Find the method
		svcDesc, err := files.FindDescriptorByName(protoreflect.FullName(service))
		if err != nil {
			return nil, fmt.Errorf("service %q not found: %w", service, err)
		}
		svcD, ok := svcDesc.(protoreflect.ServiceDescriptor)
		if !ok {
			return nil, fmt.Errorf("%q is not a service", service)
		}
		methodD := svcD.Methods().ByName(protoreflect.Name(method))
		if methodD == nil {
			return nil, fmt.Errorf("method %q not found in %s", method, service)
		}

		// Step 4: Build the request message
		inputMsg := dynamicpb.NewMessage(methodD.Input())
		unmarshaler := protojson.UnmarshalOptions{DiscardUnknown: true}
		if err := unmarshaler.Unmarshal([]byte(reqJSON), inputMsg); err != nil {
			return nil, fmt.Errorf("unmarshal request JSON: %w (use proto_describe to check field names)", err)
		}

		fullMethod := fmt.Sprintf("/%s/%s", service, method)
		marshaler := protojson.MarshalOptions{EmitUnpopulated: false, Indent: "  "}

		// Step 5: Call the method
		if methodD.IsStreamingServer() {
			// Server streaming
			streamDesc := &grpc.StreamDesc{
				StreamName:    method,
				ServerStreams:  true,
				ClientStreams:  methodD.IsStreamingClient(),
			}
			rpcStream, err := conn.NewStream(callCtx, streamDesc, fullMethod)
			if err != nil {
				return nil, fmt.Errorf("new stream: %w", err)
			}
			if err := rpcStream.SendMsg(inputMsg); err != nil {
				return nil, fmt.Errorf("send: %w", err)
			}
			if err := rpcStream.CloseSend(); err != nil {
				return nil, fmt.Errorf("close send: %w", err)
			}

			const maxMessages = 100
			responses := make([]json.RawMessage, 0)
			for i := 0; i < maxMessages; i++ {
				outMsg := dynamicpb.NewMessage(methodD.Output())
				if err := rpcStream.RecvMsg(outMsg); err != nil {
					if err == io.EOF {
						break
					}
					// Return what we have so far plus the error
					return map[string]interface{}{
						"service":        service,
						"method":         method,
						"streaming":      true,
						"message_count":  len(responses),
						"responses":      responses,
						"stream_error":   err.Error(),
					}, nil
				}
				b, _ := marshaler.Marshal(outMsg)
				responses = append(responses, json.RawMessage(b))
			}

			return map[string]interface{}{
				"service":       service,
				"method":        method,
				"streaming":     true,
				"message_count": len(responses),
				"responses":     responses,
			}, nil

		} else {
			// Unary
			outMsg := dynamicpb.NewMessage(methodD.Output())
			if err := conn.Invoke(callCtx, fullMethod, inputMsg, outMsg); err != nil {
				return nil, fmt.Errorf("invoke %s: %w", fullMethod, err)
			}
			b, err := marshaler.Marshal(outMsg)
			if err != nil {
				return nil, fmt.Errorf("marshal response: %w", err)
			}
			var parsed interface{}
			json.Unmarshal(b, &parsed)
			return map[string]interface{}{
				"service":  service,
				"method":   method,
				"response": parsed,
			}, nil
		}
	})
}

// resolveDependencies fetches missing file descriptors that the initial
// response depends on (e.g. google/protobuf/timestamp.proto, empty.proto).
func resolveDependencies(ctx context.Context, refClient grpc_reflection_v1alpha.ServerReflectionClient, fdSet *descriptorpb.FileDescriptorSet, seen map[string]bool) error {
	queue := make([]string, 0)
	for _, fd := range fdSet.File {
		for _, dep := range fd.GetDependency() {
			if !seen[dep] {
				queue = append(queue, dep)
				seen[dep] = true
			}
		}
	}

	for len(queue) > 0 {
		filename := queue[0]
		queue = queue[1:]

		stream, err := refClient.ServerReflectionInfo(ctx)
		if err != nil {
			return err
		}
		if err := stream.Send(&grpc_reflection_v1alpha.ServerReflectionRequest{
			MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_FileByFilename{
				FileByFilename: filename,
			},
		}); err != nil {
			stream.CloseSend()
			continue
		}
		resp, err := stream.Recv()
		stream.CloseSend()
		if err != nil {
			continue
		}
		fdResp := resp.GetFileDescriptorResponse()
		if fdResp == nil {
			continue
		}
		for _, raw := range fdResp.GetFileDescriptorProto() {
			fd := &descriptorpb.FileDescriptorProto{}
			if err := proto.Unmarshal(raw, fd); err != nil {
				continue
			}
			if !seen[fd.GetName()] {
				seen[fd.GetName()] = true
				fdSet.File = append(fdSet.File, fd)
				for _, dep := range fd.GetDependency() {
					if !seen[dep] {
						queue = append(queue, dep)
						seen[dep] = true
					}
				}
			}
		}
	}
	return nil
}

// resolveServiceEndpoint looks up a service's direct address:port from etcd config.
// Tries exact name first, then fuzzy prefix match (e.g. "title" matches "title.TitleService").
func resolveServiceEndpoint(serviceName string) (string, error) {
	all, err := config.GetServicesConfigurations()
	if err != nil {
		return "", err
	}
	// Exact match first, then prefix match
	for _, svc := range all {
		name, _ := svc["Name"].(string)
		if strings.EqualFold(name, serviceName) {
			return endpointFromConfig(svc), nil
		}
	}
	lower := strings.ToLower(serviceName)
	for _, svc := range all {
		name, _ := svc["Name"].(string)
		if strings.HasPrefix(strings.ToLower(name), lower+".") || strings.HasPrefix(strings.ToLower(name), lower) {
			return endpointFromConfig(svc), nil
		}
	}
	return "", fmt.Errorf("service %q not found in etcd", serviceName)
}

func endpointFromConfig(svc map[string]interface{}) string {
	port, _ := svc["Port"].(float64)
	addr, _ := svc["Address"].(string)
	if addr == "" {
		addr = "localhost"
	}
	// Strip any existing port from address to avoid "host:port:port".
	if host, _, err := net.SplitHostPort(addr); err == nil && host != "" {
		addr = host
	}
	if port > 0 {
		return fmt.Sprintf("%s:%d", addr, int(port))
	}
	return ""
}

// Ensure imports are used
var _ = strings.TrimSpace
