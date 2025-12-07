# Perses Operator

An operator to install [Perses](https://github.com/perses/perses) in a k8s cluster.

## Getting Started

Install the Perses Operator in your Kubernetes cluster. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.

### Prerequisites

You’ll need:

- a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) or [minikube](https://minikube.sigs.k8s.io/docs/) to get a local cluster for testing, or run against a remote cluster.
  **Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).
- [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) installed and configured to use your cluster.

### Running on the cluster

1. Install custom resource definitions:

```sh
make install
```

2. Deploy the operator:

**Option A: Using cert-manager**
```sh
make install-cert-manager
make deploy
```

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
make uninstall
```

### Undeploy controller

UnDeploy the controller from the cluster:

```sh
make undeploy
```

## Docs

- [API Docs](docs/api.md)
- [Developer Docs](docs/dev.md)
- Sample CRDs
  - [Kubernetes](config/samples)
  - [OpenShift](config/samples/openshift)
  - [Using TLS](config/samples/tls)

## License

Copyright 2025 The Perses Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
