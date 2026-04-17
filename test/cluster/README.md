# Integration Tests

These tests run against a containerized Globular cluster provided by
[globular-quickstart](https://github.com/globulario/globular-quickstart).

## Quick Start

```bash
# 1. Start the test cluster (from quickstart repo)
cd ../../../globular-quickstart
make up

# 2. Wait for cluster health
make status

# 3. Run integration tests
cd ../services
GLOBULAR_TEST_CLUSTER=1 make test-integration
```

## CI Pipeline

- **On PR**: `make test-invariants` (fast, no cluster, ~1 second)
- **On merge to main**: `make test-integration` (containerized cluster)
- **On tag**: full deploy simulation via quickstart
