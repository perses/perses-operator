# OLM Installation Guide

## Pre-requisites

* [operator-sdk](https://sdk.operatorframework.io/docs/installation/) CLI (install via `make operator-sdk`)
* A Kubernetes cluster like [kind](https://sigs.k8s.io/kind), [minikube](https://minikube.sigs.k8s.io/docs/), or [OpenShift](https://www.redhat.com/en/technologies/cloud-computing/openshift) (OLM is pre-installed on OpenShift)

## Quick Start (Testing)

For quick testing, use [`operator-sdk run bundle`](https://sdk.operatorframework.io/docs/cli/operator-sdk_run_bundle/) which handles CatalogSource and Subscription creation automatically:

1. Install OLM (skip on OpenShift)

    ```bash
    bin/operator-sdk olm install
    ```

2. Build and push the operator and bundle images

    ```bash
    IMAGE_TAG_BASE=<some-registry>/perses-operator
    make test-image-build image-push bundle-build bundle-push
    ```

3. Run the bundle

    ```bash
    bin/operator-sdk run bundle <some-registry>/perses-operator-bundle:v0.2.0
    ```

4. Create a Perses CR

    ```bash
    kubectl create namespace perses-dev
    kubectl apply -f config/samples/v1alpha2/perses.yaml
    ```

To clean up: `bin/operator-sdk cleanup perses-operator`

## Manual Steps

For more control (e.g., testing catalog images or subscriptions), follow these steps:

1. Install OLM (skip on OpenShift)

    ```bash
    bin/operator-sdk olm install
    ```

2. Build and push all required images

    ```bash
    IMAGE_TAG_BASE=<some-registry>/perses-operator
    make test-image-build image-push bundle-build bundle-push catalog-build catalog-push
    ```

3. Update the image in `hack/resources/catalog-source.yaml` to match the image pushed above, then create a CatalogSource

    ```bash
    kubectl apply -f hack/resources/catalog-source.yaml
    ```

4. Create a Subscription

    ```bash
    kubectl apply -f hack/resources/operator-subscription.yaml
    ```

5. Check that the operator is installed

    ```bash
    kubectl get clusterserviceversion -n operators -w
    ```

6. Create a Perses CR

    ```bash
    kubectl create namespace perses-dev
    kubectl apply -f config/samples/v1alpha2/perses.yaml
    ```
