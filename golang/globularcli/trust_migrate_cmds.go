package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/repository/repository_client"
)

var trustMigrateCmd = &cobra.Command{
	Use:   "trust-migrate",
	Short: "Migrate existing packages into the trust model",
	Long: `Migrate existing artifacts into the trust model:
  1. Create namespaces for all existing publishers
  2. Generate synthetic provenance for legacy artifacts
  3. Set explicit publish state for artifacts without one

This is normally done automatically on repository service startup.
Use this command to check status or run migration manually.`,
}

var trustMigrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show trust migration status",
	RunE:  runTrustMigrateStatus,
}

func init() {
	trustMigrateCmd.AddCommand(trustMigrateStatusCmd)
	pkgCmd.AddCommand(trustMigrateCmd)
}

func runTrustMigrateStatus(cmd *cobra.Command, args []string) error {
	address, _ := config.GetAddress()
	if address == "" {
		address = "localhost"
	}

	client, err := repository_client.NewRepositoryService_Client(address, "repository.PackageRepository")
	if err != nil {
		return fmt.Errorf("connect to repository: %w", err)
	}
	defer client.Close()

	artifacts, err := client.ListArtifacts()
	if err != nil {
		return fmt.Errorf("list artifacts: %w", err)
	}

	publishers := make(map[string]int)
	withProvenance := 0
	withState := 0

	for _, a := range artifacts {
		if a.GetRef() != nil {
			publishers[a.GetRef().GetPublisherId()]++
		}
		if a.GetProvenance() != nil {
			withProvenance++
		}
		if a.GetPublishState() != 0 {
			withState++
		}
	}

	fmt.Printf("Trust Model Migration Status\n")
	fmt.Printf("  Total artifacts:      %d\n", len(artifacts))
	fmt.Printf("  With provenance:      %d\n", withProvenance)
	fmt.Printf("  With explicit state:  %d\n", withState)
	fmt.Printf("  Publishers:\n")
	for pub, count := range publishers {
		if pub == "" {
			pub = "(empty)"
		}
		fmt.Printf("    %-30s %d artifacts\n", pub, count)
	}

	// Check namespace ownership.
	fmt.Println("\nNamespace ownership status:")
	for pub := range publishers {
		if strings.TrimSpace(pub) == "" {
			continue
		}
		resp, err := client.GetNamespace(pub)
		if err != nil || resp == nil || resp.GetNamespace() == nil {
			fmt.Printf("  %-30s NOT CLAIMED\n", pub)
		} else {
			ns := resp.GetNamespace()
			fmt.Printf("  %-30s owners: %s\n", pub, strings.Join(ns.GetOwners(), ", "))
		}
	}

	return nil
}
