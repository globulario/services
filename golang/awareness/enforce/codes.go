package enforce

// Stable machine-readable finding codes for all awareness audit results.
// These codes appear in JSON output, suppression rules, and CI tooling.
// Never rename a code without a deprecation path — suppression rules reference them by name.
const (
	// --- Annotation well-formedness ---

	// CodeAnnotationUnknownDirective: a //globular: directive is not in the known set.
	CodeAnnotationUnknownDirective = "ANNOTATION_UNKNOWN_DIRECTIVE"

	// CodeAnnotationMissingValue: a //globular: directive has no value.
	CodeAnnotationMissingValue = "ANNOTATION_MISSING_VALUE"

	// CodeAnnotationBadStateTransition: state_transition value is not "FROM -> TO".
	// Alias: CodeMalformedStateTransition (legacy).
	CodeAnnotationBadStateTransition = "ANNOTATION_BAD_STATE_TRANSITION"

	// CodeAnnotationBadIdentifier: identifier directive has whitespace.
	CodeAnnotationBadIdentifier = "ANNOTATION_BAD_IDENTIFIER"

	// CodeAnnotationBadTestName: tested_by value does not start with Test/Benchmark/Example.
	CodeAnnotationBadTestName = "ANNOTATION_BAD_TEST_NAME"

	// CodeAnnotationUnknownInvariant: enforces/protects references an invariant not in the graph.
	CodeAnnotationUnknownInvariant = "ANNOTATION_UNKNOWN_INVARIANT"

	// --- Annotation reference checks (strict mode) ---

	// CodeAnnotationRefInvariantMissing: enforces/protects target not found in graph.
	CodeAnnotationRefInvariantMissing = "ANNOTATION_REF_INVARIANT_MISSING"

	// CodeAnnotationRefTestMissing: tested_by target not found in graph.
	CodeAnnotationRefTestMissing = "ANNOTATION_REF_TEST_MISSING"

	// --- Hash schema contracts ---

	// CodeHashSchemaOrphaned: hash_schema node exists with no producer and no consumer.
	// Alias: CodeOrphanedHashSchema (legacy).
	CodeHashSchemaOrphaned = "HASH_SCHEMA_ORPHANED"

	// CodeHashSchemaNoProducer: hash_schema has consumers but no producer.
	// Alias: CodeMissingHashProducer (legacy).
	CodeHashSchemaNoProducer = "HASH_SCHEMA_NO_PRODUCER"

	// CodeHashSchemaNoConsumer: hash_schema has a producer but no consumer yet.
	// Alias: CodeMissingHashConsumer (legacy).
	CodeHashSchemaNoConsumer = "HASH_SCHEMA_NO_CONSUMER"

	// --- Required tests ---

	// CodeRequiredTestMissing: tested_by target does not exist in the graph at all.
	CodeRequiredTestMissing = "REQUIRED_TEST_MISSING"

	// CodeRequiredTestNoPath: tested_by target is in the graph but has no source file path.
	CodeRequiredTestNoPath = "REQUIRED_TEST_NO_PATH"

	// CodeRequiredTestLookupError: graph query failed while checking tested_by target.
	CodeRequiredTestLookupError = "REQUIRED_TEST_LOOKUP_ERROR"

	// --- Graph integrity ---

	// CodeGraphSourceFileMissing: source_file graph node exists but the file is gone from disk.
	// Alias: CodeStaleSourceFileNode (legacy).
	CodeGraphSourceFileMissing = "GRAPH_SOURCE_FILE_MISSING"

	// CodeInvariantNoEnforcer: invariant node has no enforces or protects edge.
	// Alias: CodeOrphanedInvariantNode (legacy).
	CodeInvariantNoEnforcer = "INVARIANT_NO_ENFORCER"

	// CodeDependencyCycleDangerous: a dangerous dependency cycle was detected.
	CodeDependencyCycleDangerous = "DEPENDENCY_CYCLE_DANGEROUS"

	// --- Package contracts ---

	// CodePackageContractMissing: a package has no awareness contract in the graph.
	CodePackageContractMissing = "PACKAGE_CONTRACT_MISSING"

	// --- Infrastructure ---

	// CodeNoGraph: a graph-dependent check was skipped because no graph DB is available.
	CodeNoGraph = "NO_GRAPH"

	// --- Legacy codes (kept for backwards-compatibility; emitted by pre-9.2 code) ---
	// Do NOT use these in new code. Use the canonical codes above.

	// CodeMalformedStateTransition is the legacy code for CodeAnnotationBadStateTransition.
	CodeMalformedStateTransition = "MALFORMED_STATE_TRANSITION"

	// CodeOrphanedHashSchema is the legacy code for CodeHashSchemaOrphaned.
	CodeOrphanedHashSchema = "ORPHANED_HASH_SCHEMA"

	// CodeMissingHashProducer is the legacy code for CodeHashSchemaNoProducer.
	CodeMissingHashProducer = "MISSING_HASH_PRODUCER"

	// CodeMissingHashConsumer is the legacy code for CodeHashSchemaNoConsumer.
	CodeMissingHashConsumer = "MISSING_HASH_CONSUMER"

	// CodeStaleSourceFileNode is the legacy code for CodeGraphSourceFileMissing.
	CodeStaleSourceFileNode = "STALE_SOURCE_FILE_NODE"

	// CodeOrphanedInvariantNode is the legacy code for CodeInvariantNoEnforcer.
	CodeOrphanedInvariantNode = "ORPHANED_INVARIANT_NODE"
)
