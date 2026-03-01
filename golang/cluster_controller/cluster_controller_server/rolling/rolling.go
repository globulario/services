package rolling

type RollingPolicy struct {
	MaxUnavailable int
	Serial         bool
}

type NodeRollState struct {
	NodeID      string
	IsUpgrading bool
	IsHealthy   bool
}

// AdmitRolling returns whether a new upgrade can start under the policy.
func AdmitRolling(policy RollingPolicy, states []NodeRollState) (bool, string) {
	if policy.Serial {
		for _, st := range states {
			if st.IsUpgrading {
				return false, "serial policy: upgrade already in progress"
			}
		}
		return true, "allowed"
	}

	maxUnavailable := policy.MaxUnavailable
	if maxUnavailable <= 0 {
		maxUnavailable = 1
	}
	unavailable := 0
	for _, st := range states {
		if !st.IsHealthy || st.IsUpgrading {
			unavailable++
		}
	}
	if unavailable >= maxUnavailable {
		return false, "max unavailable reached"
	}
	return true, "allowed"
}
