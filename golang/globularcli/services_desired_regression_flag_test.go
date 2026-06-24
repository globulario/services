package main

// services_desired_regression_flag_test.go — D4: the CLI half of invariant
// desired.no_regression_all_paths. `services desired set` exposes an explicit,
// audited --allow-regression override (NOT a generic --force); the flag must
// thread into UpsertDesiredServiceRequest.AllowRegression.

import "testing"

func TestDesiredSet_AllowRegressionFlagThreads(t *testing.T) {
	flag := servicesDesiredSetCmd.Flags().Lookup("allow-regression")
	if flag == nil {
		t.Fatal("`services desired set` must register a --allow-regression flag")
	}

	prev := svcDesiredSetAllowRegression
	t.Cleanup(func() {
		svcDesiredSetAllowRegression = prev
		_ = servicesDesiredSetCmd.Flags().Set("allow-regression", "false")
	})

	// Default must be false — a regression must be hard to ask for.
	if svcDesiredSetAllowRegression {
		t.Fatal("--allow-regression must default to false")
	}
	if err := servicesDesiredSetCmd.Flags().Set("allow-regression", "true"); err != nil {
		t.Fatalf("set --allow-regression: %v", err)
	}
	if !svcDesiredSetAllowRegression {
		t.Fatal("--allow-regression must bind to svcDesiredSetAllowRegression (threaded into the request)")
	}
}
