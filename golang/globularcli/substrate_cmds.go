package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/substrate"
)

// substrate commands implement the coordination-store survival contract:
// etcd must be recreatable from durable authority, bounded desired-state
// backup, and live observation. `dump` writes the bounded backup; `recover`
// walks the recovery ladder (restart-members → from-survivor → from-dump).

var (
	substrateCmd = &cobra.Command{
		Use:   "substrate",
		Short: "Coordination-store (etcd) survival: dump, recover, verify",
		Long: `Dump and recover the /globular coordination keyspace.

A dump captures the FULL keyspace with a classified manifest; a restore
applies the classification: identity/trust/audit keys restore as
authoritative, desired state restores as RESTORED_UNVERIFIED (convergence
must re-observe before destructive actions), observations are rebuilt by
their owners, and leases/locks/heartbeats/stale approvals are discarded.

Schedule 'globular substrate dump' from cron/systemd-timer on every
control-plane node so a recent dump is always available for rung 3.`,
	}

	substrateDumpDir string

	substrateDumpCmd = &cobra.Command{
		Use:   "dump",
		Short: "Write a classified logical dump of the /globular keyspace",
		Long: `Reads the full /globular keyspace with serializable (local) reads —
this works even on a quorum-less member — and writes an integrity-checked
dump file. Dumps contain operator secrets; files are created 0600.`,
		RunE: runSubstrateDump,
	}

	substrateRestartMembers bool
	substrateFromSurvivor   bool
	substrateFromDump       string
	substrateRecoverDumpDir string
	substrateRecoverDryRun  bool
	substrateRecoverForce   bool

	substrateRecoverCmd = &cobra.Command{
		Use:   "recover",
		Short: "Walk the recovery ladder: --restart-members | --from-survivor | --from-dump",
		Long: `Recovery ladder, in order of preference:

  --restart-members     rung 1: start stopped local substrate units
                        (globular-etcd, globular-node-agent). Existing member,
                        existing data — no data or membership mutation.
  --from-survivor       rung 2: this node holds the only surviving copy.
                        Takes a dump, backs up the data dir, restarts etcd
                        once with force-new-cluster (single voter, all data
                        kept), then hands back to the normal unit.
  --from-dump [file]    rung 3: import a classified dump into a fresh etcd.
                        Without a file, selects the best dump in --dir by
                        desired epoch (not timestamp).

Rungs 2 and 3 write a RESTORED_UNVERIFIED marker: restored desired state is
evidence, not authority, until reconciled against observation
('globular substrate mark-verified' attests that).`,
		RunE: runSubstrateRecover,
	}

	substrateStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show the recovery marker and substrate unit states",
		RunE:  runSubstrateStatus,
	}

	markVerifiedNote string

	substrateMarkVerifiedCmd = &cobra.Command{
		Use:   "mark-verified",
		Short: "Attest that restored state has been reconciled against observed reality",
		RunE:  runSubstrateMarkVerified,
	}
)

func init() {
	substrateDumpCmd.Flags().StringVar(&substrateDumpDir, "dir", substrate.DefaultDumpDir, "Directory to write the dump into")

	substrateRecoverCmd.Flags().BoolVar(&substrateRestartMembers, "restart-members", false, "Rung 1: start stopped local substrate units")
	substrateRecoverCmd.Flags().BoolVar(&substrateFromSurvivor, "from-survivor", false, "Rung 2: rebuild a single-voter cluster from this surviving member")
	substrateRecoverCmd.Flags().StringVar(&substrateFromDump, "from-dump", "", "Rung 3: dump file to import ('' with --dir selects the best dump)")
	substrateRecoverCmd.Flags().StringVar(&substrateRecoverDumpDir, "dir", substrate.DefaultDumpDir, "Dump directory for --from-dump selection")
	substrateRecoverCmd.Flags().BoolVar(&substrateRecoverDryRun, "dry-run", false, "Classify and report without writing (from-dump only)")
	substrateRecoverCmd.Flags().BoolVar(&substrateRecoverForce, "force", false, "Override cluster-UID/live-key guards (from-dump) or a failed pre-recovery dump (from-survivor)")
	substrateRecoverCmd.Flags().Lookup("from-dump").NoOptDefVal = "auto"

	substrateMarkVerifiedCmd.Flags().StringVar(&markVerifiedNote, "note", "", "Attestation note recorded in the marker")

	substrateCmd.AddCommand(substrateDumpCmd, substrateRecoverCmd, substrateStatusCmd, substrateMarkVerifiedCmd)
	rootCmd.AddCommand(substrateCmd)
}

// substrateKV returns the KV adapter. Serializable reads are the default so
// every read path here keeps working on a quorum-less member.
func substrateKV() (*substrate.EtcdKV, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("etcd client: %w", err)
	}
	return &substrate.EtcdKV{Client: cli, Serializable: true}, nil
}

func runSubstrateDump(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	kv, err := substrateKV()
	if err != nil {
		return err
	}
	dump, err := substrate.TakeDump(ctx, kv, true)
	if err != nil {
		return fmt.Errorf("dump: %w", err)
	}
	path, err := dump.WriteFile(substrateDumpDir)
	if err != nil {
		return err
	}
	fmt.Printf("dump written: %s\n", path)
	fmt.Printf("  cluster_uid:   %s\n", dump.Manifest.ClusterUID)
	fmt.Printf("  keys:          %d\n", dump.Manifest.KeyCount)
	fmt.Printf("  desired_epoch: %d\n", dump.Manifest.DesiredEpoch)
	fmt.Printf("  etcd_revision: %d\n", dump.Manifest.SourceEtcdRevision)
	return nil
}

func runSubstrateRecover(cmd *cobra.Command, args []string) error {
	modes := 0
	for _, on := range []bool{substrateRestartMembers, substrateFromSurvivor, substrateFromDump != ""} {
		if on {
			modes++
		}
	}
	if modes != 1 {
		return fmt.Errorf("choose exactly one of --restart-members, --from-survivor, --from-dump")
	}

	switch {
	case substrateRestartMembers:
		return recoverRestartMembersRun()
	case substrateFromSurvivor:
		return recoverFromSurvivorRun()
	default:
		return recoverFromDumpRun()
	}
}

func recoverRestartMembersRun() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	reports, err := substrate.RestartMembers(ctx, 60*time.Second)
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	for _, r := range reports {
		fmt.Fprintf(w, "%s\t%s\t%s\n", r.Unit, r.Action, r.Detail)
	}
	w.Flush()
	return err
}

func recoverFromSurvivorRun() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	return substrate.FromSurvivor(ctx, substrate.SurvivorOptions{
		Force: substrateRecoverForce,
		TakeDump: func(ctx context.Context) (string, error) {
			kv, err := substrateKV()
			if err != nil {
				return "", err
			}
			d, err := substrate.TakeDump(ctx, kv, true)
			if err != nil {
				return "", err
			}
			return d.WriteFile(substrateRecoverDumpDir)
		},
		ProbeHealthy: probeEtcdLinearizable,
		WriteMarker: func(ctx context.Context) error {
			kv, err := freshLinearizableKV()
			if err != nil {
				return err
			}
			return substrate.WriteMarker(ctx, kv, substrate.RestoreMarker{
				Status:     substrate.StatusRestoredUnverified,
				Mode:       "from-survivor-force-new-cluster",
				RestoredAt: time.Now().UTC().Format(time.RFC3339),
				Note:       "membership rewritten to single voter; other nodes must rejoin fresh",
			})
		},
	})
}

func recoverFromDumpRun() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	kv, err := substrateKV()
	if err != nil {
		return err
	}

	var dump *substrate.Dump
	path := substrateFromDump
	if path == "auto" || path == "" {
		liveUID := ""
		if kvp, _, err := kv.Get(ctx, substrate.ClusterIDKey); err == nil && kvp != nil {
			liveUID = string(kvp.Value)
		}
		path, dump, err = substrate.SelectLatestDump(substrateRecoverDumpDir, liveUID)
		if err != nil {
			return err
		}
		fmt.Printf("selected dump: %s (desired_epoch %d, created %s by %s)\n",
			path, dump.Manifest.DesiredEpoch, dump.Manifest.CreatedAt, dump.Manifest.CreatedByNode)
	} else {
		dump, err = substrate.ReadDumpFile(path)
		if err != nil {
			return err
		}
	}

	res, err := substrate.RestoreDump(ctx, kv, dump, substrate.RestoreOptions{
		Force:  substrateRecoverForce,
		DryRun: substrateRecoverDryRun,
		Note:   fmt.Sprintf("restored from %s", path),
	})
	if err != nil {
		return err
	}

	verb := "restored"
	if substrateRecoverDryRun {
		verb = "would restore"
	}
	fmt.Printf("%s: %d authoritative, %d unverified\n", verb,
		res.Restored[substrate.RestoreAuthoritative], res.Restored[substrate.RestoreAsUnverified])
	fmt.Printf("skipped: %d existing-live, %d leased, %d rebuild-from-observation, %d discard\n",
		res.SkippedExisting, res.SkippedLease,
		res.SkippedPolicy[substrate.RebuildFromObservation], res.SkippedPolicy[substrate.Discard])
	if len(res.UnknownPrefixes) > 0 {
		fmt.Printf("WARNING — unknown prefixes (classification-table gap, restored as unverified): %v\n", res.UnknownPrefixes)
	}
	if !substrateRecoverDryRun {
		fmt.Printf("marker: %s = %s — run convergence checks, then 'globular substrate mark-verified'\n",
			substrate.RestoreMarkerKey, substrate.StatusRestoredUnverified)
	}
	return nil
}

func runSubstrateStatus(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	kv, err := substrateKV()
	if err != nil {
		return err
	}
	marker, err := substrate.ReadMarker(ctx, kv)
	if err != nil {
		return fmt.Errorf("read marker: %w", err)
	}
	if marker == nil {
		fmt.Println("no recovery marker — the store was never restored")
		return nil
	}
	out, _ := json.MarshalIndent(marker, "", "  ")
	fmt.Println(string(out))
	return nil
}

func runSubstrateMarkVerified(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	kv, err := freshLinearizableKV()
	if err != nil {
		return err
	}
	m, err := substrate.MarkVerified(ctx, kv, markVerifiedNote)
	if err != nil {
		return err
	}
	fmt.Printf("marker: %s (verified at %s)\n", m.Status, m.VerifiedAt)
	return nil
}

// probeEtcdLinearizable proves quorum with a linearizable read on a fresh
// client — the cached singleton may hold connections from before a recovery
// restart.
func probeEtcdLinearizable(ctx context.Context) error {
	cli, err := config.NewEtcdClient()
	if err != nil {
		return err
	}
	defer cli.Close()
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err = cli.Get(probeCtx, substrate.ClusterIDKey) // linearizable by default
	return err
}

// freshLinearizableKV returns a non-serializable KV on a fresh client, for
// writes and post-recovery reads that must prove quorum.
func freshLinearizableKV() (*substrate.EtcdKV, error) {
	cli, err := config.NewEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("etcd client: %w", err)
	}
	return &substrate.EtcdKV{Client: cli, Serializable: false}, nil
}
