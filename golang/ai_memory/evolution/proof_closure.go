package evolution

import "fmt"

// ValidateRequiredTestClosure verifies only the local/static test leg of proof.
// It is used before cluster simulation so expensive/destructive lab work cannot
// outrun the cheaper exact-revision tests declared by the ChangeEnvelope.
func (e ChangeEnvelope) ValidateRequiredTestClosure() error {
	required := map[string]TestRequirement{}
	for _, requirement := range e.RequiredTests {
		if requirement.Required {
			required[requirement.Name] = requirement
		}
	}
	if len(required) == 0 {
		return nil
	}
	satisfied := map[string]bool{}
	for _, test := range e.Tests {
		requirement, ok := required[test.Name]
		if !ok || test.Result != "PASS" {
			continue
		}
		if test.CandidateRepository != "" && test.CandidateRepository != e.CandidateRepository {
			return fmt.Errorf(
				"test %q candidate repository %q does not match %q",
				test.Name,
				test.CandidateRepository,
				e.CandidateRepository,
			)
		}
		if test.CandidateRevision != e.CandidateRevision {
			return fmt.Errorf(
				"test %q candidate revision %q does not match %q",
				test.Name,
				test.CandidateRevision,
				e.CandidateRevision,
			)
		}
		if test.PlanDigest != e.PlanDigest {
			return fmt.Errorf(
				"test %q plan digest %q does not match %q",
				test.Name,
				test.PlanDigest,
				e.PlanDigest,
			)
		}
		if !stringSlicesEqual(test.Command, requirement.Command) {
			return fmt.Errorf("test %q command does not match declared requirement", test.Name)
		}
		satisfied[test.Name] = true
	}
	for name := range required {
		if !satisfied[name] {
			return fmt.Errorf(
				"required test %q is not proven for candidate revision %s plan %s",
				name,
				e.CandidateRevision,
				e.PlanDigest,
			)
		}
	}
	return nil
}
