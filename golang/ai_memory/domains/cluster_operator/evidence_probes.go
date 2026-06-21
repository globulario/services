package cluster_operator

// evidence_probes.go maps required-evidence refs to a probe SPEC. PR-5 provides
// the static mapping only — it does NOT execute probes, and the runtime hot path
// (CheckAction) never calls it. The behavioral gate evaluates already-recorded
// evidence and declared provided-evidence refs.
//
// Probe execution is deliberately left behind the EvidenceProber interface with
// no live wiring in PR-5; a later PR can supply a real prober (the only place
// cluster/probe clients would be imported).

// ProbeSpec describes how a required-evidence ref WOULD be gathered. It is data,
// not an action.
type ProbeSpec struct {
	Ref       string // the required-evidence ref this spec satisfies
	ProbeRef  string // logical probe identifier (e.g. "etcd.alarm_status")
	Lane      string // static_only | runtime_required | hybrid
}

// EvidenceProber executes a probe spec to gather evidence. PR-5 intentionally
// ships NO implementation — it defines the boundary so probe execution stays
// explicit and out of the kernel hot path.
type EvidenceProber interface {
	Probe(spec ProbeSpec) (result string, err error)
}

// EvidenceProbes returns the static ref→spec mapping built from the
// required-evidence catalog. No execution occurs.
func (p *Pack) EvidenceProbes() map[string]ProbeSpec {
	out := make(map[string]ProbeSpec, len(p.catalogs.RequiredEvidence))
	for _, e := range p.catalogs.RequiredEvidence {
		out[e.ID] = ProbeSpec{Ref: e.ID, ProbeRef: e.Fields["probe_ref"], Lane: e.Fields["lane"]}
	}
	return out
}
