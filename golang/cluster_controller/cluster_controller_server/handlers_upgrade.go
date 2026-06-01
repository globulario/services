// @awareness namespace=globular.platform
// @awareness component=platform_controller.workflow
// @awareness file_role=platform_upgrade_grpc_handlers
// @awareness implements=globular.platform:intent.release.bom_is_precise_release_authority
// @awareness risk=high
package main


// defaultPublisherID returns the publisher ID for desired-state operations.
func defaultPublisherID() string {
	return "core@globular.io"
}
