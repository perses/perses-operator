# Perses Operator Metrics

The Perses Operator exposes Prometheus metrics for monitoring operator health and performance.

## Authentication and Authorization

The metrics endpoint uses controller-runtime's built-in `SecureServing` with
`filters.WithAuthenticationAndAuthorization` to protect the `/metrics` endpoint.
This replaces the previous kube-rbac-proxy sidecar approach.

When `--metrics-secure=true` (the default), every request to `/metrics` is
authenticated via Kubernetes **TokenReview** and authorized via
**SubjectAccessReview**. Clients must present a valid bearer token for a
service account that has `get` permission on the `/metrics` non-resource URL
(granted by the `metrics-reader` ClusterRole).

This approach differs from kube-rbac-proxy's optional mutual TLS (mTLS) client
certificate authentication. Instead of client certificates, callers
authenticate with bearer tokens, which is the standard mechanism recommended by
Kubebuilder since v4.1.0 (see the [Kubebuilder Metrics reference](https://book.kubebuilder.io/reference/metrics.html)
and the [design doc](https://github.com/kubernetes-sigs/kubebuilder/blob/master/designs/discontinue_usage_of_kube_rbac_proxy.md)).

## TLS Certificates

When secure metrics is enabled, the metrics server reuses the same TLS
certificates as the webhook server. These certificates are provisioned by
cert-manager and mounted at the path specified by `--webhook-cert-dir`
(default `/tmp/k8s-webhook-server/serving-certs`). The cert-manager
Certificate includes DNS names for both the webhook and metrics services,
allowing a single certificate to serve both endpoints.

To use HTTP instead of HTTPS (not recommended for production), set
`--metrics-secure=false` and `--metrics-bind-address=:8080`.

## Accessing Metrics

Metrics are exposed on port `8443` over HTTPS at the `/metrics` endpoint with authentication and authorization enabled:

```bash
# Port forward to the operator pod
kubectl port-forward -n perses-operator-system \
  deployment/perses-operator-controller-manager 8443:8443

# View metrics (requires a valid bearer token)
curl -sk https://localhost:8443/metrics \
  -H "Authorization: Bearer $(kubectl create token -n perses-operator-system perses-operator-controller-manager)"
```

## Available Metrics

### `perses_operator_syncs`

Number of objects per sync status (ok/failed)

**Type:** Gauge  
**Labels:**

- `status`



---

### `perses_operator_managed_resources`

Number of resources managed by the operator per state (synced/failed)

**Type:** Gauge  
**Labels:**

- `resource`
- `state`



---

### `perses_operator_reconcile_operations_total`

Total number of reconciliation operations by controller

**Type:** Counter  
**Labels:**

- `controller`



---

### `perses_operator_reconcile_errors_total`

Total number of reconciliation errors by controller and reason

**Type:** Counter  
**Labels:**

- `controller`
- `reason`



---

### `perses_operator_managed_perses_instances`

Number of Perses instances managed by the operator

**Type:** Gauge  
**Labels:**

- `resource_namespace`



---

### `perses_operator_ready`

Whether the operator is ready (1=yes, 0=no)

**Type:** Gauge  
**Labels:**

- `controller`



---


## Standard Controller-Runtime Metrics

In addition to custom metrics, the operator exposes standard controller-runtime metrics:

- `controller_runtime_reconcile_total`: Total number of reconciliations per controller
- `controller_runtime_reconcile_errors_total`: Total number of reconciliation errors
- `controller_runtime_reconcile_time_seconds`: Length of time per reconciliation
- `workqueue_*`: Work queue metrics (depth, duration, etc.)
- `rest_client_*`: Kubernetes API client metrics

See [controller-runtime metrics](https://book.kubebuilder.io/reference/metrics-reference.html) for details.

---

*This documentation is auto-generated from the metrics code. Do not edit manually.*
*Run `make generate-metrics-docs` to regenerate.*
