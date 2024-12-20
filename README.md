# Perses Operator

An operator to install [Perses](https://github.com/perses/perses) in a k8s cluster.

## Getting Started

You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster

1. Install custom resource definitions:
```sh
make install
```

2. Install custom resources:

```sh
kubectl apply -k config/samples
```

3. Usint the the location specified by `IMG`, build and push the image to the registry, then deploy the controller to the cluster:

```sh
IMG=<some-registry>/perses-operator:tag make image-build image-push deploy
```

5. Access the Perses UI at `http://localhost:8080` by port-forwarding the service:

```sh
kubectl port-forward svc/perses-sample 8080:8080
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

## Contributing

// TODO

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
PERSES_IMAGE=docker.io/persesdev/perses:latest make install run
```

2. Install a CRD instance

```sh
kubectl apply -f config/samples/v1alpha1_perses.yaml --namespace default
```

3. Uninstall the CRD instance

```sh
kubectl delete -f config/samples/v1alpha1_perses.yaml --namespace default
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

Copyright 2023 The Perses Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
