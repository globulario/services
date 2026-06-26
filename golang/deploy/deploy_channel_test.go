package deploy

import "testing"

// b2 of package.release_vs_dev_channel_boundary: a DEV-channel deploy publishes a
// dev-lane artifact but must NOT write cluster desired-state; release channels do.
func TestDeployUpdatesDesiredState_DevSkips(t *testing.T) {
	for _, dev := range []string{"dev", "DEV", " dev ", "Dev"} {
		if deployUpdatesDesiredState(dev) {
			t.Fatalf("channel %q is DEV — deploy must NOT update cluster desired-state", dev)
		}
	}
}

func TestDeployUpdatesDesiredState_ReleaseChannelsWrite(t *testing.T) {
	for _, rel := range []string{"stable", "candidate", "canary", "bootstrap", "", "STABLE"} {
		if !deployUpdatesDesiredState(rel) {
			t.Fatalf("channel %q is a release tier — deploy must update cluster desired-state", rel)
		}
	}
}
