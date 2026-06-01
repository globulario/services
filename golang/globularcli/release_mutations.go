package main

import (
	"context"
	"encoding/json"
	"fmt"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// fetchRelease retrieves a ServiceRelease by name.
func fetchRelease(ctx context.Context, name string, client releaseResourcesClient) (*cluster_controllerpb.ServiceRelease, error) {
	rel, err := client.GetServiceRelease(ctx, &cluster_controllerpb.GetServiceReleaseRequest{Name: name}, jsonCallOption())
	if err != nil {
		return nil, err
	}
	if rel == nil || rel.Spec == nil || rel.Meta == nil {
		return nil, fmt.Errorf("release %s not found", name)
	}
	return rel, nil
}

// applyRelease writes the ServiceRelease back via ApplyServiceRelease.
func applyRelease(ctx context.Context, rel *cluster_controllerpb.ServiceRelease, client releaseResourcesClient) (*cluster_controllerpb.ServiceRelease, error) {
	return client.ApplyServiceRelease(ctx, &cluster_controllerpb.ApplyServiceReleaseRequest{Object: rel}, jsonCallOption())
}

// patchSpec merges fields from patch into rel.Spec (shallow, JSON merge semantics).
func patchSpec(rel *cluster_controllerpb.ServiceRelease, patch map[string]any) error {
	if rel.Spec == nil {
		rel.Spec = &cluster_controllerpb.ServiceReleaseSpec{}
	}
	current, err := json.Marshal(rel.Spec)
	if err != nil {
		return err
	}
	var base map[string]any
	if err := json.Unmarshal(current, &base); err != nil {
		return err
	}
	for k, v := range patch {
		base[k] = v
	}
	merged, err := json.Marshal(base)
	if err != nil {
		return err
	}
	var out cluster_controllerpb.ServiceReleaseSpec
	if err := json.Unmarshal(merged, &out); err != nil {
		return err
	}
	rel.Spec = &out
	return nil
}
