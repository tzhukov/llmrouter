# Phase 3: Helm Chart & Infrastructure - Completion Report

## Reasoning
To operationalize the "Ambient" Control Plane architecture, we needed to update the infrastructure manifests to deploy the `llm-config-server` and connect it to the `llmrouter`. This includes ensuring High Availability (HA) for the control plane and setting up the necessary security permissions (RBAC) to interact with the Kubernetes API.

## Changes Made
- **Helm Chart Infrastructure:**
    - **Config Server Deployment:** Created `charts/llmrouter/templates/config-server-deployment.yaml` with **3 replicas** for High Availability.
    - **Config Server Service:** Created `charts/llmrouter/templates/config-server-service.yaml` to provide a stable internal endpoint for the routers.
    - **RBAC Configuration:** Created `charts/llmrouter/templates/config-server-rbac.yaml` with a `ClusterRole` and `ClusterRoleBinding` allowing the config server to `get`, `list`, and `watch` the `Agent` CRDs.
- **Router Configuration:**
    - Updated `charts/llmrouter/templates/deployment.yaml` to dynamically switch between `file` and `remote` config sources based on `values.yaml`.
    - Automatically injects the correct `CONFIG_URL` when `remote` mode is enabled.
- **Values Schema:** Updated `charts/llmrouter/values.yaml` to support the new `configServer` and `config.source` settings.

## Proof of Work
- **Helm Validation:** Successfully validated the chart using `helm template`. The generated manifests correctly show:
    - 3 replicas for the `llmrouter-config-server`.
    - Correct environment variables (`CONFIG_SOURCE="remote"` and `CONFIG_URL`) in the `llmrouter` deployment.
    - Valid RBAC resources for the config server.
- **Service Discovery:** Verified that the router is configured to use the stable service name `http://llmrouter-config-server:8081/v1/sync` for discovery.
