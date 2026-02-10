# Testing

## Unit and Integration Tests

| Target | Description |
| ------ | ----------- |
| `make test-unit` | Run unit tests only (no envtest, fast) |
| `make test-integration` | Run controller integration tests with [envtest](https://pkg.go.dev/sigs.k8s.io/controller-runtime/tools/setup-envtest) |

Unit tests (`internal/`, `api/`, `scripts/`) use standard Go testing with [testify](https://github.com/stretchr/testify) or [Ginkgo](https://onsi.github.io/ginkgo/).

Integration tests (`controllers/`) use [Ginkgo](https://onsi.github.io/ginkgo/) with envtest to spin up a lightweight API server with CRDs installed.

## E2E Tests (kuttl)

E2E tests use [kuttl](https://kuttl.dev/) to validate the operator in a real [kind](https://kind.sigs.k8s.io/) cluster.

### Prerequisites

- [Go](https://go.dev/doc/install)
- [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- [Docker](https://docs.docker.com/get-docker/) or [Podman](https://podman.io/docs/installation)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)

### Makefile Targets

| Target | Description |
| ------ | ----------- |
| `make e2e-setup` | Create kind cluster and deploy operator |
| `make e2e-create-cluster` | Create kind cluster only |
| `make e2e-deploy` | Build image, load into kind, and deploy (requires existing cluster) |
| `make test-e2e` | Run kuttl tests (requires operator to be deployed) |
| `make e2e-cleanup` | Delete the kind cluster |

### Quick Start

```bash
make e2e-setup    # Create cluster + deploy operator
make test-e2e     # Run tests
make e2e-cleanup  # Tear down
```

When iterating, run `make e2e-setup` once, then repeat `make test-e2e`. Use `make e2e-deploy` to redeploy after operator code changes without recreating the cluster.

### Configuration

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
├── perses-statefulset/   # database.file → StatefulSet
│   ├── 00-install.yaml   # Create Perses instance, assert StatefulSet + sub-resources
│   ├── 00-assert.yaml
│   ├── 01-assert.yaml    # Assert pod readiness
│   ├── 02-install.yaml   # Create PersesDatasource, assert reconciled
│   ├── 02-assert.yaml
│   ├── 03-install.yaml   # Create PersesDashboard, assert reconciled
│   ├── 03-assert.yaml
│   ├── 04-delete.yaml    # Delete all, assert cleanup
│   └── 04-assert.yaml
└── perses-deployment/    # database.sql → Deployment
    ├── 00-install.yaml   # Create Perses instance with SQL config, assert Deployment + sub-resources
    ├── 00-assert.yaml
    ├── 01-delete.yaml    # Delete all, assert cleanup
    └── 01-assert.yaml
```

To add a new test case, create a directory under `test/e2e/`. To add steps to an existing test case, add numbered files (e.g. `05-install.yaml`, `05-assert.yaml`).

### Debugging

```bash
# Operator logs
kubectl logs -n perses-operator-system deployment/perses-operator-controller-manager -f

# Events in test namespace
kubectl get events -A --sort-by='.lastTimestamp' | tail -50

# All Perses resources
kubectl get perses,persesdashboard,persesdatasource -A

# Run a single test case
bin/kubectl-kuttl test --config test/e2e/kuttl-test.yaml --test perses-statefulset
```
