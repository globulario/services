# Globular CLI Testing Guide

## Overview

The CLI has comprehensive test coverage to prevent regressions, especially for critical bugs that break shell script integration.

## Critical Bug Fixes Covered

### Fix #1: Domain Status Exit Codes
**Problem:** `globular domain status --fqdn missing.example.com` printed "Domain not found" but returned exit code 0, causing shell scripts to incorrectly conclude the domain existed.

**Fix:** Returns exit code 1 when domain not found.

**Tests:**
- `TestDomainStatusNotFoundExitCode` - Verifies non-zero exit on not-found (both plain and JSON output)

### Fix #2: Domain Add Verification
**Problem:** `globular domain add` reported success without verifying the spec persisted to etcd, leading to false "registered successfully" messages when etcd was misconfigured.

**Fix:** Reads back the spec after writing and returns error if verification fails.

**Tests:**
- `TestDomainAddVerification` - Verifies read-back check prevents false positives

### Fix #3: JSON Output Consistency
**Problem:** `globular domain status --output json` returned mixed plain text and JSON, breaking `jq` parsing in scripts.

**Fix:**
- Always outputs valid JSON when `--output json` is used
- Returns `[]` when no domains exist (not an error)
- Returns JSON error object `{"error": "...", "fqdn": "..."}` to stderr when domain not found

**Tests:**
- `TestDomainStatusNoDomainsJSONOutput` - Verifies empty array output
- `TestDomainStatusJSONArrayConsistency` - Verifies array format consistency
- `TestDomainStatusJSONFieldStability` - Verifies JSON schema stability

## Test Types

### Unit Tests (No Dependencies)
These tests run without external services and verify core logic:

```bash
go test -v -run "TestFormatCondition|TestFormatDuration|TestDomainSpec"
```

**Tests:**
- `TestDomainSpecValidation` - Domain spec validation logic
- `TestDomainSpecJSONRoundtrip` - Serialization/deserialization
- `TestFormatCondition` - Condition status formatting (✓/✗/-)
- `TestFormatDuration` - Duration formatting (30s ago, 5m ago, etc.)

### Integration Tests (Require etcd)
These tests interact with a real etcd instance:

```bash
# Start etcd first
docker run -d --name etcd-test \
  -p 2379:2379 -p 2380:2380 \
  quay.io/coreos/etcd:latest \
  /usr/local/bin/etcd \
  --advertise-client-urls http://127.0.0.1:2379 \
  --listen-client-urls http://0.0.0.0:2379

# Run integration tests
go test -v -run TestDomain

# Cleanup
docker stop etcd-test && docker rm etcd-test
```

**Tests:**
- `TestDomainStatusNotFoundExitCode` - Exit code behavior
- `TestDomainStatusNoDomainsJSONOutput` - Empty list handling
- `TestDomainAddVerification` - Persistence verification
- `TestDomainStatusJSONArrayConsistency` - JSON array output
- `TestDomainStatusJSONFieldStability` - JSON schema stability

**Note:** Integration tests automatically skip if etcd is unavailable.

## Running All Tests

```bash
# Run all tests (unit + integration, integration skips without etcd)
go test -v ./...

# Run only domain tests
go test -v -run TestDomain

# Run specific test
go test -v -run TestDomainStatusNotFoundExitCode
```

## Test Coverage

```bash
# Generate coverage report
go test -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## CI/CD Integration

For CI environments, ensure etcd is available:

```yaml
# .github/workflows/test.yml example
services:
  etcd:
    image: quay.io/coreos/etcd:latest
    ports:
      - 2379:2379
    options: >-
      --health-cmd "etcdctl endpoint health"
      --health-interval 10s
```

## Preventing Regressions

When adding new features that affect exit codes or JSON output:

1. **Add tests first** - Write tests that verify the expected behavior
2. **Verify scripts** - Test with actual shell scripts that depend on exit codes
3. **Check JSON schema** - Use `jq` to validate JSON structure
4. **Run full suite** - Ensure existing tests still pass

## Common Pitfalls

### Exit Code Testing
```go
// ❌ WRONG - doesn't verify exit code
fmt.Println("Domain not found")
return nil  // Exit 0 - WRONG!

// ✅ CORRECT - returns error for non-zero exit
return fmt.Errorf("domain %s not found", fqdn)
```

### JSON Output Testing
```go
// ❌ WRONG - mixed plain text and JSON
if notFound {
    fmt.Println("Domain not found")  // Plain text
    return nil
}

// ✅ CORRECT - always valid JSON in JSON mode
if notFound {
    if outputFormat == "json" {
        fmt.Fprintf(os.Stderr, `{"error": "domain not found", "fqdn": %q}`+"\n", fqdn)
    } else {
        fmt.Fprintf(os.Stderr, "Domain %q not found.\n", fqdn)
    }
    return fmt.Errorf("domain %s not found", fqdn)
}
```

### Array Consistency
```go
// ❌ WRONG - different types for single vs multiple
if len(domains) == 1 {
    json.Marshal(domains[0])  // Object
} else {
    json.Marshal(domains)  // Array
}

// ✅ CORRECT - always array for consistency
json.Marshal(domains)  // Always array, even with 0 or 1 item
```

## Shell Script Validation

Test your CLI with actual shell scripts:

```bash
#!/bin/bash
set -e  # Exit on error

# Test 1: Exit code check
if globular domain status --fqdn missing.example.com 2>/dev/null; then
    echo "ERROR: Should fail for missing domain"
    exit 1
fi
echo "✓ Exit code check passed"

# Test 2: JSON parsing
STATUS=$(globular domain status --fqdn test.example.com --output json 2>/dev/null || echo "[]")
PHASE=$(echo "$STATUS" | jq -r '.[0].status.phase // "Unknown"')
echo "✓ JSON parsing passed: phase=$PHASE"

# Test 3: Empty list
ALL=$(globular domain status --output json 2>/dev/null)
if ! echo "$ALL" | jq -e 'type == "array"' >/dev/null; then
    echo "ERROR: Should return JSON array"
    exit 1
fi
echo "✓ Empty list check passed"
```

## Additional Resources

- [Cobra Testing Guide](https://github.com/spf13/cobra/blob/master/doc/testing.md)
- [Go Testing Best Practices](https://go.dev/doc/tutorial/add-a-test)
- [Table-Driven Tests in Go](https://go.dev/wiki/TableDrivenTests)
