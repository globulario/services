package main

import (
	"context"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
)

// Desired state is stored only in the ResourceStore (ClusterNetwork/default, ServiceDesiredVersion/*).
// Legacy DesiredState persistence is removed; reconcile/health must not read legacy state.

// loadDesiredNetwork loads the ClusterNetwork/default object from the resource store.
func (srv *server) loadDesiredNetwork(ctx context.Context) (*clustercontrollerpb.ClusterNetwork, error) {
	if srv.resources == nil {
		return nil, nil
	}
	obj, _, err := srv.resources.Get(ctx, "ClusterNetwork", "default")
	if err != nil || obj == nil {
		return nil, err
	}
	cn, ok := obj.(*clustercontrollerpb.ClusterNetwork)
	if !ok {
		return nil, nil
	}
	return cn, nil
}

// loadDesiredServices loads all ServiceDesiredVersion resources and returns a canonical map.
func (srv *server) loadDesiredServices(ctx context.Context) (map[string]string, map[string]*clustercontrollerpb.ServiceDesiredVersion, error) {
	result := make(map[string]string)
	full := make(map[string]*clustercontrollerpb.ServiceDesiredVersion)
	if srv.resources == nil {
		return result, full, nil
	}
	items, _, err := srv.resources.List(ctx, "ServiceDesiredVersion", "")
	if err != nil {
		return result, full, err
	}
	for _, obj := range items {
		sdv, ok := obj.(*clustercontrollerpb.ServiceDesiredVersion)
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
