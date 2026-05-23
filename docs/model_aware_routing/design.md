# Design: Model-Aware Routing

## Overview
Currently, the `llmrouter` selects providers based solely on global strategies (cost, latency, round-robin) without considering if the chosen provider actually supports the requested model. This leads to failures when LiteLLM (acting as a Claude-to-OpenAI adapter) requests specific models that only some backends support.

This feature enables the router to filter the available provider pool for each request based on a supported `models` list defined in the configuration.

## Core Architecture

### 1. Configuration Extension
The `ProviderConfig` in `pkg/config/config.go` will be extended to include a `models` field:
```yaml
providers:
  - name: groq-backend
    type: groq
    models: ["llama3-8b-8192", "llama3-70b-8192"]
  - name: gemini-backend
    type: gemini
    models: ["gemini-1.5-flash", "claude-3-5-sonnet"] # Note: including aliases for Claude compatibility
```

### 2. Provider Metadata
The `ProviderWithMetadata` struct in `pkg/router/router.go` will store this list to allow for fast lookups during the routing decision.

### 3. Routing Logic (The "Filter" Step)
The `Router.selectProviders` method will be updated to:
1. Receive the requested `Model` name from the `ChatCompletionRequest`.
2. Filter the pool of providers to only those that list the requested model (or all providers if no specific model filtering is defined for that agent).
3. Apply the existing strategy (cost/latency) to the *filtered* subset.

## Staged Implementation Plan

### Phase 1: Schema & Registry Updates
- Update `pkg/config/config.go` with the `Models` field.
- Update `pkg/router/router.go` metadata struct and the initialization logic in `pkg/router/registry.go`.

### Phase 2: Filtering Logic Implementation
- Refactor `selectProviders` in `pkg/router/router.go` to support model-based filtering.
- Update `ChatCompletion` and `StreamChatCompletion` to pass the requested model to the selection engine.

### Phase 3: Validation & Real-API Configuration
- Add unit tests for model filtering.
- Update `local-dev/configmap.yaml` with real model mappings for Groq and Gemini to support Claude Code.

## Benefits
- **Reliability:** Eliminate "404 Model Not Found" errors from upstream providers.
- **Interoperability:** Seamlessly support Claude Code (via LiteLLM) by mapping Claude model names to the appropriate backend.
- **Granular Control:** Allow different agents to use overlapping provider pools while ensuring model compatibility.
