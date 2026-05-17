# Per-Agent Config — Copilot Assessment

**Date:** 2026-05-16  
**Evaluator:** GitHub Copilot  
**Overall Status:** ✅ Implemented (4 phases complete, 1 partial)

---

## Phase-by-Phase Status

| Phase | Title | Status | Notes |
|-------|-------|--------|-------|
| Phase 1 | Config Schema Extension | ✅ Complete | `AgentConfig` struct, `agents` map in `config.go`, backward-compat `default` fallback |
| Phase 2 | Router Registry | ✅ Complete | `pkg/router/registry.go` — `RouterRegistry`, `GetRouter()` with default fallback, `UpdateAgents()` atomic hot-reload |
| Phase 3 | Request Handling | ✅ Complete | `AgentID` on `ChatCompletionRequest`, server dispatches by agent, nil-engine guard |
| Phase 4 | Observability | ✅ Complete | `agent_id` label on `RequestDuration` histogram and `TokenUsage` counter; `WithAgentID()` fluent setter on router |
| Phase 5 | Production Readiness | ⚠️ Partial | See detail below |

---

## Phase 5 Detail

### ✅ What's Done
- `pkg/server/agent_integration_test.go` — 4 subtests: default routing, specific agent, fallback to default, hot-reload via registry
- Streaming subtest present in `pkg/router/router_test.go`
- `local-dev/configmap.yaml` updated with dual-agent layout (`default` + `researcher`) using real providers
- Helm `configmap.yaml` template dynamically renders the `agents` structure from `values.yaml`
- `pkg/router` coverage: **73.7%** — exceeds 70% target ✓
- `pkg/config` coverage: **77.4%** ✓

### ⚠️ Gaps
- `pkg/server` coverage: **64.3%** — Phase 5 stated this was "maintained at 70%"; currently below target. Untested paths: `WatchRemoteConfig`, streaming error path.
- `local-dev/secret.yaml` only contains `GROQ_API_KEY`. The `researcher` agent in `local-dev/configmap.yaml` references `${GEMINI_API_KEY}` (for the Gemini provider), which will resolve to an empty string at runtime.

---

## Issues Found

### 🔴 High Priority

**`local-dev/secret.yaml` missing `GEMINI_API_KEY`**  
The `researcher` agent configures a Gemini provider with `api_key: "${GEMINI_API_KEY}"`. The secret does not include this key, so `kubectl apply -f local-dev/` will result in a broken `researcher` agent. Fix: add `GEMINI_API_KEY` to `local-dev/secret.yaml`.

### 🟡 Medium Priority

**`pkg/server` test coverage at 64.3%** (target 70%)  
Primary gaps:
- `WatchRemoteConfig()` — the goroutine that bridges `RemoteProvider` updates to `Registry.UpdateAgents()` is untested
- Streaming error path (when provider returns error on `StreamChatCompletion`)

**`config.go` uses stdlib `log.Printf`**  
All other packages use `zerolog`. The per-agent config additions in `config.go` use `log.Printf` inconsistently.

---

## What's Working Well

- Clean layering: config schema → registry → router → server with no coupling between layers
- Full build passes, all tests pass (`go build ./...`, `go test ./...`)
- Hot-reload works at the per-agent registry level, not just global config
- Gemini provider is implemented and wired into the registry (beyond original phase docs)
- Helm chart correctly feature-flags control-plane components

---

## Recommended Actions

| Priority | Action |
|----------|--------|
| 🔴 | Add `GEMINI_API_KEY` to `local-dev/secret.yaml` |
| 🟡 | Add test for `WatchRemoteConfig` to push server coverage above 70% |
| 🟡 | Replace `log.Printf` with `zerolog` in `pkg/config/config.go` |
