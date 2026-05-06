package enforce

const (
	CodeRequiredTestMissing        = "REQUIRED_TEST_MISSING"
	CodeAnnotationUnknownDirective = "ANNOTATION_UNKNOWN_DIRECTIVE"
	CodeAnnotationMissingValue     = "ANNOTATION_MISSING_VALUE"
	CodeAnnotationBadStateTrans    = "ANNOTATION_BAD_STATE_TRANSITION"
	CodeAnnotationUnknownInvariant = "ANNOTATION_UNKNOWN_INVARIANT"
	CodeHashSchemaNoProducer       = "HASH_SCHEMA_NO_PRODUCER"
	CodeHashSchemaNoConsumer       = "HASH_SCHEMA_NO_CONSUMER"
	CodeGraphSourceFileMissing     = "GRAPH_SOURCE_FILE_MISSING"
	CodeStaleSourceFileNode        = CodeGraphSourceFileMissing
	CodeInvariantNoEnforcer        = "INVARIANT_NO_ENFORCER"
	CodePackageContractMissing     = "PACKAGE_CONTRACT_MISSING"
	CodeDependencyCycleDangerous   = "DEPENDENCY_CYCLE_DANGEROUS"

	// Backward-compatible aliases used by existing triage/action mappings.
	CodeMalformedStateTransition = CodeAnnotationBadStateTrans
	CodeAnnotationBadIdentifier  = "ANNOTATION_BAD_IDENTIFIER"
	CodeAnnotationBadTestName    = "ANNOTATION_BAD_TEST_NAME"
	CodeAnnotationRefInvariantMissing = CodeAnnotationUnknownInvariant
	CodeAnnotationRefTestMissing = "ANNOTATION_REF_TEST_MISSING"
	CodeRequiredTestNoPath       = "REQUIRED_TEST_NO_PATH"
	CodeMissingHashProducer      = CodeHashSchemaNoProducer
	CodeMissingHashConsumer      = CodeHashSchemaNoConsumer
	CodeHashSchemaOrphaned       = "HASH_SCHEMA_ORPHANED"
	CodeOrphanedHashSchema       = CodeHashSchemaOrphaned
	CodeOrphanedInvariantNode    = CodeInvariantNoEnforcer
	CodeNoGraph                  = "NO_GRAPH"
)
