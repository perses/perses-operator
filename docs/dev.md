# Developer Guide

This guide covers the development workflow for the Perses Operator. For general usage and deployment, see the [README](../README.md).

## Prerequisites

* A Kubernetes cluster — use [kind](https://sigs.k8s.io/kind) or [minikube](https://minikube.sigs.k8s.io/docs/) for local development
* [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) configured for your cluster
* [Go](https://go.dev/doc/install)

## How It Works

This project follows the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/) using [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/) that reconcile resources until the desired state is reached.

Each `Perses` CR instance manages the following resources:

* A ConfigMap holding the Perses server configuration
* A Deployment (SQL storage) or StatefulSet (file-based storage) running the Perses server
* A Service for in-cluster API access

Dashboards and datasources (`PersesDashboard`, `PersesDatasource`, `PersesGlobalDatasource`) are synced to matching Perses instances via the Perses API.

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html).

## Running Locally

Run the controller on your host machine (outside the cluster). This is the fastest iteration loop for development.

1. Install CRDs into the cluster:

```shell
make install-crds
```

2. Run the controller:

```shell
make run
```

> [!NOTE]
> The Perses server image used by the operator is resolved in this order (highest priority first):
>
> 1. `spec.image` in the Perses CR
> 2. `--perses-default-base-image` flag passed to the operator binary
> 3. `DefaultPersesImage` constant in `internal/operator/defaults.go` (compile-time default)
>
> To use a different image when running locally:
>
> ```shell
> make run ARGS="--perses-default-base-image=docker.io/persesdev/perses:latest"
> ```

3. In another terminal, create a namespace and apply sample resources:

```shell
kubectl create namespace perses-dev
kubectl apply -k config/samples
```

4. Access the Perses UI:

```shell
kubectl -n perses-dev port-forward svc/perses-sample 8080:8080
```

## Deploying to a Cluster

To test with the operator running inside the cluster, build and deploy a development image. The operator requires TLS certificates for the conversion webhook — choose one of the options below. Make sure the image is pushed to a registry accessible from the cluster.

For local kind clusters, use the e2e targets which build the image and load it directly into kind without pushing to a registry:

```shell
make e2e-deploy  # Build image, load into kind, and deploy
```

For remote clusters, build, push to a registry, and deploy:

> [!IMPORTANT]
> Make sure your kubectl context is pointing to the target cluster before deploying.

**Using self-signed certificates (for development/testing):**

```shell
IMG=<some-registry>/perses-operator:tag make test-image-build image-push deploy-local
```

**Using [cert-manager](https://cert-manager.io/) (recommended for production-like environments):**

```shell
make install-cert-manager
IMG=<some-registry>/perses-operator:tag make test-image-build image-push deploy
```

> [!NOTE]
> If you already have an image built and pushed, you can skip the build step: `IMG=<some-registry>/perses-operator:tag make deploy`.

### Undeploy

```shell
make undeploy
```

### Using OLM

To install the operator via [Operator Lifecycle Manager (OLM)](https://olm.operatorframework.io/), see [hack/README.md](../hack/README.md).

## Modifying the API Definitions

After editing the API types under `api/`, regenerate manifests and code:

```shell
make generate          # Regenerate DeepCopy methods
make bundle            # Regenerate CRDs, RBAC, webhooks, and OLM bundle manifests
make generate-api-docs # Regenerate API reference documentation
```

CI will fail if the generated manifests or API docs are out of date.

## Modifying or Adding Metrics

Metrics are defined in `internal/metrics/`. After adding or modifying metrics, regenerate the documentation:

```shell
make generate-metrics-docs
```

This updates [docs/metrics.md](metrics.md) automatically from the metrics code. CI will fail if the generated docs are out of date.

## Modifying or Adding Alerting Rules

Alerting rules are defined in [`jsonnet/mixin/alerts/alerts.libsonnet`](../jsonnet/mixin/alerts/alerts.libsonnet). After making changes, regenerate the YAML resources and run the alert tests:

```shell
make jsonnet-resources  # Regenerate YAML files in jsonnet/examples/
make test-alerts        # Validate alerting rules with promtool
```

See [Testing](testing.md#alerting-rule-tests) for details.

## Useful Make Targets

Run `make help` to see all available targets with descriptions.

## Contributing

Please read [CONTRIBUTING.md](../CONTRIBUTING.md) for details on the contribution process and commit message conventions.
