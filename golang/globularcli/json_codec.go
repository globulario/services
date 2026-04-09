package main

import (
	"encoding/json"

	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
)

// jsonCodec is a gRPC codec that marshals/unmarshals using JSON.
// The cluster controller's ResourcesService uses plain Go structs (not
// protobuf-generated), so both server and client must use this codec.
type jsonCodec struct{}

func (jsonCodec) Marshal(v interface{}) ([]byte, error)      { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v interface{}) error  { return json.Unmarshal(data, v) }
func (jsonCodec) Name() string                                { return "json" }

func init() { encoding.RegisterCodec(jsonCodec{}) }

// jsonCallOption returns a grpc.CallOption that forces JSON encoding
// for ResourcesService RPCs.
func jsonCallOption() grpc.CallOption {
	return grpc.CallContentSubtype("json")
}
