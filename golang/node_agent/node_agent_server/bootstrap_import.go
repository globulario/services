// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.bootstrap
// @awareness file_role=day0_bootstrap_import_workflow
// @awareness implements=globular.platform:intent.day0_day1_are_repeatable_ceremonies
// @awareness risk=high
package main

// bootstrap_import.go — provisional artifact import (removed).
//
// Packages are now distributed via the gateway's /join/packages/ endpoint
// and stored locally on each node before the node-agent starts. There is no
// longer a publish-to-MinIO step — the local filesystem is the sole authority
// for package bytes on each node.

import "context"

// importProvisionalPackages is a no-op. Package distribution is handled by
// the join script downloading packages from the bootstrap gateway.
func (srv *NodeAgentServer) importProvisionalPackages(_ context.Context) {}
