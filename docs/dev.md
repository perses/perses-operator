# Perses Operator

An operator to install [Perses](https://github.com/perses/perses) in a k8s cluster.

## Getting Started

You’ll need:

- a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) or [minikube](https://minikube.sigs.k8s.io/docs/) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).
- [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) installed and configured to use your cluster.

### Running on the cluster

1. Install custom resource definitions:
```sh
make install-crds
```

2. Create a namespace for the resources:
```sh
kubectl create namespace perses-dev
```

3. Install custom resources:

```sh
kubectl apply -k config/samples
```

4. Using the the location specified by `IMG`, build a testing image and push it to the registry, then deploy the controller to the cluster:
> **Note:** Make sure the image is accessible either publicly or from the cluster internal registry.
> 
> A cert is also required to run the operator due to the conversion webhook.

**Option A: Using self-signed certificates (for development/testing)**
```sh
IMG=<some-registry>/perses-operator:tag make test-image-build image-push deploy-local
```

**Option B: Using cert-manager (recommended for production)**
```sh
make install-cert-manager
IMG=<some-registry>/perses-operator:tag make test-image-build image-push deploy
```

> **Note:** If you already have an image built, you can deploy it to the cluster using `IMG=<some-registry>/perses-operator:tag make deploy`.

5. Port forward the service so you can access the Perses UI at `http://localhost:8080`:

```sh
kubectl -n perses-dev port-forward svc/perses-sample 8080:8080
```

### Uninstall CRDs

To delete the CRDs from the cluster:

```sh
make uninstall-crds
```

### Undeploy controller

UnDeploy the controller from the cluster:

```sh
make undeploy
```

## Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct, and the process for submitting pull requests to us.

### How it works

This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

Each instance of the CRD deploys the following resources:

- A ConfigMap holding the perses configuration
- A Service so perses API can be accessed from within the cluster
- A Deployment holding the perses API

### Test It Out

1. Install Instances of Custom Resources and run the controller:

```sh
PERSES_IMAGE=docker.io/persesdev/perses:v0.50.3 make install-crds run
```

2. Install a CRD instance

```sh
kubectl apply -f config/samples/v1alpha1_perses.yaml
```

3. Uninstall the CRD instance

```sh
kubectl delete -f config/samples/v1alpha1_perses.yaml
```

### Modifying the API definitions

If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests # Generate YAML manifests like CRDs, RBAC etc.
make generate # Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

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
