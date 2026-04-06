package installed_state

import (
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// This file exists ONLY to anchor the `+globular:schema:` pragma on the
// generated proto type node_agentpb.InstalledPackage, which cannot
// itself carry pragmas because it is regenerated from the .proto file.
// The schema-extractor walks Go source for pragma blocks and doesn't
// care whether the attached type is a struct definition or a type
// alias — it records the etcd key/writer/readers metadata either way.

// InstalledPackageRecord is a schema-anchor alias for
// node_agentpb.InstalledPackage as it is persisted in etcd. The actual
// struct is generated from node_agent.proto; this alias is here purely
// so the pragma parser has a Go type to attach the schema metadata to.
//
// +globular:schema:key="/globular/nodes/{node_id}/packages/{kind}/{name}"
// +globular:schema:writer="globular-node-agent"
// +globular:schema:readers="globular-cluster-controller,globular-gateway,globular-cluster-doctor,globular-repository"
// +globular:schema:description="Per-node installed-package record — one entry per (node, kind, name) tuple."
// +globular:schema:invariants="Written only after a successful install/upgrade/remove lifecycle transition; deleted by DeleteInstalledPackage or node cleanup; kind MUST be one of SERVICE|INFRASTRUCTURE|COMMAND."
type InstalledPackageRecord = node_agentpb.InstalledPackage
