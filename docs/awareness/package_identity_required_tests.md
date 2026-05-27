# Package Identity Required Tests

Add or map real tests to these names. Do not list tests that do not exist in the repository.

## Core publish / upload tests

- `repository:TestUploadIdempotentSameDigestReturnsExistingBuildID`
- `repository:TestUploadDifferentDigestSamePublishedVersionRejected`
- `repository:TestPublishRejectsSameBuildNumberDifferentBuildID`
- `repository:TestVersionImmutabilityRunsAfterDigestIdempotency`

## Repository doctor tests

- `repository:TestRepositoryDoctorReportsDuplicateBuildNumberCollision`
- `repository:TestRepositoryDoctorReportsBuildIDReuse`
- `repository:TestRepositoryDoctorCollisionFindingIncludesForbiddenFixes`
- `repository:TestDesiredBuildIDOrphanedIncludesRepositoryRepairGuidance`

## Repair safety tests

- `repository:TestRepairArtifactUsesLatestExistingBuildWhenBroken`
- `repository:TestRepositoryRepairDoesNotRequireInstallableState`
- `repository:TestRepairDoesNotUseBuildNumberAsIdentity`
- `repository:TestCollisionRepairRefusesDesiredPinnedArtifact`
- `repository:TestCollisionRepairArchivesOnlyUnpinnedDuplicate`

## Runtime evidence tests

- `awareness_intentaudit:TestRepositoryIdentityRuntimeEvidencePass`
- `awareness_intentaudit:TestRepositoryIdentityRuntimeEvidenceFailsOnBuildNumberCollision`
- `awareness_intentaudit:TestRepositoryIdentityRuntimeEvidenceFailsOnBuildIDReuse`
- `awareness_intentaudit:TestRepositoryIdentityRuntimeEvidenceUnknownWhenRepositoryUnavailable`
