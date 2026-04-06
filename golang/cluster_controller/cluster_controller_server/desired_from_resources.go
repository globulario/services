package main

import (
	"context"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// Desired state is stored only in the ResourceStore (ClusterNetwork/default, ServiceDesiredVersion/*).
// Legacy DesiredState persistence is removed; reconcile/health must not read legacy state.

// loadDesiredNetwork loads the ClusterNetwork/default object from the resource store.
func (srv *server) loadDesiredNetwork(ctx context.Context) (*cluster_controllerpb.ClusterNetwork, error) {
	if srv.resources == nil {
		return nil, nil
	}
	obj, _, err := srv.resources.Get(ctx, "ClusterNetwork", "default")
	if err != nil || obj == nil {
		return nil, err
	}
	cn, ok := obj.(*cluster_controllerpb.ClusterNetwork)
	if !ok {
		return nil, nil
	}
	return cn, nil
}

// loadDesiredServices loads all ServiceDesiredVersion resources and returns a canonical map.
func (srv *server) loadDesiredServices(ctx context.Context) (map[string]string, map[string]*cluster_controllerpb.ServiceDesiredVersion, error) {
	result := make(map[string]string)
	full := make(map[string]*cluster_controllerpb.ServiceDesiredVersion)
	if srv.resources == nil {
		return result, full, nil
	}
	items, _, err := srv.resources.List(ctx, "ServiceDesiredVersion", "")
	if err != nil {
		return result, full, err
	}
	for _, obj := range items {
		sdv, ok := obj.(*cluster_controllerpb.ServiceDesiredVersion)
		if !ok || sdv.Spec == nil {
			continue
		}
		canon := canonicalServiceName(sdv.Spec.ServiceName)
		if canon == "" && sdv.Meta != nil {
			canon = canonicalServiceName(sdv.Meta.Name)
		}
		if canon == "" {
			continue
		}
		result[canon] = sdv.Spec.Version
		full[canon] = sdv
	}
	return result, full, nil
}

// mergeInfraDesiredInto loads InfrastructureRelease resources and merges
// them into the given desired map. This ensures infrastructure packages
// participate in drift detection alongside services.
func (srv *server) mergeInfraDesiredInto(ctx context.Context, desired map[string]string) {
	if srv.resources == nil {
		return
	}
	items, _, err := srv.resources.List(ctx, "InfrastructureRelease", "")
	if err != nil {
		return
	}
	for _, obj := range items {
		rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease)
		if !ok || rel.Spec == nil || rel.Spec.Component == "" {
			continue
		}
		canon := canonicalServiceName(rel.Spec.Component)
		if canon == "" {
			continue
		}
		// Don't overwrite service desired if both exist — service takes priority.
		if _, exists := desired[canon]; !exists {
			desired[canon] = rel.Spec.Version
		}
	}
}
