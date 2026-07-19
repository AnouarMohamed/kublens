# Kubernetes Deployment Manifests

This directory uses a **base + overlays** structure.

## Layout

- `base/`: shared resources (namespace, config, deployments, services, NetworkPolicies, PDB, HPA)
- `overlays/dev`: development profile (operator RBAC + dev config)
- `overlays/demo`: read-focused demo profile (readonly RBAC)
- `overlays/prod`: production profile (auth required + stricter defaults)
- `overlays/tracing`: demo profile with OpenTelemetry wired to the in-cluster Jaeger backend
- `overlays/observability`: demo profile with Prometheus + Grafana dashboards
- `secret.example.yaml`: secrets template

Each overlay includes explicit RBAC manifests (`clusterrole.yaml`, `clusterrolebinding.yaml`) so `kubectl kustomize` and CI validation work with root-only load restrictions.

RBAC note:

- Overlays intentionally do not grant `secrets` read access by default.
- If secret inventory is required in your environment, extend overlay RBAC explicitly.

## Deploy

### Dev overlay

```bash
kubectl apply -k k8s/overlays/dev
```

### Demo overlay

```bash
kubectl apply -k k8s/overlays/demo
```

### Prod overlay

1. Create secret:

```bash
cp k8s/secret.example.yaml k8s/secret.yaml
# fill values, especially AUTH_TOKENS, DATABASE_URL, KUBECONFIG_DATA, and AUDIT_SIGNING_KEY
kubectl apply -f k8s/secret.yaml
```

The prod overlay sets `DATABASE_DRIVER=postgres`, `MEMORY_STORE=sql`, `AUDIT_STORE=sql`, and probes `/api/readiness/production`, so `DATABASE_URL` must point to a reachable Postgres database before the deployment can become ready.

2. Apply overlay:

```bash
kubectl apply -k k8s/overlays/prod
```

### Tracing overlay

```bash
kubectl apply -k k8s/overlays/tracing
```

Jaeger UI access (port-forward):

```bash
kubectl -n kubernetes-operations-dashboard port-forward svc/k8s-ops-jaeger 16686:16686
```

### Observability overlay (Prometheus + Grafana)

```bash
kubectl apply -k k8s/overlays/observability
```

Grafana UI access (port-forward):

```bash
kubectl -n kubernetes-operations-dashboard port-forward svc/k8s-ops-grafana 3001:3000
```

Prometheus access (port-forward):

```bash
kubectl -n kubernetes-operations-dashboard port-forward svc/k8s-ops-prometheus 9090:9090
```

## Validate manifests locally

```bash
kubectl kustomize k8s/overlays/dev > /dev/null
kubectl kustomize k8s/overlays/demo > /dev/null
kubectl kustomize k8s/overlays/prod > /dev/null
kubectl kustomize k8s/overlays/tracing > /dev/null
kubectl kustomize k8s/overlays/observability > /dev/null
```

CI also validates rendered manifests with `kubeconform`.
