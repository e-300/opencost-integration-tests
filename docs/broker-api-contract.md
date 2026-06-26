# Broker API Contract

This document is the source of truth for the trusted broker HTTP API.

The broker exists because integration test runners are untrusted. Tests must not
receive kubeconfig, cloud credentials, or permission to run arbitrary `kubectl`
commands. Instead, tests call fixed broker endpoints. The broker validates each
request, runs only allowlisted operations, and returns trimmed JSON responses.

This contract freezes paths, verbs, request shapes, response shapes, and
allowlisted operation names. The broker implements the server side. `pkg/cluster`
implements the caller side. Integration tests should call `pkg/cluster`, not raw
HTTP.

## Base URL

The broker base URL comes from:

```text
OPENCOST_BROKER_URL
```

## Authentication

Every request must include:

```text
Authorization: Bearer <OPENCOST_BROKER_TOKEN>
```

The token authenticates to the broker only. It is not a Kubernetes credential or
cloud credential.

## Response Rules

All successful responses are JSON unless an endpoint explicitly documents a
different content type.

All JSON error responses use:

```json
{
  "error": "human readable reason",
  "code": "INVALID_PARAM"
}
```

Status codes:

| Status | Meaning |
| --- | --- |
| `200` | Success |
| `400` | Invalid request input |
| `401` | Missing or invalid broker token |
| `404` | Unknown endpoint, fixture, or chaos scenario |
| `409` | Precondition failed, not ready, or conflicting state |
| `502` | Upstream Kubernetes or cluster failure |
| `500` | Broker failure |

## Endpoint Summary

| Verb | Path | Status | Purpose |
| --- | --- | --- | --- |
| `GET` | `/healthz` | Live | Broker liveness |
| `GET` | `/v1/pods` | Live | Read trimmed OpenCost pod state |
| `POST` | `/v1/restart` | Live | Restart pinned OpenCost deployment |
| `GET` | `/v1/chaos` | Live | List allowlisted chaos scenarios |
| `GET` | `/v1/nodes` | Future | Read trimmed node facts |
| `GET` | `/v1/deployments/{name}` | Live | Read pinned deployment readiness |
| `GET` | `/v1/logs` | Future | Read trimmed logs |
| `POST` | `/v1/config` | Future | Apply allowlisted fixture config |
| `DELETE` | `/v1/config` | Future | Remove allowlisted fixture config |
| `POST` | `/v1/chaos/{scenario}` | Live | Inject allowlisted chaos scenario |
| `DELETE` | `/v1/chaos/{scenario}` | Live | Cleanup allowlisted chaos scenario |

## Frozen Identifiers

Fixture IDs:

| ID | Purpose |
| --- | --- |
| `pricing-fixed-v1` | Asset pricing fixture for asset ground-truth tests |

Chaos scenarios:

| Scenario | Purpose |
| --- | --- |
| `kill-opencost` | Delete or restart the allowlisted OpenCost pod/deployment |
| `kill-prometheus` | Delete or restart the allowlisted Prometheus pod/deployment |
| `partition-prometheus` | Isolate OpenCost from Prometheus |
| `latency-prometheus` | Add latency between OpenCost and Prometheus |

Pinned deployment:

| Field | Value |
| --- | --- |
| Namespace | `opencost` |
| Deployment | `opencost` |

Adding a fixture ID, scenario, endpoint, request field, or response field is a
contract change and must be coordinated across broker, `pkg/cluster`, and tests.

## Test Consumer Mapping

Integration tests should not build broker URLs themselves. They should call
`pkg/cluster`, and `pkg/cluster` should be the only test-side package that knows
these paths and JSON shapes.

Current planned consumers:

| Test area | Contract operations used |
| --- | --- |
| Chaos testing | Live: `GET /v1/chaos`, `POST /v1/chaos/{scenario}`, `DELETE /v1/chaos/{scenario}`; optionally `GET /v1/pods` for recovery checks |
| Restart recovery | Live: `POST /v1/restart`, `GET /v1/deployments/{name}`, `GET /v1/pods` |
| Asset ground truth | `GET /v1/nodes`, `POST /v1/config`, `DELETE /v1/config` |

## Broker Metadata Endpoints

### `GET /healthz`

Purpose: unauthenticated liveness for the broker process.

Request: no body.

Response:

```json
{
  "status": "ok"
}
```

Validation:

- No query parameters are accepted.
- The response must not include cluster credentials, kubeconfig, cloud
  credentials, or environment variables.
- This endpoint does not require `Authorization`.

RBAC:

- None. This endpoint must not require Kubernetes API access.

### `GET /v1/chaos`

Purpose: list the allowlisted chaos scenarios supported by this broker. Chaos
tests use this as a preflight before requesting injection.

Request: no body.

Response:

```json
{
  "scenarios": [
    {
      "id": "kill-opencost",
      "description": "Delete or restart the allowlisted OpenCost pod/deployment",
      "engine": "chaos-mesh"
    },
    {
      "id": "kill-prometheus",
      "description": "Delete or restart the allowlisted Prometheus pod/deployment",
      "engine": "chaos-mesh"
    },
    {
      "id": "partition-prometheus",
      "description": "Isolate OpenCost from Prometheus",
      "engine": "chaos-mesh"
    },
    {
      "id": "latency-prometheus",
      "description": "Add latency between OpenCost and Prometheus",
      "engine": "chaos-mesh"
    }
  ]
}
```

Validation:

- No query parameters are accepted.
- Returned IDs must be a subset of the frozen chaos scenario identifiers.
- The broker must not expose raw Kubernetes selectors, manifests, namespaces, or
  implementation details needed to mutate the cluster directly.

RBAC:

- None. This endpoint reports broker capabilities and must not require
  Kubernetes API access.

## Read Endpoints

### `GET /v1/nodes`

Purpose: return trimmed node facts for asset ground-truth tests.

Request: no body.

Response:

```json
{
  "nodes": [
    {
      "name": "node-1",
      "cpu": "4",
      "ram": "16Gi"
    }
  ]
}
```

Validation:

- No query parameters are accepted.
- The broker must return only the documented fields.

RBAC:

- `get`, `list` on `nodes`

### `GET /v1/pods`

Purpose: list trimmed OpenCost pod state for wait-for-ready checks after a
restart. The namespace and selector are fixed broker-side.

Request: no body.

Response:

```json
{
  "pods": [
    {
      "name": "opencost-abc123",
      "phase": "Running",
      "ready": true,
      "restartCount": 0
    }
  ]
}
```

If no pods match, the broker returns:

```json
{
  "pods": []
}
```

Validation:

- No query parameters are accepted in the live contract.
- Namespace and selector are fixed broker-side.
- The broker must return only the documented fields.

RBAC:

- `get`, `list` on `pods`

### `GET /v1/deployments/{name}`

Purpose: read readiness for the allowlisted OpenCost deployment.

Request: no body.

Path parameters:

| Name | Required | Notes |
| --- | --- | --- |
| `name` | Yes | Must match the pinned deployment name |

Response:

```json
{
  "name": "opencost",
  "ready": true,
  "replicas": 1,
  "readyReplicas": 1,
  "updatedReplicas": 1,
  "availableReplicas": 1,
  "observedGeneration": 3,
  "generation": 3
}
```

Validation:

- Deployment name must match the pinned allowlisted deployment.
- Namespace is fixed broker-side.
- The broker must return only the documented fields.

RBAC:

- `get` on `deployments`

### `GET /v1/logs?namespace=<ns>&selector=<labelSelector>&container=<name>&tailLines=<n>`

Purpose: return trimmed logs for panic/error checks.

Request: no body.

Query parameters:

| Name | Required | Notes |
| --- | --- | --- |
| `namespace` | Yes | Must match an allowlisted namespace |
| `selector` | Yes | Kubernetes label selector, validated by the broker |
| `container` | No | Container name, validated by the broker |
| `tailLines` | No | Defaults to `200`, max `2000` |

Response:

```json
{
  "lines": [
    "log line 1",
    "log line 2"
  ]
}
```

Validation:

- Namespace must be allowlisted.
- `tailLines` must be between `1` and `2000`.
- The broker must not expose full pod specs, env vars, or secrets.

RBAC:

- `get`, `list` on `pods`
- `get` on `pods/log`

## Config Endpoints

### `POST /v1/config`

Purpose: apply a known fixture configuration, such as fixed asset pricing.

Request:

```json
{
  "fixtureId": "pricing-fixed-v1"
}
```

Response:

```json
{
  "applied": true,
  "fixtureId": "pricing-fixed-v1"
}
```

Validation:

- Fixture ID must be allowlisted.
- The caller must never provide raw Kubernetes manifests, cloud credentials, or
  arbitrary file paths.

RBAC:

- `create`, `update`, `patch`, `delete` permissions are `TODO` pending the
  chosen fixture application method.

### `DELETE /v1/config`

Purpose: clean up a previously applied fixture configuration.

Request:

```json
{
  "fixtureId": "pricing-fixed-v1"
}
```

Response:

```json
{
  "deleted": true,
  "fixtureId": "pricing-fixed-v1"
}
```

Validation:

- Fixture ID must be allowlisted.
- Cleanup must affect only resources owned by the named fixture.

RBAC:

- `delete` permissions are `TODO` pending the chosen fixture application method.

## Mutation Endpoints

### `POST /v1/restart`

Purpose: rollout-restart the single allowlisted OpenCost deployment.

Request: no body. The target namespace and deployment are fixed broker-side.

Response:

```json
{
  "status": "restart triggered"
}
```

Validation:

- No request body is accepted in the live contract.
- The broker must not accept arbitrary deployment names or namespaces from the
  caller.

RBAC:

- `get`, `patch` on `deployments`

### `POST /v1/chaos/{scenario}`

Purpose: run a single allowlisted chaos scenario.

Request:

No body by default. Scenario-specific parameters require a contract update.

Path parameters:

| Name | Required | Notes |
| --- | --- | --- |
| `scenario` | Yes | Must be one of the frozen chaos scenarios |

Response:

```json
{
  "injected": true,
  "scenario": "kill-opencost"
}
```

Validation:

- Unknown scenario returns `404`.
- Scenario implementation must be fixed code, not caller-supplied commands.
- The broker must never execute caller-provided shell strings.

RBAC:

- For Chaos Mesh, expected permissions are `get`, `list`, `watch`, `create`,
  and `delete` on the allowlisted chaos resource kinds used by the scenario.
- Sensitive grants such as `pods/exec` require explicit review and sign-off.

### `DELETE /v1/chaos/{scenario}`

Purpose: clean up one broker-owned chaos scenario.

Request:

No body by default. Scenario-specific parameters require a contract update.

Path parameters:

| Name | Required | Notes |
| --- | --- | --- |
| `scenario` | Yes | Must be one of the frozen chaos scenarios |

Response:

```json
{
  "deleted": true,
  "scenario": "kill-opencost"
}
```

Validation:

- Unknown scenario returns `404`.
- Cleanup must affect only resources owned by the named scenario.
- Cleanup must not delete arbitrary pods, services, deployments, namespaces, or
  caller-provided resource names.

RBAC:

- For Chaos Mesh, expected permissions are `get`, `list`, `watch`, `create`,
  and `delete` on the allowlisted chaos resource kinds used by the scenario.

## Security Rules

- No endpoint accepts arbitrary shell commands.
- No endpoint accepts arbitrary Kubernetes manifests.
- No endpoint accepts arbitrary file paths.
- No endpoint returns Kubernetes Secrets, cloud credentials, env vars, or full pod
  specs.
- All mutating operations must be allowlisted by fixture ID, deployment name, or
  chaos scenario.
- Broker logs should audit mutating operations: caller identity, operation,
  target, result, and timestamp.

## Development Order

1. Freeze this contract.
2. Build a stub broker that returns these shapes.
3. Build `pkg/cluster` against the contract.
4. Build RBAC and ServiceAccount for the real broker.
5. Replace stub broker handlers with real Kubernetes fixture logic.
6. Wire integration tests through `pkg/cluster`.
