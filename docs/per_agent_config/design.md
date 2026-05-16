# Design: Per-Agent Configuration

## Overview
This document outlines a staged approach to support per-agent configurations in the `llmrouter`. This allows individual agents to define their own routing policies, provider subsets, and model parameters, moving away from a single global routing configuration.

## Proposed Architecture
The core change involves shifting from a single global `Router` to an `AgentRouterRegistry` that manages multiple routing policies based on the `AgentID` provided in the request.

## Staged Implementation Plan

### Phase 1: Configuration Schema Extension
- Update `Config` struct in `pkg/config/config.go` to support a map of agent configurations.
- Allow agents to specify their own `Providers` and `Routing` (strategy, failover, etc.).
- Maintain a "default" agent configuration for backward compatibility.

### Phase 2: Router Registry
- Create `RouterRegistry` to manage instances of `Router`.
- Implement logic to load/reload configurations and instantiate routers for each agent.
- Ensure the registry is thread-safe for hot-reloads.

### Phase 3: Request Handling
- Update `api.ChatCompletionRequest` to include an `AgentID` field.
- Modify the `server` to look up the appropriate `Router` instance from the `RouterRegistry` using the provided `AgentID`.
- Handle cases where no `AgentID` is provided (fall back to the default router).

### Phase 4: Observability & Metrics
- Update metrics (Prometheus) to include `agent_id` as a label in all relevant counters (token usage, latency, error rates).

### Phase 5: Production Readiness & Validation
- **Enhanced Testing:** Implement integration tests specifically for per-agent routing, streaming, and hot-reload behavior.
- **Manifest Modernization:** Update Helm charts and local development manifests to support and demonstrate multi-agent configurations.
- **Validation:** Ensure 70%+ coverage on core routing and registry logic.

## Migration & Rollback
- Configuration files will remain YAML-based.
- A smooth migration path will be provided by treating existing configurations as the "default" agent.
- Rollback involves reverting the `Config` struct and the `RouterRegistry` logic back to the single-router singleton pattern.
