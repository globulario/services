package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/protobuf/proto"
	dpb "google.golang.org/protobuf/types/descriptorpb"
)

func registerProtoTools(s *server) {

	// ── proto_describe ──────────────────────────────────────────────────────
	s.register(toolDef{
		Name: "proto_describe",
		Description: `Describe a gRPC service or message using server reflection. Returns methods with their request/response types and field details.

Use this to check field names, types, and method signatures without reading proto files.

Examples:
- proto_describe(service="title.TitleService") — list all methods
- proto_describe(service="title.TitleService", method="SearchTitles") — describe one method with request/response fields
- proto_describe(service="file.FileService", method="ReadDir") — see ReadDir request fields`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"service":  {Type: "string", Description: "Full gRPC service name (e.g. 'title.TitleService', 'file.FileService')"},
				"method":   {Type: "string", Description: "Optional: specific method to describe in detail"},
				"endpoint": {Type: "string", Description: "Optional: direct endpoint (host:port). Default: gateway (localhost:443)"},
			},
			Required: []string{"service"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		service := getStr(args, "service")
		methodFilter := getStr(args, "method")
		endpoint := getStr(args, "endpoint")

		if service == "" {
			return nil, fmt.Errorf("service is required")
		}
		if endpoint == "" {
			endpoint = gatewayEndpoint()
		}

		conn, err := s.clients.get(ctx, endpoint)
		if err != nil {
			return nil, fmt.Errorf("connect to %s: %w", endpoint, err)
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		refClient := grpc_reflection_v1alpha.NewServerReflectionClient(conn)
		stream, err := refClient.ServerReflectionInfo(callCtx)
		if err != nil {
			if isConnError(err) {
				s.clients.invalidate(endpoint)
			}
			return nil, fmt.Errorf("reflection stream: %w", err)
		}

		// Request the file descriptor for this service
		if err := stream.Send(&grpc_reflection_v1alpha.ServerReflectionRequest{
			MessageRequest: &grpc_reflection_v1alpha.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: service,
			},
		}); err != nil {
			return nil, fmt.Errorf("send reflection request: %w", err)
		}

		resp, err := stream.Recv()
		if err != nil {
			return nil, fmt.Errorf("recv reflection response: %w", err)
		}

		fdResp := resp.GetFileDescriptorResponse()
		if fdResp == nil {
			errResp := resp.GetErrorResponse()
			if errResp != nil {
				return nil, fmt.Errorf("reflection error: %s", errResp.GetErrorMessage())
			}
			return nil, fmt.Errorf("unexpected reflection response")
		}

		// Parse all file descriptors
		msgTypes := make(map[string]*dpb.DescriptorProto)
		var targetService *dpb.ServiceDescriptorProto

		for _, fdBytes := range fdResp.GetFileDescriptorProto() {
			fd := &dpb.FileDescriptorProto{}
			if err := proto.Unmarshal(fdBytes, fd); err != nil {
				continue
			}
			pkg := fd.GetPackage()

			// Collect all message types
			for _, msg := range fd.GetMessageType() {
				fqn := pkg + "." + msg.GetName()
				msgTypes[fqn] = msg
				// Also collect nested types
				collectNested(pkg+"."+msg.GetName(), msg, msgTypes)
			}

			// Find the target service
			for _, svc := range fd.GetService() {
				fqn := pkg + "." + svc.GetName()
				if fqn == service {
					targetService = svc
				}
			}
		}

		if targetService == nil {
			return nil, fmt.Errorf("service %q not found in reflection response", service)
		}

		methods := make([]map[string]interface{}, 0, len(targetService.GetMethod()))
		for _, m := range targetService.GetMethod() {
			if methodFilter != "" && m.GetName() != methodFilter {
				continue
			}

			entry := map[string]interface{}{
				"name":             m.GetName(),
				"input_type":       shortName(m.GetInputType()),
				"output_type":      shortName(m.GetOutputType()),
				"client_streaming": m.GetClientStreaming(),
				"server_streaming": m.GetServerStreaming(),
			}

			// Describe input fields
			if inMsg, ok := msgTypes[strings.TrimPrefix(m.GetInputType(), ".")]; ok {
				entry["input_fields"] = describeFields(inMsg, msgTypes)
			}

			// Describe output fields (if specific method requested)
			if methodFilter != "" {
				if outMsg, ok := msgTypes[strings.TrimPrefix(m.GetOutputType(), ".")]; ok {
					entry["output_fields"] = describeFields(outMsg, msgTypes)
				}
			}

			methods = append(methods, entry)
		}

		sort.Slice(methods, func(i, j int) bool {
			return methods[i]["name"].(string) < methods[j]["name"].(string)
		})

		return map[string]interface{}{
			"service":      service,
			"method_count": len(methods),
			"methods":      methods,
		}, nil
	})
}

func collectNested(prefix string, msg *dpb.DescriptorProto, out map[string]*dpb.DescriptorProto) {
	for _, nested := range msg.GetNestedType() {
		fqn := prefix + "." + nested.GetName()
		out[fqn] = nested
		collectNested(fqn, nested, out)
	}
}

func shortName(fqn string) string {
	fqn = strings.TrimPrefix(fqn, ".")
	return fqn
}

func describeFields(msg *dpb.DescriptorProto, allMsgs map[string]*dpb.DescriptorProto) []map[string]interface{} {
	fields := make([]map[string]interface{}, 0, len(msg.GetField()))
	for _, f := range msg.GetField() {
		entry := map[string]interface{}{
			"name":   f.GetName(),
			"number": f.GetNumber(),
			"type":   fieldTypeName(f),
		}
		if f.GetLabel() == dpb.FieldDescriptorProto_LABEL_REPEATED {
			entry["repeated"] = true
		}
		if f.GetOneofIndex() != 0 || (f.OneofIndex != nil) {
			if int(*f.OneofIndex) < len(msg.GetOneofDecl()) {
				entry["oneof"] = msg.GetOneofDecl()[*f.OneofIndex].GetName()
			}
		}
		// For message/enum types, show the type name
		if f.GetTypeName() != "" {
			entry["message_type"] = shortName(f.GetTypeName())
		}
		fields = append(fields, entry)
	}
	return fields
}

func fieldTypeName(f *dpb.FieldDescriptorProto) string {
	switch f.GetType() {
	case dpb.FieldDescriptorProto_TYPE_STRING:
		return "string"
	case dpb.FieldDescriptorProto_TYPE_INT32, dpb.FieldDescriptorProto_TYPE_SINT32, dpb.FieldDescriptorProto_TYPE_SFIXED32:
		return "int32"
	case dpb.FieldDescriptorProto_TYPE_INT64, dpb.FieldDescriptorProto_TYPE_SINT64, dpb.FieldDescriptorProto_TYPE_SFIXED64:
		return "int64"
	case dpb.FieldDescriptorProto_TYPE_UINT32, dpb.FieldDescriptorProto_TYPE_FIXED32:
		return "uint32"
	case dpb.FieldDescriptorProto_TYPE_UINT64, dpb.FieldDescriptorProto_TYPE_FIXED64:
		return "uint64"
	case dpb.FieldDescriptorProto_TYPE_FLOAT:
		return "float"
	case dpb.FieldDescriptorProto_TYPE_DOUBLE:
		return "double"
	case dpb.FieldDescriptorProto_TYPE_BOOL:
		return "bool"
	case dpb.FieldDescriptorProto_TYPE_BYTES:
		return "bytes"
	case dpb.FieldDescriptorProto_TYPE_ENUM:
		return "enum"
	case dpb.FieldDescriptorProto_TYPE_MESSAGE:
		return "message"
	default:
		return f.GetType().String()
	}
}
