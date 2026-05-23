# Multi-Provider LLM Router

A high-performance, Go-based proxy server that routes LLM requests across multiple providers (OpenAI, Groq, Anthropic, etc.) based on cost, latency, and availability.

## 🚀 Key Features

- **Unified OpenAI Interface:** Drop-in replacement for OpenAI-compatible tools and SDKs.
- **Per-Agent Multi-Tenancy:** Define independent routing policies, providers, and parameters per agent ID.
- **Scalable Ambient Control Plane:** Kubernetes-native self-service via CRDs with a high-availability (3 replicas) config server and real-time SSE streaming.
- **Intelligent Routing:** Optimize for **Cost** ($ per 1k tokens) or **Latency** (Moving Average).
- **Resilience:** Automatic failover and retry logic across multiple provider pools.
- **K8s-Native:** Built for Kubernetes with hot reloading, secure API key injection, and least-privilege RBAC.
- **Observability-First:** Standardized Prometheus metrics (per-agent), structured JSON logging, and request-id tracing.
- **Claude Code Ready:** Includes built-in LiteLLM support for Anthropic Messages API compatibility.

## 🏗️ Architecture

The router uses an "Ambient" architecture where the Control Plane is decoupled from the Data Plane for maximum scalability.

```text
[ K8s API (Agents CRD) ] 
         |
[ HA Config Server (3 reps) ] --- (SSE Stream) --- [ Router Pod 1 ] --- [ Providers ]
                                              \--- [ Router Pod N ] --- [ Providers ]
```

## 🛠️ Getting Started

### Prerequisites
- Go 1.26+
- Docker
- Helm (for Kubernetes deployment)
- Tilt (for local development)

### Local Development (Tilt)
```bash
tilt up
```
This builds the router, deploys the HA Config Server, and sets up LiteLLM for Claude Code compatibility on `:8000`.

## ⚙️ Configuration

The router supports both local file configuration and remote streaming.

### Agent-Based YAML (File Mode)
```yaml
agents:
  researcher:
    routing:
      strategy: "latency"
    providers:
      - name: openai-gpt4
        type: openai
        api_key: ${OPENAI_API_KEY}
  coder:
    routing:
      strategy: "cost"
    providers:
      - name: groq-llama3
        type: groq
        api_key: ${GROQ_API_KEY}
```

## 📊 Observability

- **Metrics:** `GET /metrics` (Includes `agent_id` labels for cost tracking)
- **Health:** `GET /health`
- **Logs:** Structured JSON logs.

## 📄 Documentation

- [Per-Agent Configuration (Implementation)](./docs/per_agent_config/design.md)
- [Self-Service CRD & Control Plane](./docs/self-service-crd/design.md)
- [Security & Performance Audit](./docs/audit_and_benchmarking.md)
- [Legacy Design Document](./docs/poc/DESIGN.md)

## ⚖️ License
MIT
