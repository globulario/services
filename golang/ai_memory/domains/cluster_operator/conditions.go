package cluster_operator

// ConditionIDs returns the sorted condition catalog ids defined by this pack.
func (p *Pack) ConditionIDs() []string { return sortedIDs(p.catalogs.Conditions) }
