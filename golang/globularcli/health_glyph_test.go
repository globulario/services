package main

import "testing"

// The CLI is an independent honesty boundary.
//
// The server in this build no longer emits status="healthy" together with a
// non-empty LastError, but the CLI must not depend on that. An older controller
// during a rolling upgrade, a mixed-version cluster, a replayed response, or a
// future server regression can all still produce that pair. If the glyph is
// derived from status alone, the original defect reappears here verbatim:
//
//	✅ a166b992… (node-3) — healthy
//	   Error: globular-file.service is inactive
//
// So nodeHealthGlyph consumes BOTH facts and downgrades on its own authority.
// These tests exercise the presentation layer directly; the seven tests in
// cluster_controller_server cover the response classification, which is a
// different boundary.
func TestNodeHealthGlyph(t *testing.T) {
	cases := []struct {
		name      string
		status    string
		lastError string
		wantIcon  string
		wantLabel string
	}{
		{"healthy with no error is green", "healthy", "", "✅", "healthy"},

		// THE TRAPDOOR: a server that still reports healthy alongside an error
		// must not be rendered green by this layer.
		{"healthy with active error is never green", "healthy", "globular-file.service is inactive", "⚠️", "degraded"},
		{"healthy with whitespace-only error is still green", "healthy", "   ", "✅", "healthy"},

		{"converging with error is converging, never green", "converging", "still installing", "🔄", "converging"},
		{"converging without error is still converging", "converging", "", "🔄", "converging"},
		{"degraded is warned", "degraded", "unit down", "⚠️", "degraded"},
		{"unknown is not green", "unknown", "not seen for 5m", "❔", "unknown"},
		{"unhealthy is red", "unhealthy", "connection refused", "❌", "unhealthy"},

		// Normalization: a server that varies case or pads must not fall into
		// the default red arm and mislabel a healthy node.
		{"case and padding normalized", "  HEALTHY  ", "", "✅", "healthy"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			icon, label := nodeHealthGlyph(tc.status, tc.lastError)
			if icon != tc.wantIcon {
				t.Errorf("icon = %q, want %q", icon, tc.wantIcon)
			}
			if label != tc.wantLabel {
				t.Errorf("label = %q, want %q", label, tc.wantLabel)
			}
		})
	}
}

// Only "healthy" may ever be green — stated as its own property so a future
// glyph addition cannot quietly widen the green set.
func TestNodeHealthGlyph_OnlyHealthyIsGreen(t *testing.T) {
	for _, status := range []string{"healthy", "converging", "degraded", "unknown", "unhealthy", "weird-new-status"} {
		for _, errText := range []string{"", "something is wrong"} {
			icon, _ := nodeHealthGlyph(status, errText)
			green := icon == "✅"
			shouldBeGreen := status == "healthy" && errText == ""
			if green != shouldBeGreen {
				t.Errorf("status=%q error=%q: green=%v, want %v", status, errText, green, shouldBeGreen)
			}
		}
	}
}
