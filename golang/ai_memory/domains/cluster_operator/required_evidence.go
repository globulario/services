package cluster_operator

// RequiredEvidenceIDs returns the sorted required-evidence catalog ids.
func (p *Pack) RequiredEvidenceIDs() []string { return sortedIDs(p.catalogs.RequiredEvidence) }
