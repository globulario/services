package main

import (
	"context"
	"sort"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/domain"
	"github.com/globulario/services/golang/ai_memory/behavioral/store"
)

// behavioralDomainLoad reports the startup persistence result for one domain
// pack. Keeping the result explicit makes a missing/failed pack observable in
// tests and logs instead of collapsing it into an empty catalog later.
type behavioralDomainLoad struct {
	Name   string
	Result domain.LoadResult
	Err    error
}

// loadRegisteredBehavioralDomains persists every domain known to the same
// registry used by the runtime kernel. This is the important ownership rule:
// there must not be one list for runtime registration and another list for
// store seeding, because the two can drift while both paths appear healthy.
func loadRegisteredBehavioralDomains(ctx context.Context, st store.Store) []behavioralDomainLoad {
	if st == nil || st.Backend() == "unconfigured" {
		return nil
	}

	reg := behavioralRegistry()
	names := reg.Names()
	sort.Strings(names)

	loads := make([]behavioralDomainLoad, 0, len(names))
	for _, name := range names {
		pack, ok := reg.Lookup(name)
		if !ok || pack == nil {
			loads = append(loads, behavioralDomainLoad{Name: name, Err: store.ErrNotFound})
			continue
		}
		res, err := domain.LoadCatalogs(ctx, st, behavioralSeedProject, pack)
		loads = append(loads, behavioralDomainLoad{Name: name, Result: res, Err: err})
	}
	return loads
}

// seedRegisteredBehavioralDomains is the production startup wrapper. Domain
// seed is supplementary and therefore best-effort, but failures are loud: an
// empty catalog must never be mistaken for a domain with nothing to govern.
func seedRegisteredBehavioralDomains(st store.Store) {
	if st == nil || st.Backend() == "unconfigured" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, load := range loadRegisteredBehavioralDomains(ctx, st) {
		if load.Err != nil {
			logger.Warn("behavioral seed: domain pack load failed (non-fatal)",
				"domain", load.Name, "err", load.Err)
			continue
		}
		logger.Info("behavioral seed: domain pack loaded",
			"domain", load.Name,
			"authorities", load.Result.Authorities,
			"conditions", load.Result.Conditions,
			"principles_seeded", load.Result.PrinciplesSeeded,
			"principles_skipped", load.Result.PrinciplesSkipped)
	}
}
