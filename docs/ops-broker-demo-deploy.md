# Ops Broker Demo Deployment

This runbook describes the remaining deployment plumbing needed to run
broker-driven chaos tests against the OpenCost demo cluster.

The repository contains the broker code, manifests, RBAC, `pkg/cluster` client,
and chaos tests. A user with demo-cluster access still needs to build/publish the
image and apply the manifests.

## Prerequisites

- Access to the demo cluster with permission to install CRDs/controllers and
  apply resources in the `opencost` namespace.
- A container registry reachable by the demo cluster.
- Chaos Mesh installed in the target cluster.
- Confirmed labels/namespaces for OpenCost and Prometheus.

Expected defaults:

```text
BROKER_NAMESPACE=opencost
BROKER_OPENCOST_DEPLOYMENT=opencost
BROKER_OPENCOST_SELECTOR=app.kubernetes.io/name=opencost
BROKER_PROMETHEUS_NAMESPACE=prometheus-system
BROKER_PROMETHEUS_SELECTOR=app.kubernetes.io/name=prometheus,app.kubernetes.io/component=server
BROKER_CHAOS_NAMESPACE=opencost
```

If demo uses different labels or namespaces, update
`deploy/ops-broker/deployment.yaml` and the namespace-scoped RBAC before
deploying.

## Build And Publish

From the repository root:

```bash
docker build -f cmd/ops-broker/Dockerfile -t <registry>/ops-broker:<tag> .
docker push <registry>/ops-broker:<tag>
```

Update the image:

```bash
kubectl kustomize deploy/ops-broker \
  | sed 's#REPLACE_ME/ops-broker:latest#<registry>/ops-broker:<tag>#' \
  > /tmp/ops-broker-rendered.yaml
```

Alternatively, edit `deploy/ops-broker/kustomization.yaml` with the real image
name and tag.

## Install Chaos Mesh

Example local/dev install:

```bash
helm repo add chaos-mesh https://charts.chaos-mesh.org
helm repo update
helm install chaos-mesh chaos-mesh/chaos-mesh -n chaos-mesh --create-namespace
```

For demo, use the platform team's preferred install path. The broker requires
the `podchaos.chaos-mesh.org` and `networkchaos.chaos-mesh.org` CRDs.

## Create Broker Token

Do not commit the real token.

```bash
export OPENCOST_BROKER_TOKEN="$(openssl rand -base64 32)"

kubectl -n opencost create secret generic ops-broker-auth \
  --from-literal=token="${OPENCOST_BROKER_TOKEN}"
```

The same value is passed to tests as `OPENCOST_BROKER_TOKEN`.

## Deploy Broker

```bash
kubectl apply -k deploy/ops-broker
```

If Prometheus runs outside the `opencost` namespace, also apply the matching
Chaos Mesh RBAC in that namespace. The checked-in example grants the broker
ServiceAccount permission to create and clean up broker-owned Chaos Mesh
resources in `prometheus-system`:

```bash
kubectl apply -f deploy/ops-broker/prometheus-chaos-rbac.yaml
```

If demo uses a different Prometheus namespace, update the namespace in that
file before applying it. This is required because Chaos Mesh admission checks
the broker ServiceAccount against the namespace targeted by each chaos object.

If using the rendered image file instead:

```bash
kubectl apply -f /tmp/ops-broker-rendered.yaml
```

Wait for rollout:

```bash
kubectl -n opencost rollout status deployment/ops-broker
```

## Smoke Test

For a private/internal deployment, port-forward first:

```bash
kubectl -n opencost port-forward svc/ops-broker 8080:80
```

Then in another shell:

```bash
export OPENCOST_BROKER_URL="http://localhost:8080"
export OPENCOST_BROKER_TOKEN="<same token from the Secret>"
./scripts/smoke-ops-broker.sh
```

The smoke script checks:

- `GET /healthz`
- `GET /v1/pods`
- `GET /v1/chaos`

It does not inject chaos.

## First Chaos Check

After smoke passes and the demo owner confirms it is safe to mutate the demo
cluster, run one scenario:

```bash
export CHAOS_ENABLED=1
export OPENCOST_URL="https://demo.infra.opencost.io/model"
export OPENCOST_BROKER_URL="http://localhost:8080"
export OPENCOST_BROKER_TOKEN="<same token from the Secret>"

go test ./test/chaos -run 'TestChaosScenarios/Network Latency Injection' -v
```

Use the lowest-risk scenario first. Cleanup is called automatically by the test,
but broker-owned Chaos Mesh resources can also be removed directly by deleting
resources labeled:

```text
opencost.io/broker-owned=true
```

## Remaining Demo Work

- Replace the placeholder image in manifests or kustomization.
- Decide how CI reaches the broker: private ingress, internal service, or
  port-forward in a trusted setup step.
- Confirm the `NetworkPolicy` works with the demo cluster CNI and does not block
  kubelet liveness probes.
- Confirm Chaos Mesh behavior and labels in demo before enabling all scenarios.
