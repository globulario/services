package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func nodeIdentityFromValues(values interface{}) *resourcepb.NodeIdentity {
	valuesMap, _ := values.(map[string]interface{})
	if valuesMap == nil {
		return nil
	}
	node := &resourcepb.NodeIdentity{
		NodeId:            Utility.ToString(valuesMap["_id"]),
		Hostname:          Utility.ToString(valuesMap["hostname"]),
		Domain:            Utility.ToString(valuesMap["domain"]),
		ExternalIpAddress: Utility.ToString(valuesMap["external_ip_address"]),
		LocalIpAddress:    Utility.ToString(valuesMap["local_ip_address"]),
		Mac:               Utility.ToString(valuesMap["mac"]),
		Protocol:          Utility.ToString(valuesMap["protocol"]),
		PortHttp:          int32(Utility.ToInt(valuesMap["PortHttp"])),
		PortHttps:         int32(Utility.ToInt(valuesMap["PortHttps"])),
		Fingerprint:       Utility.ToString(valuesMap["fingerprint"]),
		LastSeen:          int64(Utility.ToNumeric(valuesMap["last_seen"])),
		Enabled:           Utility.ToBool(valuesMap["enabled"]),
		Labels:            labelsFromInterface(valuesMap["labels"]),
		Status:            Utility.ToString(valuesMap["status"]),
		TypeName:          Utility.ToString(valuesMap["typeName"]),
	}
	if node.NodeId == "" {
		node.NodeId = Utility.ToString(valuesMap["node_id"])
	}
	return node
}

func nodeIdentityToDocument(node *resourcepb.NodeIdentity) map[string]interface{} {
	if node == nil {
		return map[string]interface{}{}
	}
	doc := map[string]interface{}{
		"_id":                 node.NodeId,
		"node_id":             node.NodeId,
		"hostname":            node.Hostname,
		"domain":              node.Domain,
		"external_ip_address": node.ExternalIpAddress,
		"local_ip_address":    node.LocalIpAddress,
		"mac":                 node.Mac,
		"protocol":            node.Protocol,
		"PortHttp":            node.PortHttp,
		"PortHttps":           node.PortHttps,
		"fingerprint":         node.Fingerprint,
		"last_seen":           node.LastSeen,
		"enabled":             node.Enabled,
		"status":              node.Status,
		"typeName":            node.TypeName,
	}
	if len(node.Labels) > 0 {
		labels := make(map[string]interface{}, len(node.Labels))
		for key, value := range node.Labels {
			labels[key] = value
		}
		doc["labels"] = labels
	}
	return doc
}

func labelsFromInterface(values interface{}) map[string]string {
	result := make(map[string]string)
	switch typed := values.(type) {
	case map[string]interface{}:
		for key, value := range typed {
			result[key] = Utility.ToString(value)
		}
	case map[string]string:
		for key, value := range typed {
			result[key] = value
		}
	case map[interface{}]interface{}:
		for key, value := range typed {
			result[Utility.ToString(key)] = Utility.ToString(value)
		}
	case string:
		m := make(map[string]string)
		if err := json.Unmarshal([]byte(typed), &m); err == nil {
			return m
		}
	}
	return result
}

func (srv *server) UpsertNodeIdentity(ctx context.Context, rqst *resourcepb.UpsertNodeIdentityRqst) (*resourcepb.UpsertNodeIdentityRsp, error) {
	if rqst == nil || rqst.Node == nil {
		return nil, status.Error(codes.InvalidArgument, "node identity required")
	}

	node := rqst.Node
	if node.NodeId == "" {
		node.NodeId = Utility.GenerateUUID(node.Mac)
	}
	if node.LastSeen == 0 {
		node.LastSeen = time.Now().Unix()
	}
	if node.TypeName == "" {
		node.TypeName = "NodeIdentity"
	}

	doc := nodeIdentityToDocument(node)
	jsonStr, err := Utility.ToJson(doc)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := fmt.Sprintf(`{"_id":"%s"}`, node.NodeId)
	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", nodeIdentityCollection, q, jsonStr, `[{"upsert":true}]`)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.UpsertNodeIdentityRsp{Node: node}, nil
}

func (srv *server) GetNodeIdentity(ctx context.Context, rqst *resourcepb.GetNodeIdentityRqst) (*resourcepb.GetNodeIdentityRsp, error) {
	if rqst == nil || rqst.NodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := fmt.Sprintf(`{"_id":"%s"}`, rqst.NodeId)
	data, err := p.FindOne(context.Background(), "local_resource", "local_resource", nodeIdentityCollection, q, "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	if data == nil {
		return nil, status.Errorf(codes.NotFound, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("node identity not found")))
	}

	return &resourcepb.GetNodeIdentityRsp{Node: nodeIdentityFromValues(data)}, nil
}

func (srv *server) ListNodeIdentities(rqst *resourcepb.ListNodeIdentitiesRqst, stream resourcepb.ResourceService_ListNodeIdentitiesServer) error {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	query := rqst.Query
	if query == "" {
		query = "{}"
	}

	values, err := p.Find(context.Background(), "local_resource", "local_resource", nodeIdentityCollection, query, rqst.Options)
	if err != nil {
		return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	maxSize := 100
	batch := make([]*resourcepb.NodeIdentity, 0, maxSize)
	for _, item := range values {
		identity := nodeIdentityFromValues(item)
		if identity == nil {
			continue
		}
		batch = append(batch, identity)
		if len(batch) >= maxSize {
			if err := stream.Send(&resourcepb.ListNodeIdentitiesRsp{Nodes: batch}); err != nil {
				return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := stream.Send(&resourcepb.ListNodeIdentitiesRsp{Nodes: batch}); err != nil {
			return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return nil
}

func (srv *server) SetNodeIdentityEnabled(ctx context.Context, rqst *resourcepb.SetNodeIdentityEnabledRqst) (*resourcepb.SetNodeIdentityEnabledRsp, error) {
	if rqst == nil || rqst.NodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := fmt.Sprintf(`{"_id":"%s"}`, rqst.NodeId)
	update := fmt.Sprintf(`{"$set":{"enabled":%t}}`, rqst.Enabled)
	err = p.UpdateOne(context.Background(), "local_resource", "local_resource", nodeIdentityCollection, q, update, "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.SetNodeIdentityEnabledRsp{Result: true}, nil
}
