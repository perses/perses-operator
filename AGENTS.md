# AGENTS.md

Guidance for AI coding agents. Human-oriented docs: [README.md](README.md), [docs/dev.md](docs/dev.md),
[docs/testing.md](docs/testing.md), [CONTRIBUTING.md](CONTRIBUTING.md).

## Project Overview

The **Perses Operator** is a Kubernetes operator (Go, Kubebuilder v4, controller-runtime) that deploys
[Perses](https://github.com/perses/perses) servers and syncs dashboards and datasources to them via the
Perses API.

- Module: `github.com/perses/perses-operator`. Go version: `go.mod`. Operator version: `VERSION`.
- API group `perses.dev`. Target **`v1alpha2`** (storage version). `Perses`, `PersesDashboard`, and
  `PersesDatasource` also serve legacy `v1alpha1` through conversion webhooks. `PersesGlobalDatasource`
  is v1alpha2-only.

| Kind                     | Scope      | Short name | Operator behavior                                                          |
|--------------------------|------------|------------|----------------------------------------------------------------------------|
| `Perses`                 | Namespaced | `per`      | ConfigMap, Service, Deployment (SQL storage) or StatefulSet (file storage) |
| `PersesDashboard`        | Namespaced | `perdb`    | Synced to matching instances (`instanceSelector`); namespace = project     |
| `PersesDatasource`       | Namespaced | `perds`    | Project-scoped datasource synced to matching instances                     |
| `PersesGlobalDatasource` | Cluster    | `pergds`   | Global datasource shared across projects                                   |

Dashboards and datasources re-sync every `--resource-sync-interval` (default 5m), so the operator
recreates anything deleted in the Perses UI.

## Repository Layout

```text
main.go                 # Flags, manager setup, controller + webhook registration
api/v1alpha2/           # Current/storage API types
api/v1alpha1/           # Legacy types, conversions (*_conversion.go), webhooks (*_webhook.go)
controllers/            # One package per CR: perses/, dashboards/, datasources/, globaldatasources/
                        #   *_test.go here are Ginkgo + envtest integration tests
internal/subreconciler/ # Ordered reconcile steps (RunAll, ContinueReconciling, Requeue...)
internal/perses/common/ # Shared helpers: Perses API client factory, TLS/auth, labels, volumes, status, sync
internal/metrics/       # Operator Prometheus metrics
internal/operator/      # Defaults (DefaultPersesImage, DefaultResourceSyncInterval)
internal/{cache,tls,openshift}/ # Secret cache filtering, operator TLS, OpenShift TLS profile
config/                 # Kustomize: crd, rbac, manager, webhook, samples, e2e, local, components/ha
jsonnet/                # Jsonnet library + mixin (alerts in jsonnet/mixin/alerts/)
test/e2e/               # kuttl e2e cases, one dir per case, numbered steps (00-install.yaml, 00-assert.yaml)
test/promtool/          # Alerting rule tests
scripts/                # Helper scripts and Go tools (changelog, license check, metrics docs)
```

Controllers run ordered sub-reconciler steps. Most use `subreconciler.RunAll` (see
`controllers/perses/perses_controller.go`). Add behavior as a focused step. When you add error paths,
keep the status, metrics, and `ReconciliationTracker` updates.

## Commands

Run `make help` for the full list. Tools install into `./bin` on demand (`make build-tools` installs all).

```bash
# Build & run
go build ./...             # Fast compile check
make build                 # Regenerate code/manifests, fmt, vet, build bin/manager
make install-crds          # Install CRDs into the current kubeconfig cluster
make run                   # Run the operator locally (generates local webhook certs)

# Tests
make test-unit             # All packages except ./controllers
make test-integration      # Ginkgo + envtest for ./controllers/...
make test-alerts           # promtool tests for alerting rules
go test ./internal/metrics/... -run TestNewMetrics -v
bin/ginkgo --focus "GetVolumes" -v ./internal/perses/common/...
KUBEBUILDER_ASSETS="$(bin/setup-envtest use -p path)" bin/ginkgo --focus "<It text>" -v ./controllers/...

# E2E (kind + kuttl). Requires the kind version pinned in .github/env. See docs/testing.md
make e2e-setup && make test-e2e && make e2e-cleanup

# Format, lint, hygiene
make format                # gofmt + shfmt (writes); CI runs make checkformat
make lint                  # golangci-lint + jsonnet lint + shellcheck
make lint-api              # kube-api-linter on API types
make fmt-docs              # mdox markdown format + link validation
make tidy                  # go mod tidy, fails on diff
make checklicense          # Apache headers (make fixlicense to add)
```

CI ([go.yaml](.github/workflows/go.yaml), [docs.yaml](.github/workflows/docs.yaml),
[e2e.yaml](.github/workflows/e2e.yaml)) runs these checks. It also fails if regenerating files produces
a diff. Before you finish, run the checks that apply to your change.

## Making Changes

**Never hand-edit generated files.** Edit the source, then regenerate:
`zz_generated.*.go`, `config/crd/bases/`, `config/rbac/role.yaml`, `bundle/`, `bundle.yaml`,
`jsonnet/generated/`, `jsonnet/examples*/`, `docs/api.md`, `docs/metrics.md`.

| Change                               | Required action                                                                                                                                                         |
|--------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| API types / `+kubebuilder` markers   | Edit `api/v1alpha2`. Update `api/v1alpha1/*_conversion.go` + `conversion_roundtrip_test.go` if applicable. Run `make generate bundle build-installer generate-api-docs` |
| RBAC (`//+kubebuilder:rbac` markers) | `make manifests`                                                                                                                                                        |
| Metrics (`internal/metrics`)         | `make generate-metrics-docs`                                                                                                                                            |
| Alerts (`jsonnet/mixin/alerts/`)     | Update `test/promtool/alerts_test.yaml`, then `make test-alerts` (regenerates jsonnet examples)                                                                         |

Other gotchas:

- Every Go file needs the Apache header from `hack/boilerplate.go.txt`.
- By default the operator only watches Secrets labeled `perses.dev/watch=true`. Tests and samples that
  reference Secrets need this label.
- Webhooks are enabled unless `ENABLE_WEBHOOKS=false`. For certificate setup, see
  [docs/dev.md](docs/dev.md#running-locally).
- The Makefile auto-detects the container runtime (Podman first, then Docker).

## Commits & PRs

- Titles must match `[FEATURE|ENHANCEMENT|BUGFIX|BREAKINGCHANGE|DOC|IGNORE] <message>` (enforced by
  commitlint). Use `IGNORE` for changes that should not appear in the changelog.
- Sign off every commit: `git commit -s`. PRs are squash-merged, one fix or feature per PR.
  See [CONTRIBUTING.md](CONTRIBUTING.md) and [RELEASE.md](RELEASE.md).
