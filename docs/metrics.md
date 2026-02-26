# Perses Operator Metrics

The Perses Operator exposes Prometheus metrics for monitoring operator health and performance.

## Accessing Metrics

Metrics are exposed on port `8082` at the `/metrics` endpoint:

```bash
# Port forward to the operator pod
kubectl port-forward -n perses-operator-system \
  deployment/perses-operator-controller-manager 8082:8082

# View metrics
curl http://localhost:8082/metrics
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
