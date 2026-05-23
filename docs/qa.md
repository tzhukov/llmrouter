# QA Setup and Guidelines

This document describes the Quality Assurance (QA) infrastructure and procedures for the `llmrouter` project.

## 🛠️ Tools & Infrastructure

- **Testing Framework:** Standard Go `testing` package with `testify/assert` for improved readability.
- **Continuous Integration (CI):** GitHub Actions (defined in `.github/workflows/ci.yaml`).
- **Linter:** `golangci-lint` (configured in `.golangci.yml`).
- **Benchmarking:** Standard Go benchmarks to measure routing overhead.
- **Task Automation:** `Makefile` for standard development tasks.

## 🚀 Running QA Locally

### 1. Run All Tests
```bash
make test
```
This runs all unit and integration tests across the repository.

### 2. Run Linting
```bash
make lint
```
This runs `golangci-lint` to ensure code quality and adherence to Go conventions.

### 3. Run Benchmarks
```bash
make bench
```
This measures the performance of critical paths like provider selection and registry lookups.

### 4. Build Binaries
```bash
make build
```
Builds the `router` and `config-server` binaries in the `bin/` directory.

## 🧪 Testing Strategy

### Unit Tests
Located alongside the code (e.g., `router_test.go`). Focus on individual components like:
- Routing strategy logic (Round Robin, Cost, Latency).
- Configuration parsing and validation.
- Provider-specific translations (OpenAI, Gemini, etc.).

### Integration Tests
Focus on interactions between components. Key integration tests are in `pkg/server/`:
- `integration_test.go`: Tests the full server lifecycle, including config watching and hot reloading.
- `agent_integration_test.go`: Tests per-agent routing and fallbacks.

### Benchmarks
Located in `pkg/router/router_bench_test.go`. Used to ensure the router remains "high-performance" and doesn't introduce significant latency.

## 🛡️ Security Scanning
CI includes basic security checks via `golangci-lint` (govet, errcheck). Future enhancements will include:
- `gosec` for static security analysis.
- `govulncheck` for dependency vulnerability scanning.

## 📈 Future QA Roadmap
- **E2E Tests:** Automated tests running in a local `kind` cluster using the Helm charts.
- **Load Testing:** Automated scripts using `hey` or `k6` to verify stability under heavy load.
- **Fuzz Testing:** Fuzzing the configuration parser and API request handlers.
