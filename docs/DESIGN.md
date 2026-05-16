# Design Document: Multi-Provider LLM Router

## 1. Overview
A high-performance Go-based proxy server that routes LLM requests across multiple providers (OpenAI, Anthropic, Google, etc.) based on cost, latency, and availability. It exposes an OpenAI-compatible API to ensure seamless integration with existing tools and SDKs.

## 2. Goals
- **Unified Interface:** Expose an OpenAI-compatible API.
- **Resilience:** Automatic failover and retry logic across multiple providers.
- **Optimization:** Dynamic routing based on real-time latency and cost metrics.
- **Observability-First:** Built-in structured logging, Prometheus metrics, and tracing.
- **K8s-Native:** Native support for ConfigMaps (configuration) and Secrets (auth) with **Hot Reloading**.
- **Scalability:** Stateless design optimized for horizontal scaling.
- **Testability:** First-class support for MockLLM and local Kubernetes integration testing via **Tilt/Docker/Helm**.

## 3. Architecture
- **Proxy Layer:** Handles incoming HTTP requests, authentication, and request/response transformation.
- **Router Engine:** Stateless selection logic; uses a shared metrics provider for latency-based decisions.
- **Provider Adapters:** Normalized interfaces for different LLM providers (OpenAI, Groq, MockLLM).
- **Config Watcher:** Monitors YAML configuration (via ConfigMap volume) and triggers hot-reloads of provider pools without downtime.
- **Middleware:**
    - **Metrics:** Prometheus exporter for latency, token counts, and error rates.
    - **Tracing:** OpenTelemetry or LangSmith integration.
    - **Cost Tracker:** Tracks token usage and estimates cost per request.

## 4. Provider Connectivity & Authentication
- **Authentication:**
    - **API Keys:** Providers are authenticated via API keys.
    - **Credential Management:** Secrets managed via environment variables or **Kubernetes Secrets**.
- **Configuration (K8s-Native):**
    - **ConfigMaps:** LLM provider definitions (endpoints, model mappings, weights) stored in ConfigMaps.
    - **Hot Reload:** The router watches the mounted config volume and refreshes its internal state using `fsnotify`.
- **Connectivity:**
    - **HTTP Clients:** Dedicated `http.Client` per adapter with tuned timeouts and connection pooling.
    - **Streaming:** Full support for Server-Sent Events (SSE).

## 5. Observability-First Design
- **Structured Logging:** High-performance JSON logging (e.g., `zerolog`) with mandatory `request_id` propagation.
- **Standardized Metrics:**
    - `llm_router_request_duration_seconds`: Histogram by provider, model, and status.
    - `llm_router_token_usage_total`: Counter by provider and model.
    - `llm_router_provider_health`: Gauge indicating provider availability.
- **Tracing:** Distributed tracing for full request lifecycle visibility.

## 6. Routing Strategies
- **Load Balancing:** Weighted round-robin across available providers.
- **Failover/Retry:** Automatic retry with the next best provider if a request fails (e.g., 5xx, 429).
- **Latency-based:** Uses a moving average of response times to favor faster providers.
- **Cost-based:** Selects providers to minimize cost per 1k tokens.

## 7. Technology Stack
- **Language:** Go 1.22+
- **Web Framework:** `chi` (lightweight and idiomatic).
- **Logging:** `zerolog` or `zap`.
- **Observability:** `prometheus/client_golang`, OpenTelemetry/LangSmith.
- **Configuration:** YAML, `fsnotify` for hot-reload.
- **Containerization:** Docker & **Helm**.
- **Development Tooling:** Tilt for rapid local development.

## 8. Implementation Plan

### Phase 1: Core Infrastructure
- Initialize Go module.
- Set up basic HTTP server with `chi`.
- Define OpenAI-compatible Request/Response structs.
- **Create Dockerfile, Helm Chart, and Tiltfile for local development.**

### Phase 2: Provider Abstraction & Mocking
- Define `Provider` interface.
- **Implement MockLLM provider** (Priority 0).
- Implement OpenAI and Groq adapters.

### Phase 3: Routing Engine & K8s Configuration
- Implement a basic Load Balancer (Round Robin).
- Implement Failover middleware.
- **Implement Config Watcher:** Support hot-reloading YAML from ConfigMaps.
- **Integration tests using MockLLM** to verify failover and hot-reload logic.

### Phase 4: Observability-First Implementation
- **Structured Logging:** Integrate JSON logging with request ID tracing.
- **Prometheus Metrics:** Implement standard metrics suite.
- **Tracing:** Implement OTel/LangSmith tracing wrapper.

### Phase 5: Advanced Routing
- Implement Latency tracking (moving average).
- Implement Cost-based routing logic.

### Phase 6: Testing & Validation
- Unit tests, MockLLM edge-case suite, and **Kubernetes Integration Testing via Tilt/Helm**.

### Phase 7: Production Hardening
*Based on the [Copilot Assessment](../docs/copilot-assesment.md)*
- **Root README.md:** Create onboarding and architecture documentation.
- **Streaming Support (SSE):** Implement real-time streaming for chat completions.
- **Enhanced Testing:** Add unit tests for `config` and provider adapters.
- **Granular Error Handling:** Differentiate between rate limits (429) and server errors (5xx) for smarter failover.

### Phase 8: Google Gemini Provider
- **Gap:** No Google Gemini provider adapter exists in `pkg/provider/`. OpenAI and Groq are the only real LLM backends currently supported.
- **New package:** `pkg/provider/gemini/` implementing the `provider.Provider` interface.
- **API Integration:** Integrate with the [Google Gemini API](https://ai.google.dev/) (`generativelanguage.googleapis.com`). Gemini uses a different REST schema than OpenAI, so the adapter must translate between the OpenAI-compatible request/response structs (`pkg/api`) and Gemini's native format.
- **Streaming:** Implement `StreamChatCompletion` using Gemini's SSE-based streaming endpoint, reusing `pkg/provider/sse` where applicable.
- **Authentication:** API key injected via `GEMINI_API_KEY` environment variable, managed through a Kubernetes Secret.
- **Config Support:** Register `type: gemini` in `pkg/server/watcher.go` `reloadConfig` switch statement so the provider can be activated via ConfigMap.
- **Pricing:** Expose `prompt_price` and `completion_price` fields in the ConfigMap provider entry (same schema as existing providers).
- **Testing:** Unit tests in `pkg/provider/gemini/gemini_test.go` covering non-streaming and streaming paths using an `httptest` server to mock the Gemini API.
