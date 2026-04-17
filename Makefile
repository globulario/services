.PHONY: check-controller-no-exec check-nodeagent-exec-boundary check-services test-invariants test-integration test

check-controller-no-exec:
	@echo "Checking clustercontroller_server has no exec/syscall usage..."
	@if grep -R --include='*.go' -nE 'os/exec|syscall|StartProcess|exec\.Command|systemctl' ./golang/clustercontroller/clustercontroller_server; then \
		echo "ERROR: Forbidden exec-related usage found in clustercontroller_server"; \
		exit 1; \
	fi
	@echo "OK"

check-nodeagent-exec-boundary:
	@echo "Checking nodeagent_server os/exec boundary..."
	@if grep -R --include='*.go' -nE 'os/exec' ./golang/nodeagent/nodeagent_server | grep -v '/internal/supervisor/'; then \
		echo "ERROR: os/exec usage found outside internal/supervisor"; \
		exit 1; \
	fi
	@echo "OK"

check-services: check-controller-no-exec check-nodeagent-exec-boundary

test-invariants:
	@echo "Running invariant tests (no cluster required)..."
	cd golang && go test ./repository/repository_server/... -run "TestINV|TestReservation|TestMigrate" -v -count=1 -race
	@echo "All invariant tests passed."

test-integration:
	@echo "Running integration tests (requires cluster)..."
	cd golang && go test ./... -race -short -count=1
	@echo "Integration tests complete."

test: check-services test-invariants
	@echo "All checks and tests passed."
