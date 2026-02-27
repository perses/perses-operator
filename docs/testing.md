# Testing

The Perses Operator uses unit, integration, end-to-end (e2e), and alerting rule tests.

Some test commands use tools installed locally under `bin/`. Run `make build-tools` to install all tools, or let the Makefile targets (e.g. `make test-unit`, `make test-integration`) install them automatically.

## Unit Tests

Unit tests validate individual functions and utilities using standard Go testing with [testify](https://github.com/stretchr/testify) or [Ginkgo](https://github.com/onsi/ginkgo).

```bash
# Run all unit tests
make test-unit

# Run a single test by name (testify)
go test ./internal/metrics/... -run TestNewMetrics -v

# Run a single test by name (Ginkgo)
# --focus matches against Describe/It block descriptions
bin/ginkgo --focus "GetVolumes" -v ./internal/perses/common/...
```

## Integration Tests

Integration tests use [Ginkgo](https://github.com/onsi/ginkgo) with [envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/tools/setup-envtest) to spin up a lightweight API server with CRDs installed. These tests validate controller reconcile logic, CRD validation, and resource management.

```bash
# Run all integration tests
make test-integration

# Run a single integration test by name
# --focus matches against Ginkgo Describe/It block descriptions
KUBEBUILDER_ASSETS="$(bin/setup-envtest use -p path)" \
  bin/ginkgo --focus "should successfully reconcile" -v ./controllers/...
```

## E2E Tests (kuttl)

E2E tests use [kuttl](https://kuttl.dev/) to validate the operator end-to-end in a real [kind](https://kind.sigs.k8s.io/) cluster, including deployment, resource synchronization, and cleanup.

### Prerequisites

- [Go](https://go.dev/doc/install)
- [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- [Docker](https://docs.docker.com/get-docker/) or [Podman](https://podman.io/docs/installation) — the Makefile auto-detects the available container runtime
- [kubectl](https://kubernetes.io/docs/tasks/tools/)

### Quick Start

```bash
make e2e-setup    # Create cluster + deploy operator
make test-e2e     # Run tests
make e2e-cleanup  # Tear down
```

When iterating:

1. Run `make e2e-setup` once to create the cluster and deploy the operator.
2. Run `make test-e2e` to run the tests.
3. After code changes, run `make e2e-deploy` to rebuild and redeploy the operator, then `make test-e2e` again.

### Configuration

The following Makefile variables can be overridden to customize the e2e environment:

| Variable | Default | Description |
| -------- | ------- | ----------- |
| `KIND_CLUSTER_NAME` | `kuttl-e2e` | Name of the kind cluster |
| `E2E_TAG` | Git short SHA | Image tag for the operator |
| `E2E_IMG` | `docker.io/persesdev/perses-operator:<E2E_TAG>` | Full operator image reference |

### Test Structure

Tests are under `test/e2e/` following the [kuttl convention](https://kuttl.dev/docs/kuttl-test-harness.html). Kuttl creates a random namespace per test case for isolation. Steps run sequentially.

```text
test/e2e/
├── kuttl-test.yaml
├── global-datasource/      # Global datasource sync
├── multi-instance-sync/    # Dashboard/datasource sync across instances
├── namespace-isolation/    # Namespace-scoped resource isolation
├── perses-deployment/      # database.sql → Deployment
├── perses-emptydir/        # emptyDir storage
├── perses-statefulset/     # database.file → StatefulSet with PVC
├── perses-volumes/         # Custom volumes and volume mounts
└── resource-update/        # Update existing resources and assert reconciliation
```

- To add a new test case, create a directory under `test/e2e/`.
- To add steps to an existing test case, add numbered files (e.g. `05-install.yaml`, `05-assert.yaml`).

### Debugging

```bash
# Operator logs
kubectl logs -n perses-operator-system deployment/perses-operator-controller-manager -f

# Run a single test case
bin/kubectl-kuttl test --config test/e2e/kuttl-test.yaml --test perses-statefulset
```

## Alerting Rule Tests

> [!NOTE]
> Requires [`promtool`](https://prometheus.io/download/#prometheus).

Alerting rules are validated using [promtool](https://prometheus.io/docs/prometheus/latest/configuration/unit_testing_rules/). When adding or modifying rules, add corresponding test cases to `test/promtool/alerts_test.yaml` and run:

```bash
make test-alerts
```

See [Developer Guide](dev.md#modifying-or-adding-alerting-rules) for the full workflow.

## Integration vs E2E Tests

| Aspect | Integration Tests | E2E Tests |
| --- | --- | --- |
| Environment | Lightweight API server via envtest (no real cluster) | Real Kubernetes cluster via kind |
| Scope | Controller reconcile logic, CRD validation, status updates | Full operator lifecycle including pod scheduling and networking |
| Speed | Fast (seconds) | Slower (minutes, includes cluster setup) |
| Framework | Ginkgo + envtest | kuttl |
