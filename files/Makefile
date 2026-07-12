# ZAEE Test Runner
# ================
# make test-unit         → run unit tests only (no Docker needed)
# make test-integration  → run integration tests (requires docker-compose up)
# make test-system       → run full system test (requires all generators running)
# make test-all          → run everything and generate report
# make report            → generate HTML report from existing results

.PHONY: test-unit test-integration test-system test-all report clean

# ---- Unit Tests ----
test-unit:
	@echo "=== Running Unit Tests ==="
	cd engine && go test -v -race -json ./... -tags=unit 2>/dev/null | tee ../test_results.json
	@echo "Unit tests complete."

# ---- Integration Tests ----
test-integration:
	@echo "=== Starting infrastructure ==="
	docker-compose -f infra/docker-compose.yaml up -d
	@echo "Waiting for services to be ready..."
	sleep 5
	@echo "=== Running Integration Tests ==="
	cd engine && go test -v -race -json -tags=integration ./tests/integration/... 2>/dev/null | tee -a ../test_results.json
	@echo "Integration tests complete."

# ---- System Tests ----
test-system:
	@echo "=== Running System Tests ==="
	@echo "Ensure Phase 0 orchestrator is running: python generators/orchestrator.py"
	cd engine && go test -v -json -tags=system -timeout=20m ./tests/system/... 2>/dev/null | tee -a ../test_results.json
	@echo "System tests complete."

# ---- Full Run + Report ----
test-all: clean test-unit test-integration test-system report

# ---- Report Only ----
report:
	@echo "=== Generating Evidence Report ==="
	python3 tests/report_generator.py \
		--test-results test_results.json \
		--metrics "test_metrics_*.json" \
		--output report.html
	@echo "Report: report.html"

# ---- Add testify ----
setup:
	cd engine && go get github.com/stretchr/testify@latest
	cd engine && go get github.com/testcontainers/testcontainers-go@latest

clean:
	rm -f test_results.json test_metrics_*.json report.html
