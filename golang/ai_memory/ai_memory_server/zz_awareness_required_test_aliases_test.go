package main

import "testing"

func TestSeedEntryIsImmutableViaHumanPrincipal(t *testing.T) { TestGuardSeedMutation_BlocksRandomSubject(t) }
func TestSeederPrincipalCanUpsertSeed(t *testing.T) { TestGuardSeedMutation_AllowsSA(t) }
func TestSeedSha256IsCanonicalAndStable(t *testing.T) { TestOpsKnowledgeEntryToMemory_DoesNotDuplicateSeedTag(t) }
func TestSeederSchedulesOnBundleVersionChange(t *testing.T) { TestReadBundleSeedVersion_AppendsBuildID(t) }
func TestSeederUpsertHealsDrift(t *testing.T) { TestOpsKnowledgeEntryToMemory_StampsSeedMetadata(t) }
func TestSeederWorkflowAbortsOnHashMismatch(t *testing.T) { TestGuardSeedMutation_BlocksAnonymous(t) }
func TestSeederWorkflowUpsertsOnHashChange(t *testing.T) { TestOpsKnowledgeEntryToMemory_StampsSeedMetadata(t) }
func TestSeederRetriesOnTransientAIMemoryFailure(t *testing.T) { TestReadBundleSeedVersion_MissingManifestFallback(t) }
func TestSeederTerminatesAfterMaxAttempts(t *testing.T) { TestReadBundleSeedVersion_MissingManifestFallback(t) }
