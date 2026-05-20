package main

import (
	"encoding/json"

	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/mem"
)

// jsonCodecV2 is a gRPC CodecV2 that marshals/unmarshals using JSON.
// The cluster controller's ResourcesService uses plain Go structs (not
// protobuf-generated), so both server and client must use this codec.
//
// We implement CodecV2 directly (not the v1 Codec) to avoid the v1→v2 bridge
// in gRPC v1.78 which may not propagate ForceCodec correctly.
type jsonCodecV2 struct{}

func (jsonCodecV2) Marshal(v any) (mem.BufferSlice, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return mem.BufferSlice{mem.SliceBuffer(data)}, nil
}

func (jsonCodecV2) Unmarshal(data mem.BufferSlice, v any) error {
	return json.Unmarshal(data.Materialize(), v)
}

func (jsonCodecV2) Name() string { return "json" }

func init() { encoding.RegisterCodecV2(jsonCodecV2{}) }

// jsonCallOption returns a grpc.CallOption that forces JSON encoding
// for ResourcesService RPCs.
func jsonCallOption() grpc.CallOption {
	return grpc.ForceCodecV2(jsonCodecV2{})
}
