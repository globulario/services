package main

// release_promote_local_plan_test.go — the promotion plan is operator-facing
// instructions. Printing a command that the CLI does not implement sends the
// operator into a dead end, and the failure surfaces only when they type it.
//
// `promote-local` shipped two commands that never existed —
// `globular release regenerate-bom` and `globular release validate-index`.
// This test makes that class of drift impossible: every `globular ...` command
// the plan prints must resolve against the real cobra command tree.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// planCommandTokens extracts the command-name tokens from one printed
// `globular ...` invocation, stopping at the first token that is a flag or a
// placeholder rather than a subcommand name.
func planCommandTokens(invocation string) []string {
	var out []string
	for _, tok := range strings.Fields(invocation) {
		if strings.HasPrefix(tok, "-") {
			break // flags begin; the command path is complete
		}
		// Placeholders and shell/values are arguments, not subcommands.
		if strings.ContainsAny(tok, "<>$=/.\"'") {
			break
		}
		if tok == "\\" {
			continue
		}
		out = append(out, tok)
	}
	return out
}

// extractGlobularInvocations returns every `globular ...` command found in the
// text, joining backslash continuation lines first.
func extractGlobularInvocations(text string) []string {
	joined := strings.ReplaceAll(text, "\\\n", " ")
	var found []string
	for _, line := range strings.Split(joined, "\n") {
		idx := strings.Index(line, "globular ")
		if idx < 0 {
			continue
		}
		found = append(found, strings.TrimSpace(line[idx+len("globular "):]))
	}
	return found
}

// resolveCommandPath walks tokens down the command tree. It returns the first
// token that should have been a subcommand but is not registered, or "" when
// the whole path resolves.
func resolveCommandPath(root *cobra.Command, tokens []string) string {
	cur := root
	for _, tok := range tokens {
		var child *cobra.Command
		for _, c := range cur.Commands() {
			if c.Name() == tok {
				child = c
				break
			}
			for _, alias := range c.Aliases {
				if alias == tok {
					child = c
					break
				}
			}
		}
		if child == nil {
			// A token that looks like a command name, under a command that has
			// subcommands, must be one of them. Otherwise it is a positional arg.
			if cur.HasSubCommands() {
				return tok
			}
			return ""
		}
		cur = child
	}
	return ""
}

func TestPromotionPlanCommandsAllExist(t *testing.T) {
	var buf bytes.Buffer
	printPromotionPlan(&buf, promotionPlanInput{
		ServiceName:      "storage",
		LocalPublisher:   "local@globule-ryzen",
		LocalVersion:     "1.2.288+local.1",
		BuildID:          "019e2eb5-0000-7000-8000-000000000001",
		BasedOn:          "1.2.288",
		OfficialVersion:  "1.2.289",
		ArtifactPlatform: "linux_amd64",
		ArtifactKind:     "service",
	})

	invocations := extractGlobularInvocations(buf.String())
	if len(invocations) == 0 {
		t.Fatal("promotion plan printed no globular commands — extraction is broken")
	}

	for _, inv := range invocations {
		tokens := planCommandTokens(inv)
		if len(tokens) == 0 {
			continue
		}
		if missing := resolveCommandPath(rootCmd, tokens); missing != "" {
			t.Errorf("promote-local prints `globular %s`, but %q is not a registered command",
				strings.Join(tokens, " "), missing)
		}
	}
}
