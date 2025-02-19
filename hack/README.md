# Hack

## Install the operator using olm

1. Connect to a cluster like [kind](https://sigs.k8s.io/kind) or [minikube](https://minikube.sigs.k8s.io/docs/)
2. Install the Operator Lifecycle Manager (OLM)
```bash
operator-sdk olm install
```
1. Build the required images and push them to a registry
```bash
IMG=<some-registry>/perses-operator:tag make test-image-build image-push
IMG=<some-registry>/perses-operator:tag make bundle-build bundle-push
IMG=<some-registry>/perses-operator:tag make catalog-build catalog-push
```
2. Change the image inside `hack/resources/catalog-source.yaml` to the image you pushed in the previous step and create a CatalogSource
```bash
kubectl apply -f hack/resources/catalog-source.yaml
```
3. Create a Subscription
```bash
kubectl apply -f hack/resources/operator-subscription.yaml
```
4. Check that the operator is installed
```bash
kubectl get clusterserviceversion -n operators -w
```
5. Now you can create a Perses CR and the operator will create the required resources
```bash
kubectl create namespace perses-dev
kubectl apply -f config/samples/perses_v1alpha1_perses.yaml
```