package main

import (
    "context"
    "encoding/json"
    "fmt"
)

// fetchRelease retrieves a ServiceRelease by name.
func fetchRelease(ctx context.Context, name string, client releaseResourcesClient) (*clustercontrollerpb.ServiceRelease, error) {
    rel, err := client.GetServiceRelease(ctx, &clustercontrollerpb.GetServiceReleaseRequest{Name: name})
    if err != nil {
        return nil, err
    }
    if rel == nil || rel.Spec == nil || rel.Meta == nil {
        return nil, fmt.Errorf("release %s not found", name)
    }
    return rel, nil
}

// applyRelease writes the ServiceRelease back via ApplyServiceRelease.
func applyRelease(ctx context.Context, rel *clustercontrollerpb.ServiceRelease, client releaseResourcesClient) (*clustercontrollerpb.ServiceRelease, error) {
    return client.ApplyServiceRelease(ctx, &clustercontrollerpb.ApplyServiceReleaseRequest{Object: rel})
}

// patchSpec merges fields from patch into rel.Spec (shallow, JSON merge semantics).
func patchSpec(rel *clustercontrollerpb.ServiceRelease, patch map[string]any) error {
    if rel.Spec == nil {
        rel.Spec = &clustercontrollerpb.ServiceReleaseSpec{}
    }
    current, err := json.Marshal(rel.Spec)
    if err != nil {
        return err
    }
    // Decode into generic map for merge
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
    var out clustercontrollerpb.ServiceReleaseSpec
    if err := json.Unmarshal(merged, &out); err != nil {
        return err
    }
    rel.Spec = &out
    return nil
}

