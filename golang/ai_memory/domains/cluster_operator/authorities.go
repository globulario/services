package cluster_operator

// AuthorityIDs returns the sorted authority catalog ids defined by this pack.
func (p *Pack) AuthorityIDs() []string { return sortedIDs(p.catalogs.Authorities) }
