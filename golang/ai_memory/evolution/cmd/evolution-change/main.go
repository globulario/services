package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/globulario/services/golang/ai_memory/evolution"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "init":
		cmdInit(os.Args[2:])
	case "bind-candidate":
		cmdBindCandidate(os.Args[2:])
	case "status":
		cmdStatus(os.Args[2:])
	case "validate":
		cmdValidate(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: evolution-change <init|bind-candidate|status|validate> [flags]")
}

func cmdInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	out := fs.String("out", "", "output ChangeEnvelope YAML/JSON")
	id := fs.String("id", "", "change id; empty mints one")
	kind := fs.String("kind", "", "incident_repair|simulation_repair|feature|architecture_evolution")
	intent := fs.String("intent", "", "behavioral intent")
	sourceRevision := fs.String("source-revision", "", "source/baseline repository revision")
	risk := fs.String("risk", "medium", "low|medium|high|critical")
	_ = fs.Parse(args)
	if *out == "" || *kind == "" || *intent == "" || *sourceRevision == "" {
		fmt.Fprintln(os.Stderr, "evolution-change init: --out, --kind, --intent, and --source-revision are required")
		os.Exit(2)
	}
	changeID := *id
	if changeID == "" {
		var err error
		changeID, err = evolution.NewChangeID()
		if err != nil {
			fatal(err)
		}
	}
	envelope := evolution.NewChangeEnvelope(
		changeID,
		evolution.ChangeKind(*kind),
		*intent,
		*sourceRevision,
		evolution.RiskClass(*risk),
	)
	if err := envelope.Validate(); err != nil {
		fatal(err)
	}
	// An envelope is the durable record of one candidate's proof, admission, and
	// release history. Initialising over an existing one would erase that history
	// in place rather than superseding it, so a fresh DRAFT never overwrites.
	if _, err := os.Stat(*out); err == nil {
		fatal(fmt.Errorf(
			"%s already exists; a new change needs its own envelope, and an existing one is superseded by a new candidate rather than overwritten",
			*out,
		))
	} else if !os.IsNotExist(err) {
		fatal(err)
	}
	if err := evolution.SaveChangeEnvelope(*out, envelope); err != nil {
		fatal(err)
	}
	printJSON(map[string]interface{}{"path": *out, "change_id": changeID, "stage": envelope.Stage})
}

func cmdBindCandidate(args []string) {
	fs := flag.NewFlagSet("bind-candidate", flag.ExitOnError)
	path := fs.String("envelope", "", "ChangeEnvelope YAML/JSON")
	repo := fs.String("repository", "", "candidate repository owner/name")
	revision := fs.String("revision", "", "exact candidate git revision")
	_ = fs.Parse(args)
	if *path == "" || *repo == "" || *revision == "" {
		fmt.Fprintln(os.Stderr, "evolution-change bind-candidate: --envelope, --repository, and --revision are required")
		os.Exit(2)
	}
	envelope, err := evolution.LoadChangeEnvelope(*path)
	if err != nil {
		fatal(err)
	}
	bound, err := evolution.RebindCandidate(*path, envelope.Identity(), *repo, *revision)
	if err != nil {
		fatal(err)
	}
	printJSON(bound.ProofStatus())
}

func cmdStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	path := fs.String("envelope", "", "ChangeEnvelope YAML/JSON")
	_ = fs.Parse(args)
	if *path == "" {
		fmt.Fprintln(os.Stderr, "evolution-change status: --envelope is required")
		os.Exit(2)
	}
	envelope, err := evolution.LoadChangeEnvelope(*path)
	if err != nil {
		fatal(err)
	}
	printJSON(envelope.ProofStatus())
}

func cmdValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	path := fs.String("envelope", "", "ChangeEnvelope YAML/JSON")
	_ = fs.Parse(args)
	if *path == "" {
		fmt.Fprintln(os.Stderr, "evolution-change validate: --envelope is required")
		os.Exit(2)
	}
	envelope, err := evolution.LoadChangeEnvelope(*path)
	if err != nil {
		fatal(err)
	}
	digest, err := envelope.IdentityDigest()
	if err != nil {
		fatal(err)
	}
	printJSON(map[string]interface{}{"valid": true, "change_id": envelope.ID, "stage": envelope.Stage, "identity_digest": digest})
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "evolution-change: %v\n", err)
	os.Exit(1)
}

func printJSON(value interface{}) {
	if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
		fatal(err)
	}
}
