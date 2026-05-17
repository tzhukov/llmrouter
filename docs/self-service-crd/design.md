# Design: Scalable K8s Self-Service (Centralized Control Plane)

## Overview
This document outlines a "Control Plane" architecture for `llmrouter`, inspired by **Istio Ambient** and **xDS**. It avoids the resource overhead of sidecars and the 1MiB limit of ConfigMaps by using a centralized configuration service.

## Architecture: The "Control Plane" Pattern
This pattern decouples the "Watching" of the K8s API from the "Execution" of routing, allowing for extreme scale and minimal resource usage.

### 1. The `llm-config-server` (The Control Plane)
A high-availability deployment with **3 replicas** that:
- **Watches:** Uses `client-go` to watch `Agent` CRDs across the cluster.
- **Aggregates:** Compiles all agents into a unified, versioned registry.
- **Serves:** Provides a lightweight **Streaming API** (Server-Sent Events) at `/v1/sync`.

### 2. The `llmrouter` (The Data Plane)
The router instances act as "subscribers" to the control plane:
- **Client Mode:** On startup, the router connects to the `llm-config-server`.
- **Streaming Updates:** It receives the initial state and any subsequent incremental updates over the stream.
- **In-Memory Registry:** Updates its internal `RouterRegistry` in real-time.
- **Low Overhead:** No sidecar, no local file-watching, and minimal CPU/RAM usage.

### 3. Portability & Standalone Mode
To maintain standalone support, the `llmrouter` uses a provider-agnostic loading strategy:
- **Mode A (K8s):** `--config-source=http://llm-config-server/v1/sync`
- **Mode B (Standalone):** `--config-source=file:///etc/llmrouter/agents.yaml`

## Why this solves the Scaling/Cost issue
- **Reduced API Pressure:** Only 3 pods watch the K8s API, rather than 100 or 1,000.
- **Resource Efficiency:** No extra sidecar containers per pod. The `llmrouter` binary handles the lightweight HTTP stream natively.
- **No 1MB Limit:** Configuration is streamed as a payload; it never touches the K8s ConfigMap storage.
- **Instant Propagation:** Updates are pushed via long-lived connections, resulting in sub-second config propagation across the entire cluster.

## Staged Implementation Plan

### Phase 1: Control Plane (Config Server)
- Build `cmd/config-server` using `client-go`.
- Implement a simple SSE (Server-Sent Events) endpoint to stream agent updates.

### Phase 2: Router Remote Provider
- Implement a `RemoteConfigProvider` in `pkg/config`.
- Add logic to `llmrouter` to subscribe to the stream and update the registry.

### Phase 3: Helm Chart & Infrastructure
- Define the `llm-config-server` deployment (3 replicas) and service.
- Update `llmrouter` to point to the internal service URL.

## Benefits
- **Ambient-style Isolation:** Separation of configuration management from request processing.
- **Infinite Scalability:** Easily handles tens of thousands of agents.
- **Developer Portability:** The same binary works locally and in production.
