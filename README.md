# Perses Operator

[![ci](https://github.com/perses/perses-operator/actions/workflows/ci.yaml/badge.svg)](https://github.com/perses/perses-operator/actions/workflows/ci.yaml)
[![go](https://github.com/perses/perses-operator/actions/workflows/go.yaml/badge.svg)](https://github.com/perses/perses-operator/actions/workflows/go.yaml)
[![e2e](https://github.com/perses/perses-operator/actions/workflows/e2e.yaml/badge.svg)](https://github.com/perses/perses-operator/actions/workflows/e2e.yaml)
[![docs](https://github.com/perses/perses-operator/actions/workflows/docs.yaml/badge.svg)](https://github.com/perses/perses-operator/actions/workflows/docs.yaml)
[![join slack](https://img.shields.io/badge/join%20slack-%23perses--dev-brightgreen.svg)](https://cloud-native.slack.com/messages/C07KQR95WBE)

## Overview

The Perses Operator provides [Kubernetes](https://kubernetes.io/) native deployment and management of
[Perses](https://github.com/perses/perses) and related resources. The purpose of this project is to
simplify and automate the deployment and management of Perses observability dashboards on Kubernetes clusters.

The Perses Operator includes, but is not limited to, the following features:

* **Kubernetes Custom Resources**: Use Kubernetes custom resources to deploy and manage Perses instances,
  dashboards, and datasources.

* **Dashboard-as-Code**: Declaratively manage dashboards and datasources as Kubernetes resources,
  with automatic synchronization to Perses instances.

* **Flexible Storage**: Configure SQL database (Deployment) or file-based storage with PVC (StatefulSet)
  or emptyDir from a native Kubernetes resource.

* **TLS and Authentication**: Configure server TLS, client mTLS, and datasource proxy TLS.
  Support for BasicAuth, OAuth, and native Kubernetes authentication.

* **Multi-Instance Sync**: Use `instanceSelector` on dashboards and datasources to target specific
  Perses instances, with namespace-to-project mapping for isolation.

* **Observability**: Built-in Prometheus metrics and alerting rules compatible with
  [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator).

For an introduction to the Perses Operator, see the [Getting Started](#getting-started) guide. For detailed usage documentation, see the [User Guide](https://perses.dev/perses-operator/docs/user-guide/) and the [API Reference](https://perses.dev/perses-operator/docs/api/).

## Project Status

The operator is under active development. Please refer to the Custom Resource Definition (CRD) version for the current API status:

* `perses.dev/v1alpha2`: **unstable** CRDs and API, changes can happen frequently. We encourage usage
  for testing and development, but suggest caution in mission-critical environments.

## Custom Resource Definitions (CRDs)

A core feature of the Perses Operator is to watch the Kubernetes API server for changes
to specific objects and ensure that the desired Perses deployments and configurations match.
The Operator acts on the following [Custom Resource Definitions (CRDs)](https://kubernetes.io/docs/tasks/access-kubernetes-api/extend-api-custom-resource-definitions/):

* **`Perses`**, which defines a desired Perses server deployment. The operator manages the underlying
  Deployment or StatefulSet, Service, and ConfigMap based on the spec.

* **`PersesDashboard`**, which declaratively specifies a dashboard to be synced to Perses instances.
  Kubernetes namespaces map to Perses projects.

* **`PersesDatasource`**, which declaratively specifies a project-scoped datasource to be synced
  to matching Perses instances. The datasource's namespace maps to a Perses project.

* **`PersesGlobalDatasource`**, which declaratively specifies a cluster-scoped datasource shared
  across all Perses projects.

The Perses Operator automatically detects changes in the Kubernetes API server to any of the above
objects, and ensures that matching deployments and configurations are kept in sync.

## Getting Started

### Prerequisites

You’ll need:

- a Kubernetes cluster to run against. You can use [kind](https://sigs.k8s.io/kind) or [minikube](https://minikube.sigs.k8s.io/docs/) to get a local cluster for testing, or run against a remote cluster.
  **Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).
- [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) installed and configured to use your cluster.

### Running on the cluster

1. Install custom resource definitions:

```sh
make install-crds
```

2. Deploy the operator:

**Option A: Using cert-manager**

```sh
make install-cert-manager
make deploy
```

> [!IMPORTANT]
> This will deploy the controller with the default image 'docker.io/perses/perses-operator:latest'.
> To use a different image, set the `IMG` variable:
>
> ```sh
> make deploy IMG=<your-image>
> ```

**Option B: Using self-signed certificates (for development/testing)**

```sh
make deploy-local
```

3. Create a namespace for the resources:

```sh
kubectl create namespace perses-dev
```

4. Install custom resources:

```sh
kubectl apply -k config/samples
```

5. Check the Perses UI:

```sh
kubectl -n perses-dev port-forward svc/perses-sample 8080:8080
```

### Uninstall CRDs

To delete the CRDs from the cluster:

```sh
make uninstall-crds
```

### Undeploy controller

Undeploy the controller from the cluster:

```sh
make undeploy
```

## Docs

- [API Docs](docs/api.md)
- [Metrics Documentation](docs/metrics.md)
- [Developer Docs](docs/dev.md)
- [Testing](docs/testing.md)
- Examples
  - [Kubernetes](config/samples)
  - [OpenShift](config/samples/openshift)
  - [Using TLS](config/samples/tls)

## Maintainers

See [MAINTAINERS](MAINTAINERS.md)

## Contributing

See [CONTRIBUTING](CONTRIBUTING.md)

## License

Copyright The Perses Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
