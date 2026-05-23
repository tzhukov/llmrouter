# Performance Design: `llmrouter`

**Date:** 2026-05-23  
**Author:** GitHub Copilot  
**Status:** Proposed  

---

## Overview

This document captures all performance inefficiencies identified in the `llmrouter` codebase, their root causes, remediation designs, and a phased implementation plan. Issues are ranked by impact and grouped into three delivery phases.

---

## Findings Summary

| # | Area | File(s) | Impact | Phase |
|---|------|---------|--------|-------|
| F1 | Prometheus label cardinality explosion | `pkg/observability/metrics.go` | 🔴 High | 1 |
| F2 | Full registry rebuild on every config update | `pkg/router/registry.go` | 🔴 High | 2 |
| F3 | `reconcile()` fires on every CRD event without debouncing | `cmd/config-server/main.go` | 🔴 High | 2 |
| F4 | `bufio.Scanner` default 64 KB buffer silently truncates large SSE payloads | `pkg/config/remote.go` | 🟡 Medium | 1 |
| F5 | `http.Client` created per SSE reconnect — no connection pool reuse | `pkg/config/remote.go` | 🟡 Medium | 1 |
| F6 | `selectProviders` allocates + sorts on every request | `pkg/router/router.go` | 🟡 Medium | 2 |
| F7 | Double logging middleware on every HTTP request | `pkg/server/server.go` | 🟡 Medium | 1 |
| F8 | Round-robin counter is global, not per-model | `pkg/router/router.go` | 🟢 Low | 3 |

---

## Detailed Findings & Remediation

---

### F1 — Prometheus Label Cardinality Explosion

**File:** `pkg/observability/metrics.go`

**Problem:**  
`RequestDuration` and `TokenUsage` both carry a `model` label in addition to `agent_id` and `provider`. With thousands of agents each calling multiple models, the number of active time series grows as:

```
O(agents × providers × models × statuses)
```

At 1,000 agents × 3 providers × 5 models × 2 statuses = **30,000 time series** per metric. Prometheus allocates ~3–5 KB per time series, meaning just these two metrics can consume 90–150 MB of Prometheus RAM and cause slow scrapes.

**Current code:**
```go
RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{...},
    []string{"agent_id", "provider", "model", "status"})

TokenUsage = promauto.NewCounterVec(prometheus.CounterOpts{...},
    []string{"agent_id", "provider", "model", "type"})
```

**Fix:**  
Remove `model` from `RequestDuration` and `TokenUsage`. Model-level granularity is better captured by `TokenUsage` alone (which already encodes token counts, giving a proxy for cost per model). Add a separate low-cardinality `ModelRequests` counter that only tracks `{model, status}` without `agent_id`.

```go
// RequestDuration: drop "model" label
RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
    Name:    "llm_router_request_duration_seconds",
    Help:    "Duration of LLM requests in seconds.",
    Buckets: prometheus.DefBuckets,
}, []string{"agent_id", "provider", "status"})

// TokenUsage: drop "model" label
TokenUsage = promauto.NewCounterVec(prometheus.CounterOpts{
    Name: "llm_router_token_usage_total",
    Help: "Total number of tokens used.",
}, []string{"agent_id", "provider", "type"})

// New: low-cardinality model counter
ModelRequests = promauto.NewCounterVec(prometheus.CounterOpts{
    Name: "llm_router_model_requests_total",
    Help: "Total number of requests per model.",
}, []string{"model", "status"})
```

Update `router.go` `ChatCompletion` call sites to use the new label sets and increment `ModelRequests`.

---

### F2 — Full Registry Rebuild on Every Config Update

**File:** `pkg/router/registry.go`

**Problem:**  
`UpdateAgents` rebuilds the entire `newRouters` map from scratch on every call. Every provider instance is re-allocated and the old ones are thrown away for GC. At 1,000 agents this is 1,000 object allocations + teardown, plus a full write-lock on the registry while the rebuild runs:

```go
func (rr *RouterRegistry) UpdateAgents(agents map[string]config.AgentConfig) {
    rr.mu.Lock()
    defer rr.mu.Unlock()

    newRouters := make(map[string]*Router) // always full rebuild
    for agentID, agentCfg := range agents {
        // re-creates all providers regardless of whether config changed
        ...
        newRouters[agentID] = NewRouter(...)
    }
    rr.routers = newRouters // swap
}
```

The write lock blocks all `GetRouter` calls (which serve live traffic) for the entire duration of the rebuild.

**Fix — Diff-based update:**  
Compare the incoming config against the current state. Only create new `Router` instances for agents that are new or changed. Remove entries that no longer exist. The write lock becomes a brief map mutation rather than a full rebuild.

```go
func (rr *RouterRegistry) UpdateAgents(agents map[string]config.AgentConfig) {
    rr.mu.Lock()
    defer rr.mu.Unlock()

    // Remove deleted agents
    for id := range rr.routers {
        if _, exists := agents[id]; !exists {
            delete(rr.routers, id)
        }
    }

    // Add or update only changed agents
    for agentID, agentCfg := range agents {
        if existing, ok := rr.routers[agentID]; ok {
            if !agentConfigChanged(existing, agentCfg) {
                continue // skip — no change
            }
        }
        rr.routers[agentID] = buildRouter(agentID, agentCfg)
    }
}
```

`agentConfigChanged` compares the serialized config or a struct-level equality check. `buildRouter` is the existing provider-construction logic extracted into a helper.

**Expected improvement:** Lock hold time drops from O(N agents) to O(changed agents), typically 1–5 entries per reconcile cycle.

---

### F3 — `reconcile()` Fires on Every CRD Event Without Debouncing

**File:** `cmd/config-server/main.go`

**Problem:**  
The informer's `AddFunc`, `UpdateFunc`, and `DeleteFunc` all call `reconcile()` directly. A `kubectl apply` deploying 50 agents at once fires 50 sequential full-list reconciles, broadcasting 50 SSE updates to every connected router pod in rapid succession.

```go
informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc:    func(obj interface{}) { server.reconcile(factory) },
    UpdateFunc: func(oldObj, newObj interface{}) { server.reconcile(factory) },
    DeleteFunc: func(obj interface{}) { server.reconcile(factory) },
})
```

**Fix — Coalescing debounce:**  
Use a timer-based debounce. Signal a channel on each event; a dedicated goroutine waits for quiet (100 ms with no new signals) before calling `reconcile()` once.

```go
type ConfigServer struct {
    ...
    reconcileCh chan struct{}
}

func NewConfigServer() *ConfigServer {
    s := &ConfigServer{
        clients:     make(map[chan []byte]struct{}),
        reconcileCh: make(chan struct{}, 1),
    }
    return s
}

func (s *ConfigServer) triggerReconcile() {
    select {
    case s.reconcileCh <- struct{}{}: // signal without blocking
    default: // already pending, no-op
    }
}

func (s *ConfigServer) runReconcileLoop(ctx context.Context, factory dynamicinformer.DynamicSharedInformerFactory) {
    const debounce = 100 * time.Millisecond
    timer := time.NewTimer(0)
    <-timer.C // drain initial fire

    for {
        select {
        case <-s.reconcileCh:
            timer.Reset(debounce)
        case <-timer.C:
            s.reconcile(factory)
        case <-ctx.Done():
            timer.Stop()
            return
        }
    }
}
```

The informer handlers call `s.triggerReconcile()` instead of `s.reconcile(factory)` directly.

**Expected improvement:** A burst of 50 CRD events produces exactly 1 reconcile after the debounce window. SSE broadcast count drops proportionally.

---

### F4 — `bufio.Scanner` Default Buffer Silently Drops Large SSE Lines

**File:** `pkg/config/remote.go`

**Problem:**  
`bufio.NewScanner` uses a 64 KB default token buffer. A single SSE `data:` line containing 1,000 agents at ~500 bytes each is 500 KB — 8× the limit. When the line exceeds the buffer, `scanner.Scan()` returns `false` with `bufio.ErrTooLong`, `subscribe` returns the error, and the router enters a reconnect loop, losing all config until it reconnects.

```go
scanner := bufio.NewScanner(resp.Body) // 64 KB default — too small
```

**Fix:**
```go
const maxSSELine = 4 * 1024 * 1024 // 4 MB — handles ~8,000 agents
scanner := bufio.NewScanner(resp.Body)
scanner.Buffer(make([]byte, maxSSELine), maxSSELine)
```

4 MB is a conservative upper bound. The buffer is allocated once per connection, not per line.

---

### F5 — `http.Client` Created Per SSE Reconnect

**File:** `pkg/config/remote.go`

**Problem:**  
`subscribe()` creates `client := &http.Client{}` on every invocation. Each new client has its own transport with a fresh connection pool. The previous TCP connection and its TLS handshake are discarded on every reconnect, adding 50–200 ms of connection overhead each time the SSE stream drops and re-establishes.

```go
func (p *RemoteProvider) subscribe(ctx context.Context, updateCh chan<- map[string]AgentConfig) error {
    ...
    client := &http.Client{} // new client — no pool reuse
    ...
}
```

**Fix:**  
Promote the client to a struct field, initialized once with a sensible transport:

```go
type RemoteProvider struct {
    url    string
    client *http.Client
}

func NewRemoteProvider(url string) *RemoteProvider {
    return &RemoteProvider{
        url: url,
        client: &http.Client{
            Transport: &http.Transport{
                MaxIdleConns:       10,
                IdleConnTimeout:    90 * time.Second,
                DisableCompression: true, // SSE is not compressible
            },
            // No overall Timeout — SSE streams are long-lived by design.
        },
    }
}
```

`DisableCompression: true` is important for SSE: the `net/http` transport's automatic gzip decompression buffers data and breaks line-by-line streaming.

---

### F6 — `selectProviders` Allocates and Sorts on Every Request

**File:** `pkg/router/router.go`

**Problem:**  
For `cost` and `latency` strategies, `selectProviders` allocates a new slice and calls `sort.Slice` on every single `ChatCompletion` call. With a small provider pool (2–5 entries) this is O(n log n) work per request that is largely redundant — cost-ordered providers only change when config changes.

```go
case "cost":
    sorted := make([]*ProviderWithMetadata, len(filtered)) // alloc every call
    copy(sorted, filtered)
    sort.Slice(sorted, ...)                                // sort every call
    return sorted
```

**Fix:**  
Pre-sort providers at `UpdateAgents` time. Store a `costOrder` and `latencyOrder` slice alongside `providers` in the `Router` struct, updated only on config reload. For `latency`, update the pre-sorted order when `UpdateLatency` is called (a lightweight insertion step on a small slice).

```go
type Router struct {
    ...
    providers   []*ProviderWithMetadata // original registration order
    costOrder   []*ProviderWithMetadata // pre-sorted by PromptPrice at config load
    // latency order is re-derived cheaply; kept separate
}
```

`selectProviders` for `cost` then just filters `r.costOrder` instead of sorting:

```go
case "cost":
    // filter r.costOrder (already sorted) — no alloc for sort
    var filtered []*ProviderWithMetadata
    for _, p := range r.costOrder {
        if supportsModel(p, requestedModel) {
            filtered = append(filtered, p)
        }
    }
    return filtered
```

---

### F7 — Double Logging Middleware

**File:** `pkg/server/server.go`

**Problem:**  
Two logging middlewares are stacked:

```go
r.Use(observability.LoggingMiddleware) // adds request_id to context
r.Use(middleware.Logger)               // chi's built-in full request logger
```

Chi's `middleware.Logger` writes a complete structured log line for every request (method, path, status, latency). `observability.LoggingMiddleware` creates a child `zerolog.Logger` with the request ID attached to the context. This is correct and sufficient. The chi `middleware.Logger` is redundant and produces a second log line per request with no request ID enrichment.

At 10,000 req/s, this doubles log write volume and adds measurable overhead from `time.Now()` being called twice and two `io.Writer` calls per request.

**Fix:**  
Remove `middleware.Logger`. Optionally enrich `observability.LoggingMiddleware` to also log the final status code and duration using a response writer wrapper (chi's `middleware.NewWrapResponseWriter`):

```go
// Remove this line from server.go:
r.Use(middleware.Logger)
```

If request completion logging is required, extend `LoggingMiddleware` to wrap the response writer and emit a single log line after `next.ServeHTTP` returns.

---

### F8 — Round-Robin Counter Not Isolated Per Model

**File:** `pkg/router/router.go`

**Problem:**  
The round-robin counter `r.current` is a single `uint64` shared across all requests regardless of the model being routed. When an agent has providers with disjoint model support sets, requests for model A advance the counter for model B, producing uneven distribution:

```go
// current — shared counter regardless of filtered provider set
startIdx := int(atomic.AddUint64(&r.current, 1) % uint64(len(filtered)))
```

Example: Agent has provider P1 (supports gpt-4o, gpt-4o-mini) and P2 (supports gpt-4o only).  
- Request for gpt-4o-mini → filtered = [P1] → always hits P1, still increments counter  
- Next request for gpt-4o → counter is at 1 → startIdx=1 → hits P2 even though P1 was just idle

**Fix:**  
Keep per-model counters in a `sync.Map`:

```go
type Router struct {
    ...
    rrCounters sync.Map // map[string]*uint64, keyed by model
}

func (r *Router) rrIndex(model string, n int) int {
    v, _ := r.rrCounters.LoadOrStore(model, new(uint64))
    return int(atomic.AddUint64(v.(*uint64), 1) % uint64(n))
}
```

---

## Implementation Phases

### Phase 1 — Quick Wins (Low Risk, High Value)
*Estimated effort: 1–2 days*

Targets: F1, F4, F5, F7 — changes isolated to leaf packages with no architectural impact.

| Task | File(s) | Change |
|------|---------|--------|
| Fix Prometheus cardinality | `pkg/observability/metrics.go`, `pkg/router/router.go` | Drop `model` from high-cardinality vecs; add `ModelRequests` counter |
| Fix scanner buffer | `pkg/config/remote.go` | Set 4 MB buffer on `bufio.Scanner` |
| Reuse `http.Client` | `pkg/config/remote.go` | Promote client to `RemoteProvider` struct field |
| Remove double logging | `pkg/server/server.go` | Remove `middleware.Logger` |

**Acceptance criteria:**
- Prometheus series count does not exceed `agents × providers × 2` for `RequestDuration`
- SSE reconnect loop does not trigger on config payloads up to 4 MB
- No functional regression in `pkg/config/remote_test.go` or `pkg/server` integration tests

---

### Phase 2 — Core Throughput (Moderate Risk)
*Estimated effort: 3–4 days*

Targets: F2, F3, F6 — changes to the hot path and config reconcile loop.

| Task | File(s) | Change |
|------|---------|--------|
| Diff-based registry update | `pkg/router/registry.go` | Implement `agentConfigChanged` + incremental map mutation |
| Debounce reconcile | `cmd/config-server/main.go` | Add `reconcileCh` + `runReconcileLoop` with 100 ms coalesce |
| Pre-sort providers at load | `pkg/router/router.go`, `pkg/router/registry.go` | Add `costOrder` slice to `Router`; sort at `NewRouter` time |

**Acceptance criteria:**
- A burst of 50 `kubectl apply` events produces exactly 1 SSE broadcast (verified by adding a broadcast counter metric)
- `UpdateAgents` with 0 changed agents acquires and releases the lock in < 1 µs (benchmark)
- Existing `registry_test.go` and `router_test.go` pass without modification

---

### Phase 3 — Polish & Correctness (Low Risk)
*Estimated effort: 1 day*

Targets: F8 — correctness fix for round-robin fairness.

| Task | File(s) | Change |
|------|---------|--------|
| Per-model round-robin counters | `pkg/router/router.go` | Replace `r.current uint64` with `r.rrCounters sync.Map` |

**Acceptance criteria:**
- Table-driven test: agent with two providers split by model — after N requests per model, each provider receives ≈ N/2 requests for models it supports
- No regression on existing `router_test.go`

---

## Benchmark Targets

The following microbenchmarks should be added in Phase 2 to gate regressions:

```go
// pkg/router/registry_test.go
func BenchmarkUpdateAgents_NoDiff(b *testing.B)    // 1000 agents, 0 changes
func BenchmarkUpdateAgents_FullRebuild(b *testing.B) // 1000 agents, all new

// pkg/router/router_test.go  
func BenchmarkSelectProviders_Cost(b *testing.B)   // pre-sorted vs sort-per-call
func BenchmarkChatCompletion_RoundRobin(b *testing.B)
```

---

## Observability After Changes

After Phase 1, the following Prometheus queries replace the old model-label queries:

| Old query | New query |
|-----------|-----------|
| `rate(llm_router_request_duration_seconds_count{model="gpt-4o"}[5m])` | `rate(llm_router_model_requests_total{model="gpt-4o"}[5m])` |
| `sum by (model) (rate(llm_router_token_usage_total[5m]))` | Query `TokenUsage` by `agent_id`/`provider`; use `ModelRequests` for model-level rates |

---

## Risk Register

| Risk | Mitigation |
|------|-----------|
| Diff-based update misses a config change (equality bug) | Fall back to full rebuild if diff logic returns false positive; add property-based tests |
| Debounce window (100 ms) too long for latency-sensitive config changes | Make debounce duration configurable via env var `CONFIG_DEBOUNCE_MS` |
| Scanner buffer increase raises memory per router pod | 4 MB × N concurrent SSE connections is negligible; each router has exactly 1 SSE connection |
| Dropping `model` label breaks existing dashboards | Document breaking change; provide Grafana migration snippet in release notes |
