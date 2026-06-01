// @awareness namespace=globular.platform
// @awareness component=platform_cli
// @awareness file_role=cli_entrypoint
// @awareness risk=medium
package main

import "os"

func main() {
	if err := rootCmd.Execute(); err != nil {
		// Cobra already prints the error, but we must exit with non-zero code
		os.Exit(1)
	}
}
