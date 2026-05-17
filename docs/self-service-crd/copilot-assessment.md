# Self-Service CRD — Copilot Assessment

**Date:** 2026-05-16  
**Evaluator:** GitHub Copilot  
**Overall Status:** ✅ Implemented (all 3 phases complete, with known production gaps)

---

## Phase-by-Phase Status

| Phase | Title | Status | Notes |
|-------|-------|--------|-------|
| Phase 1 | Config Server & CRD | ✅ Complete | `cmd/config-server/main.go`, CRD at `charts/llmrouter/crds/agents.yaml`, SSE `/v1/sync`, `client-go` informer |
| Phase 2 | Router Remote Provider | ✅ Complete | `pkg/config/remote.go`, `WatchRemoteConfig()` on server, `CONFIG_SOURCE`/`CONFIG_URL` env vars in `cmd/server/main.go`, `pkg/config/remote_test.go` |
| Phase 3 | Helm & Infrastructure | ✅ Complete | `config-server-deployment.yaml` (3 replicas), `config-server-service.yaml`, `config-server-rbac.yaml` (ClusterRole + binding), `deployment.yaml` switches on `config.source` |

---

## Phase Detail

### Phase 1 — Config Server & CRD

**✅ Implemented:**
- `cmd/config-server/main.go` starts a `client-go` dynamic informer on `agents.llmrouter.io/v1`
- SSE endpoint at `:8081/v1/sync` fans out `Agent` state to all connected router instances
- CRD schema in `charts/llmrouter/crds/agents.yaml` matches the `AgentConfig` struct exactly
- `reconcile()` maps CRD spec → `map[string]AgentConfig` and broadcasts via SSE

**⚠️ Known issues:**
- **No local dev mode**: `rest.InClusterConfig()` failure triggers `log.Fatal()`. The comment acknowledges a `clientcmd` fallback is needed but it is not implemented. The config server cannot be run or tested outside a Kubernetes cluster.
- **`reconcile()` panic risk**: Type assertions `spec["routing"].(map[string]interface{})` and `spec["providers"].([]interface{})` have no nil/ok guards. A malformed or minimal `Agent` CRD will panic the config server process.
- **`reconcile()` logic bug**: `agents[name] = agent` is assigned inside the inner provider loop rather than after it completes. Only the last iteration's provider list is captured per agent. This produces incorrect config for any agent with more than one provider.

### Phase 2 — Router Remote Provider

**✅ Implemented:**
- `RemoteProvider` in `pkg/config/remote.go` connects to the config server SSE stream
- 5-second reconnect loop with context cancellation support
- `server.WatchRemoteConfig(ctx, url)` bridges provider updates to `Registry.UpdateAgents()`
- `CONFIG_SOURCE=remote` and `CONFIG_URL` environment variables control mode in `cmd/server/main.go`
- `pkg/config/remote_test.go` covers the SSE parsing and channel push path

**⚠️ Known issues:**
- `WatchRemoteConfig` is not covered by tests in `pkg/server` (server coverage at 64.3% vs 70% target)

### Phase 3 — Helm & Infrastructure

**✅ Implemented:**
- `config-server-deployment.yaml`: 3 replicas, gated by `configServer.enabled` in values
- `config-server-service.yaml`: ClusterIP service on port 8081
- `config-server-rbac.yaml`: `ServiceAccount`, `ClusterRole` with `get/list/watch` on `agents`, `ClusterRoleBinding`
- `deployment.yaml`: conditionally sets `CONFIG_SOURCE=remote` + `CONFIG_URL` or `CONFIG_SOURCE=file` + `CONFIG_PATH`
- `values.yaml`: `configServer.enabled`, `configServer.replicas`, `config.source`, `config.url`

**✅ Verified:**
```
helm template llmrouter charts/llmrouter --set configServer.enabled=true
```
Renders: Secret, ConfigMap, Service (x2), Deployment (x2), ServiceAccount, ClusterRole, ClusterRoleBinding — all correct.

---

## Issues Found

### 🔴 High Priority

**Config server crashes outside cluster (Phase 1)**  
`rest.InClusterConfig()` fails when running locally; `log.Fatal()` is called immediately. This makes local development, integration testing, and CI runs without a cluster impossible.  
Fix: Add `clientcmd.BuildConfigFromFlags` fallback reading `$KUBECONFIG` or `~/.kube/config`.

**`reconcile()` logic bug — only last provider captured (Phase 1)**  
```go
// BUG: assignment inside provider loop
for _, p := range providers {
    agent.Providers = append(agent.Providers, ...)
    agents[name] = agent  // ← should be AFTER the loop
}
```
For multi-provider agents, only the final provider is stored. Move `agents[name] = agent` outside the provider loop.

### 🟡 Medium Priority

**`reconcile()` missing nil/type-assertion guards (Phase 1)**  
`spec["routing"].(map[string]interface{})` will panic if `routing` is absent or not a map. Same for `providers`. Add comma-ok style assertions or existence checks.

**Server test coverage at 64.3%** (Phase 2 target: 70%)  
`WatchRemoteConfig` goroutine is untested. Add a test using a local `httptest.Server` that emits SSE events and verify `Registry.UpdateAgents` is called.

### 🟢 Low Priority

**No kubeconfig fallback documented in runbook**  
Even when the code fallback is added, the local-dev setup instructions (Tiltfile/README) don't describe how to run the config server locally against a kind cluster.

---

## What's Working Well

- Full SSE fan-out architecture is sound: CRD watch → reconcile → SSE broadcast → RemoteProvider channel → registry hot-reload
- Helm chart cleanly feature-flags the entire control plane behind a single `configServer.enabled` toggle
- `remote_test.go` validates the critical SSE parsing path
- CRD schema is precise and matches Go structs without drift
- RBAC is least-privilege (only `agents` resource, no cluster-wide write access)

---

## Recommended Actions

| Priority | Action |
|----------|--------|
| 🔴 | Fix `reconcile()` — move `agents[name] = agent` outside the provider loop |
| 🔴 | Add `clientcmd` kubeconfig fallback to config server for local dev |
| 🟡 | Add nil/ok guards on all type assertions in `reconcile()` |
| 🟡 | Add `WatchRemoteConfig` test in `pkg/server` to reach 70% coverage |
| 🟢 | Document local config-server dev setup in README or Tiltfile comments |
