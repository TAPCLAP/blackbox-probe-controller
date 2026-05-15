# blackbox-probe-controller Helm chart

Helm chart to deploy [blackbox-probe-controller](https://github.com/TAPCLAP/blackbox-probe-controller) — an operator that discovers annotated Ingress resources in remote clusters and creates VMProbe resources in the home cluster.

## Prerequisites

- Kubernetes 1.25+
- VictoriaMetrics Operator with `VMProbe` CRD installed in the **home** cluster
- Namespace for VMProbes (default `monitoring`) — the chart does **not** create it
- Blackbox exporter available at the URL configured in `controller.blackboxProberURL`

## Installing the chart

### From GHCR (OCI)

```bash
helm registry login ghcr.io -u YOUR_GITHUB_USER

helm upgrade --install blackbox-probe-controller \
  oci://ghcr.io/tapclap/charts/blackbox-probe-controller \
  --version 0.1.0 \
  --namespace blackbox-probe-controller-system \
  --create-namespace
```

### From local chart directory

```bash
helm upgrade --install blackbox-probe-controller \
  ./deploy/helm/blackbox-probe-controller \
  --namespace blackbox-probe-controller-system \
  --create-namespace
```

## Required post-install steps

1. Ensure namespace `controller.vmprobeNamespace` exists (default: `monitoring`).

2. Create a cluster Secret in the operator namespace (release namespace by default):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cluster-example
  namespace: blackbox-probe-controller-system
  labels:
    blackbox-probe-controller.tapclap.com/cluster-config: "true"
  annotations:
    blackbox-probe-controller.tapclap.com/cluster-name: my-remote-cluster
type: Opaque
stringData:
  kubeconfig: |
    # paste remote cluster kubeconfig here
```

3. Annotate Ingress resources in the **remote** cluster:

```yaml
metadata:
  annotations:
    blackbox-probe-controller.tapclap.com/enabled: "true"
    blackbox-probe-controller.tapclap.com/probe-path: "/ready"   # optional
```

## Configuration

### Image

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Container image repository | `ghcr.io/tapclap/blackbox-probe-controller` |
| `image.tag` | Image tag (defaults to chart `appVersion` if empty) | `""` |
| `image.pullPolicy` | Pull policy | `IfNotPresent` |
| `imagePullSecrets` | Image pull secrets | `[]` |

### Operator

| Parameter | Description | Default |
|-----------|-------------|---------|
| `controller.vmprobeNamespace` | Namespace for VMProbe CRs | `monitoring` |
| `controller.clusterSecretNamespace` | Namespace to watch cluster Secrets; empty = release namespace | `""` |
| `controller.probeInterval` | VMProbe `interval` | `20s` |
| `controller.probeScrapeTimeout` | VMProbe `scrapeTimeout` | `18s` |
| `controller.probeModule` | Blackbox module | `http_2xx` |
| `controller.blackboxProberURL` | `vmProberSpec.url` | `blackbox.monitoring.svc:9115` |

### Deployment

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas (use 1 unless leader election is understood) | `1` |
| `leaderElection.enabled` | Pass `--leader-elect` to the manager | `true` |
| `rbac.create` | Create RBAC resources | `true` |
| `serviceAccount.create` | Create ServiceAccount | `true` |
| `resources` | CPU/memory limits and requests | see `values.yaml` |
| `namespaceOverride` | Override operator namespace | `""` |
| `nameOverride` / `fullnameOverride` | Resource naming | `""` |

### Probes

| Parameter | Description |
|-----------|-------------|
| `livenessProbe` | HTTP GET `/healthz` on port `health` (8081) |
| `readinessProbe` | HTTP GET `/readyz` on port `health` (8081) |

## Examples

### Custom blackbox URL and probe interval

```bash
helm upgrade --install blackbox-probe-controller \
  oci://ghcr.io/tapclap/charts/blackbox-probe-controller \
  --namespace blackbox-probe-controller-system \
  --create-namespace \
  --set controller.blackboxProberURL=blackbox-exporter.monitoring.svc.cluster.local:9115 \
  --set controller.probeInterval=30s \
  --set controller.probeScrapeTimeout=25s
```

### Pin image tag

```bash
helm upgrade --install blackbox-probe-controller \
  oci://ghcr.io/tapclap/charts/blackbox-probe-controller \
  --version 0.1.0 \
  --namespace blackbox-probe-controller-system \
  --create-namespace \
  --set image.tag=v0.1.0
```

### PR preview build

Pull request builds publish temporary artifacts (see GitHub Actions `pr-preview` workflow). Example:

```bash
helm upgrade --install blackbox-probe-controller \
  oci://ghcr.io/tapclap/charts/blackbox-probe-controller \
  --version 0.0.0-pr.42.abc1234 \
  --namespace blackbox-probe-controller-system \
  --create-namespace \
  --set image.repository=ghcr.io/tapclap/blackbox-probe-controller \
  --set image.tag=pr-42-abc1234
```

## Uninstall

```bash
helm uninstall blackbox-probe-controller -n blackbox-probe-controller-system
```

This removes the operator deployment. Existing VMProbe objects are not automatically deleted unless you remove cluster Secrets / Ingress annotations first or clean up manually in the `monitoring` namespace.

## Upgrading

```bash
helm upgrade blackbox-probe-controller \
  oci://ghcr.io/tapclap/charts/blackbox-probe-controller \
  --version <new-version> \
  -n blackbox-probe-controller-system
```

## Troubleshooting

| Symptom | Check |
|---------|--------|
| No VMProbes created | VMProbe CRD present? `monitoring` namespace exists? Ingress annotated in **remote** cluster? |
| Operator logs: kubeconfig error | Secret has key `kubeconfig` and label `cluster-config=true` |
| VMProbes empty / stale | Remote SA can list/watch Ingress; cluster-name annotation set |
| Wrong probe URL | `probe-path` annotation; TLS on Ingress (https vs http) |

```bash
kubectl logs -n blackbox-probe-controller-system \
  -l app.kubernetes.io/name=blackbox-probe-controller -c manager -f
```

## Chart development

```bash
helm lint deploy/helm/blackbox-probe-controller
helm template test deploy/helm/blackbox-probe-controller \
  --namespace blackbox-probe-controller-system
```

## Links

- [Project README](../../../README.md)
- [VictoriaMetrics VMProbe](https://docs.victoriametrics.com/operator/resources/vmprobe/)
- [Sample manifests](../../../config/samples/)
