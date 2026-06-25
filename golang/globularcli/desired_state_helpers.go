package main

// desired_state_helpers.go — direct-etcd helpers for desired-state writes.
//
// platform-upgrade used to call upsertServiceDesiredVersion in a loop
// over every BOM package — that was the v1.2.159 incident. As of
// v1.2.160 platform-upgrade dispatches the platform.upgrade workflow
// instead, which writes ServiceDesiredVersion only after gated
// per-(node, package) decisions.
//
// These helpers remain because they're still used for single-package,
// operator-driven flows:
//   - upsertServiceDesiredVersion: pkg override apply/restore
//     (golang/globularcli/pkg_override_cmds.go) — explicit, one
//     package at a time, with an operator-supplied build_id.
//   - updateInfraReleaseVersion: `globular release set-version`
//     (golang/globularcli/release_cmds.go) — single-infra-package
//     equivalent of platform-upgrade, kept because the infra side
//     still has no per-(node, package) workflow yet.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/grpc"

	"github.com/globulario/services/golang/audittrail"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
)

// upsertServiceDesiredVersion writes a ServiceDesiredVersion record
// directly to etcd. publisherID may be empty (defaults to
// core@globular.io for official builds) or set to a local publisher
// (e.g. local@ryzen) when activating a local override.
//
// NOTE: this is NOT used by platform-upgrade anymore (see header). It
// remains for single-package overrides where the operator has
// explicitly chosen a (name, version, build_id) tuple.
func upsertServiceDesiredVersion(serviceName, publisherID, version string, buildNumber int64, buildID string) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}

	key := "/globular/resources/ServiceDesiredVersion/" + serviceName
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := cli.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("etcd get %s: %w", key, err)
	}

	var rec map[string]interface{}
	generation := float64(1)
	if len(resp.Kvs) > 0 {
		if err := json.Unmarshal(resp.Kvs[0].Value, &rec); err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}
		if m, ok := rec["meta"].(map[string]interface{}); ok {
			if g, ok := m["generation"].(float64); ok {
				generation = g + 1
			}
		}
	} else {
		rec = map[string]interface{}{
			"meta":   map[string]interface{}{},
			"spec":   map[string]interface{}{},
			"status": map[string]interface{}{},
		}
	}

	rec["meta"] = map[string]interface{}{
		"name":       serviceName,
		"generation": generation,
	}
	spec := map[string]interface{}{
		"service_name": serviceName,
		"version":      version,
	}
	if buildNumber > 0 {
		spec["build_number"] = buildNumber
	}
	if buildID != "" {
		spec["build_id"] = buildID
	}
	if publisherID != "" {
		spec["publisher_id"] = publisherID
	}
	rec["spec"] = spec

	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	_, err = cli.Put(ctx, key, string(data))
	if err != nil {
		return fmt.Errorf("etcd put %s: %w", key, err)
	}
	_ = audittrail.WriteDesiredWriteRecord(ctx, audittrail.DesiredWriteRecord{
		Service:   serviceName,
		Actor:     "operator-cli",
		Source:    "upsertServiceDesiredVersion",
		Action:    "upsert_desired",
		Reason:    "authoritative desired-state update via CLI",
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	})
	return nil
}

// updateInfraReleaseVersion updates the spec.version of an
// InfrastructureRelease record in etcd. Used by `globular release
// set-version` for single-infra-package version pinning.
func updateInfraReleaseVersion(publisher, component, version string) error {
	conn, err := controllerConnFactory()
	if err != nil {
		return fmt.Errorf("connect to controller: %w", err)
	}
	if c, ok := conn.(*grpc.ClientConn); ok && c != nil {
		defer func() { _ = c.Close() }()
	}
	client := resourcesClientFactory(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	name := publisher + "/" + component

	// GET-modify-Apply through the owner's typed RPC. The previous implementation
	// raw-wrote /globular/resources/InfrastructureRelease/... directly to etcd,
	// bypassing the cluster-controller (the registered owner) and its desired-state
	// ownership guard. Route through ApplyInfrastructureRelease so the write is
	// governed (generation bump, ownership check, reconcile) instead of a raw put.
	// Read the current release first to preserve sibling spec fields (build_number,
	// channel, …) and change only the version — matching the prior read-modify-write.
	obj, err := client.GetInfrastructureRelease(ctx,
		&cluster_controllerpb.GetInfrastructureReleaseRequest{Name: name}, jsonCallOption())
	if err != nil || obj == nil {
		// Not present yet (or unreadable): create a fresh release.
		obj = &cluster_controllerpb.InfrastructureRelease{}
	}
	if obj.Meta == nil {
		obj.Meta = &cluster_controllerpb.ObjectMeta{}
	}
	obj.Meta.Name = name
	if obj.Spec == nil {
		obj.Spec = &cluster_controllerpb.InfrastructureReleaseSpec{}
	}
	obj.Spec.PublisherID = publisher
	obj.Spec.Component = component
	obj.Spec.Version = version

	if _, err := client.ApplyInfrastructureRelease(ctx,
		&cluster_controllerpb.ApplyInfrastructureReleaseRequest{Object: obj}, jsonCallOption()); err != nil {
		return fmt.Errorf("apply infrastructure release %s: %w", name, err)
	}
	return nil
}
