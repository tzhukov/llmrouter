# Multi-Provider LLM Router

A high-performance, Go-based proxy server that routes LLM requests across multiple providers (OpenAI, Groq, Anthropic, etc.) based on cost, latency, and availability.

## 🚀 Key Features

- **Unified OpenAI Interface:** Drop-in replacement for OpenAI-compatible tools and SDKs.
- **Intelligent Routing:** Optimize for **Cost** ($ per 1k tokens) or **Latency** (Moving Average).
- **Resilience:** Automatic failover and retry logic across multiple provider pools.
- **K8s-Native:** Built for Kubernetes with **Hot Reloading** from ConfigMaps and secure API key injection via Secrets.
- **Observability-First:** Standardized Prometheus metrics, structured JSON logging, and request-id tracing.
- **MockLLM:** First-class support for simulating LLM backends for testing and local development.

## 🏗️ Architecture

The router acts as a stateless proxy between your application and various LLM providers.

```text
[ Client ] -> [ Router Proxy ] -> [ Strategy Engine ] -> [ Provider Adapters ]
                                          |                     |
                                  [ Metrics/Stats ]      [ OpenAI/Groq/Gemini/Mock ]
```

## 🛠️ Getting Started

### Prerequisites
- Go 1.22+
- Docker
- Helm (for Kubernetes deployment)
- Tilt (for local development)

### Local Development (Tilt)
The fastest way to get started is using [Tilt](https://tilt.dev/):
```bash
tilt up
```
This will build the container, deploy it to your local Kubernetes cluster (Kind/Minikube), and set up port-forwarding to `:8080`.

### Local Run (Go)
```bash
export CONFIG_PATH=config.yaml
go run cmd/server/main.go
```

## ⚙️ Configuration

The router is configured via a YAML file. Environment variables like `${OPENAI_API_KEY}` are automatically expanded.

```yaml
routing:
  strategy: "latency" # options: round-robin, cost, latency
  failover: true
  retries: 3

providers:
  - name: openai-gpt4
    type: openai
    api_key: ${OPENAI_API_KEY}
    prompt_price: 0.03
    completion_price: 0.06
  - name: groq-llama3
    type: groq
    api_key: ${GROQ_API_KEY}
    prompt_price: 0.0005
    completion_price: 0.0008
```

## 📊 Observability

- **Metrics:** `GET /metrics` (Prometheus format)
- **Health:** `GET /health`
- **Logs:** Structured JSON logs with `request_id` correlation.

## 📄 Documentation

Detailed phase-by-phase documentation and design decisions can be found in the [docs/](./docs/) folder.

- [Design Document](./docs/DESIGN.md)
- [Project Assessment](./docs/copilot-assesment.md)

## ⚖️ License
MIT
