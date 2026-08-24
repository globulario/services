package main

import (
	"errors"
	"testing"
)

// TestIsConsensusUnavailable pins that a lease refusal caused by lost storage
// quorum is recognised as such.
//
// The bare driver error reads "Cannot achieve consistency level for cl SERIAL.
// Requires 2, alive 1". An operator running `cluster nodes remove --force` on a
// down node saw only that, wrapped in "lease claim failed — refusing unfenced
// execution", with nothing connecting it to storage quorum. Observed
// 2026-08-23: the removal was refused, the node's controller record survived,
// and its clean rejoin was then denied with "node identity conflict: hostname
// already present" — the node stayed out of the cluster until storage
// recovered, and nothing in the output said why.
func TestIsConsensusUnavailable(t *testing.T) {
	t.Run("recognises the quorum-loss shapes", func(t *testing.T) {
		for _, msg := range []string{
			"Cannot achieve consistency level for cl SERIAL. Requires 2, alive 1",
			"cannot achieve consistency level for cl SERIAL",
			"Not enough replicas available for query at consistency SERIAL",
		} {
			if !isConsensusUnavailable(errors.New(msg)) {
				t.Errorf("must recognise a quorum-loss refusal: %q", msg)
			}
		}
	})

	t.Run("does not swallow unrelated failures", func(t *testing.T) {
		for _, msg := range []string{
			"context deadline exceeded",
			"connection refused",
			"unconfigured table workflow.executor_leases",
			"",
		} {
			if isConsensusUnavailable(errors.New(msg)) {
				t.Errorf("must not misattribute an unrelated error to quorum loss: %q", msg)
			}
		}
		if isConsensusUnavailable(nil) {
			t.Error("nil must not be reported as a consensus failure")
		}
	})
}
