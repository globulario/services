package verifier

import "testing"

func TestDoctorRule_OldPidAfterUpgrade(t *testing.T) { TestVerifyTarget_ProcessOlderThanApply_OldPidAfterUpgrade(t) }
func TestDoctorRule_RunningBinaryHashMismatch(t *testing.T) {
	TestVerifyTarget_NewBinaryOldPid_RunningBinaryHashMismatch(t)
}
