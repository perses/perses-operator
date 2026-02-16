GO				?= go
VERSION		?=$(shell cat VERSION)

# DATE defines the building date. It is used mainly for goreleaser when generating the GitHub release.
DATE := $(shell date +%Y-%m-%d)
export DATE

XARGS ?= $(shell which gxargs 2>/dev/null || which xargs)

# Include tool installation targets and versions from Makefile.tools
# See Makefile.tools for all tool versions and installation targets
include Makefile.tools

.PHONY: check-container-runtime
check-container-runtime:
	@if podman ps >/dev/null 2>&1; then \
		echo ""; \
		echo "============================================"; \
		echo " ✅ Using Podman the container runtime"; \
		echo "============================================"; \
	elif docker ps >/dev/null 2>&1; then \
		echo ""; \
		echo "============================================"; \
		echo " ✅ Using Docker the container runtime"; \
		echo "============================================"; \
	else \
		echo ""; \
		echo "============================================"; \
		echo " ❌ Neither Podman nor Docker daemon is running"; \
		echo "============================================"; \
		exit 1; \
	fi

# CONTAINER_RUNTIME defines the container runtime to use for building the bundle image.
CONTAINER_RUNTIME := $(shell if podman ps >/dev/null 2>&1; then echo podman; elif docker ps >/dev/null 2>&1; then echo docker; fi)

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# perses.dev/perses-operator-bundle:$VERSION and perses.dev/perses-operator-catalog:$VERSION.
IMAGE_TAG_BASE ?= docker.io/persesdev/perses-operator

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)

# USE_IMAGE_DIGESTS defines if images are resolved via tags or digests
# You can enable this value if you would like to use SHA Based Digests
# To enable set flag to true
USE_IMAGE_DIGESTS ?= false
ifeq ($(USE_IMAGE_DIGESTS), true)
	BUNDLE_GEN_FLAGS += --use-image-digests
endif

## Versions

# Image URL to use all building/pushing image targets
IMG ?= $(IMAGE_TAG_BASE):v$(VERSION)

# Docker image tag with git SHA and date
DOCKER_IMAGE_TAG ?= $(subst /,-,$(shell git rev-parse --abbrev-ref HEAD))-$(shell date +%Y-%m-%d)-$(shell git rev-parse --short HEAD)

.PHONY: print-image-tag
print-image-tag: ## Print the docker image tag with git SHA
	@echo $(DOCKER_IMAGE_TAG)

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)


.PHONY: checkformat
checkformat:
	@echo ">> checking go code format"
	! gofmt -d $$(find . -name '*.go' -print) | grep '^'

.PHONY: update-go-deps
update-go-deps: ## Update all Go dependencies to latest versions
	@echo ">> updating Go dependencies"
	@for m in $$(go list -mod=readonly -m -f '{{ if and (not .Indirect) (not .Main)}}{{.Path}}{{end}}' all); do \
		echo "Updating $$m"; \
		go get -u $$m; \
	done
	@go mod tidy -v
	@echo ">> Dependencies updated, run tests before committing."

.PHONY: tidy
tidy: ## Verify that go.mod and go.sum are tidy (for CI)
	@echo ">> running check for unused/missing packages in go.mod"
	@go mod tidy
	@git diff --exit-code -- go.sum go.mod

##@ Development

.PHONY: manifests
manifests: controller-gen gojsontoyaml ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects. Also generates json, for jsonnet lib.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd:ignoreUnexportedFields=true webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	find config/crd/bases -name '*.yaml' ! -name 'kustomization.yaml' -exec sh -c '\
	  for file do \
	    filename=$$(basename "$$file" .yaml); \
	    out_file="$$(pwd)/jsonnet/generated/$${filename}-crd.json"; \
	    $(GOJSONTOYAML_BINARY) -yamltojson < "$$file" | jq > "$$out_file"; \
	  done' sh {} +
	find config/rbac -name '*.yaml' ! -name 'kustomization.yaml' -exec sh -c '\
	  for file do \
	    filename=$$(basename "$$file" .yaml); \
	    out_file="$$(pwd)/jsonnet/generated/$${filename}.json"; \
	    $(GOJSONTOYAML_BINARY) -yamltojson < "$$file" | jq > "$$out_file"; \
	  done' sh {} +
	find config/manager -name '*.yaml' ! -name 'kustomization.yaml' ! -name 'namespace.yaml' -exec sh -c '\
	  for file do \
	    filename=$$(basename "$$file" .yaml); \
	    out_file="$$(pwd)/jsonnet/generated/$${filename}.json"; \
	    $(GOJSONTOYAML_BINARY) -yamltojson < "$$file" | jq > "$$out_file"; \
	  done' sh {} +
	find config/prometheus -name '*.yaml' ! -name 'kustomization.yaml' -exec sh -c '\
	  for file do \
	    filename=$$(basename "$$file" .yaml); \
	    out_file="$$(pwd)/jsonnet/generated/$${filename}.json"; \
	    $(GOJSONTOYAML_BINARY) -yamltojson < "$$file" | jq > "$$out_file"; \
	  done' sh {} +
	$(MAKE) jsonnet-resources

.PHONY: jsonnet-resources
jsonnet-resources: jsonnet gojsontoyaml
	@echo ">>>>> Running perses operator jsonnet"
	rm -f jsonnet/examples/*.yaml
	$(JSONNET_BINARY) -m jsonnet/examples jsonnet/example.jsonnet | $(XARGS) -I{} sh -c 'cat {} | $(GOJSONTOYAML_BINARY) > {}.yaml' -- {}
	find jsonnet/examples -type f -not -name "*.*" -delete

JSONNET_SRC = $(shell find . -type f -not -path './*vendor_jsonnet/*' \( -name '*.libsonnet' -o -name '*.jsonnet' \))

.PHONY: jsonnet-format
jsonnet-format: $(JSONNET_SRC) jsonnetfmt
	@echo ">>>>> Running format"
	$(JSONNETFMT_BINARY) -n 2 --max-blank-lines 2 --string-style s --comment-style s -i $(JSONNET_SRC)

.PHONY: generate
generate: controller-gen conversion-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."
	$(CONVERSION_GEN) \
		--output-file=zz_generated.conversion.go \
		--go-header-file=./hack/boilerplate.go.txt \
		--skip-unsafe=true \
		./api/v1alpha1

TYPES_V1ALPHA2_TARGET := api/v1alpha2/perses_types.go
TYPES_V1ALPHA2_TARGET += api/v1alpha2/persesdashboard_types.go
TYPES_V1ALPHA2_TARGET += api/v1alpha2/persesdatasource_types.go
TYPES_V1ALPHA2_TARGET += api/v1alpha2/persesglobaldatasource_types.go

# Extract Kubernetes API version from go.mod (e.g. v0.34.0 -> 1.34)
K8S_API_VERSION := $(shell grep 'k8s.io/api ' go.mod | awk '{print $$2}' | sed 's/v0\.\([0-9]*\)\..*/1.\1/')

.PHONY: generate-api-docs
generate-api-docs: crd-ref-docs $(TYPES_V1ALPHA2_TARGET) ## Generate API reference documentation from Go types.
	sed 's/K8S_API_VERSION/$(K8S_API_VERSION)/' docs/config/api-docs-config.yaml > docs/config/api-docs-config-resolved.yaml
	$(CRD_REF_DOCS) \
		--source-path=./api/v1alpha2 \
		--config=./docs/config/api-docs-config-resolved.yaml \
		--renderer=markdown \
		--output-path=./docs/api.md
	rm -f docs/config/api-docs-config-resolved.yaml

.PHONY: check-api-docs
check-api-docs: generate-api-docs ## Verify that generated API docs are up-to-date.
	@if ! git diff --quiet --exit-code docs/api.md; then \
		echo "docs/api.md is out of date. Please run 'make generate-api-docs' and commit the result."; \
		git diff docs/api.md --exit-code; \
	fi

.PHONY: generate-metrics-docs
generate-metrics-docs: ## Generate metrics documentation from metrics code.
	@echo "Generating metrics documentation..."
	@go run ./scripts/generate-metrics-docs/main.go docs/metrics.md

.PHONY: check-metrics-docs
check-metrics-docs: generate-metrics-docs ## Verify that generated metrics docs are up-to-date.
	@if ! git diff --quiet --exit-code docs/metrics.md; then \
		echo "docs/metrics.md is out of date. Please run 'make generate-metrics-docs' and commit the result."; \
		git diff docs/metrics.md --exit-code; \
	fi

.PHONY: fmt
fmt: jsonnet-format ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test-unit
test-unit: fmt vet ## Run unit tests.
	go test $(shell go list ./... | grep -v ./controllers) -v -coverprofile cover-unit.out

.PHONY: test-integration
test-integration: manifests generate fmt vet envtest ## Run integration tests using envtest.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./controllers/... -v -coverprofile cover-integration.out

.PHONY: lint-jsonnet
lint-jsonnet: $(JSONNETLINT_BINARY)
	@echo ">>>>> Running linter"
	echo ${JSONNET_SRC} | $(XARGS) -n 1 -- $(JSONNETLINT_BINARY)

.PHONY: lint
lint: lint-jsonnet ## Run linting.
	golangci-lint run

##@ E2E Testing

KIND_CLUSTER_NAME ?= kuttl-e2e
KIND_VERSION ?= $(shell grep kind-version .github/env | sed 's/kind-version=//')
KIND_NODE_IMAGE ?= $(shell grep kind-image .github/env | sed 's/kind-image=//')
E2E_TAG ?= $(shell git rev-parse --short HEAD)
E2E_IMG ?= $(IMAGE_TAG_BASE):$(E2E_TAG)

.PHONY: e2e-versions
e2e-versions: ## Display versions used for e2e testing.
	@echo "Kind version: $(KIND_VERSION)"
	@echo "Kind node image: $(KIND_NODE_IMAGE)"
	@echo "Go version: $(shell grep golang-version .github/env | sed 's/golang-version=//')"

.PHONY: e2e-create-cluster
e2e-create-cluster: ## Create a kind cluster for e2e tests.
	@echo ">> Creating kind cluster with image $(KIND_NODE_IMAGE)..."
	kind create cluster --name $(KIND_CLUSTER_NAME) --image $(KIND_NODE_IMAGE) --wait 5m 2>/dev/null || true

.PHONY: e2e-deploy
e2e-deploy: manifests generate kustomize check-container-runtime ## Build operator image, load into kind and deploy.
	kubectl config use-context kind-$(KIND_CLUSTER_NAME)
	@echo ">> Building operator image..."
	$(CONTAINER_RUNTIME) build -f Dockerfile.dev -t $(E2E_IMG) .
	@echo ">> Loading image into kind cluster..."
ifeq ($(CONTAINER_RUNTIME),podman)
	$(CONTAINER_RUNTIME) save -o /tmp/perses-operator-e2e.tar $(E2E_IMG)
	kind load image-archive /tmp/perses-operator-e2e.tar --name $(KIND_CLUSTER_NAME)
	rm -f /tmp/perses-operator-e2e.tar
else
	kind load docker-image $(E2E_IMG) --name $(KIND_CLUSTER_NAME)
endif
	@echo ">> Installing cert-manager..."
	$(MAKE) install-cert-manager
	@echo ">> Installing CRDs and deploying operator..."
	$(KUSTOMIZE) build config/default | kubectl apply -f -
	kubectl set image -n perses-operator-system deployment/perses-operator-controller-manager manager=$(E2E_IMG)
	kubectl patch deployment perses-operator-controller-manager -n perses-operator-system \
		-p '{"spec":{"template":{"spec":{"containers":[{"name":"manager","imagePullPolicy":"IfNotPresent"}]}}}}'
	kubectl wait --for=condition=Available --timeout=300s -n perses-operator-system deployment/perses-operator-controller-manager

.PHONY: e2e-setup
e2e-setup: e2e-create-cluster e2e-deploy ## Create kind cluster and deploy operator (local development).

.PHONY: test-e2e
test-e2e: kuttl ## Run e2e tests using kuttl (run e2e-setup first if cluster is not ready).
	$(KUTTL) test --config test/e2e/kuttl-test.yaml

.PHONY: e2e-cleanup
e2e-cleanup: ## Delete the kind cluster used for e2e tests.
	kind delete cluster --name $(KIND_CLUSTER_NAME)

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager main.go

# Generate local webhook certs and patch CRDs with CA Bundle
.PHONY: generate-local-certs
generate-local-certs: yq
	./scripts/gen-certs.sh

.PHONY: run
run: generate-local-certs manifests generate fmt vet ## Run a controller from your host.
	go run ./main.go $(ARGS)

# If you wish built the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64 ). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: image-build
image-build: check-container-runtime build test-unit test-integration ## Build docker image with the manager.
	${CONTAINER_RUNTIME} build -f Dockerfile -t ${IMG} .

.PHONY: test-image-build
test-image-build: check-container-runtime test-unit test-integration ## Build a testing docker image with the manager.
	${CONTAINER_RUNTIME} build -f Dockerfile.dev -t ${IMG} .

.PHONY: image-push
image-push: ## Push docker image with the manager.
	${CONTAINER_RUNTIME} push ${IMG}

# PLATFORMS defines the target platforms for  the manager image be build to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - able to use docker buildx . More info: https://docs.docker.com/build/buildx/
# - have enable BuildKit, More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image for your registry (i.e. if you do not inform a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To properly provided solutions that supports more than one platform you should use this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: test-unit test-integration ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- docker buildx create --name project-v3-builder
	docker buildx use project-v3-builder
	- docker buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- docker buildx rm project-v3-builder
	rm Dockerfile.cross

.PHONY: podman-cross-build
podman-cross-build: test-unit test-integration
	podman manifest create -a ${IMG}
	podman build --platform $(PLATFORMS) --manifest ${IMG} -f Dockerfile.dev
	podman manifest push ${IMG}

# Add a bundle.yaml file with CRDs and deployment, with kustomize config.
.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	@echo ">> generating bundle.yaml (override image using IMG)"
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default > bundle.yaml

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install-cert-manager
install-cert-manager: ## Install cert-manager into the K8s cluster specified in ~/.kube/config.
	kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.yaml
	kubectl wait --for=condition=Available --timeout=300s -n cert-manager deployment/cert-manager
	kubectl wait --for=condition=Available --timeout=300s -n cert-manager deployment/cert-manager-cainjector
	kubectl wait --for=condition=Available --timeout=300s -n cert-manager deployment/cert-manager-webhook

.PHONY: install-crds
install-crds: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall-cert-manager
uninstall-cert-manager: ## Uninstall cert-manager from the K8s cluster specified in ~/.kube/config.
	kubectl delete -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.2/cert-manager.yaml --ignore-not-found=$(ignore-not-found)

.PHONY: uninstall-crds
uninstall-crds: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: deploy-with-certmanager ## Deploy controller to the K8s cluster specified in ~/.kube/config.

.PHONY: deploy-with-certmanager
deploy-with-certmanager: manifests kustomize install-cert-manager
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

## Deploy controller to the K8s cluster specified in ~/.kube/config.
## This target generates the webhook certs locally and applies them to the cluster
.PHONY: deploy-local
deploy-local: generate-local-certs manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	kubectl -n perses-operator-system create secret tls webhook-server-cert \
    	--dry-run=client -o yaml \
		--cert="/tmp/k8s-webhook-server/serving-certs/tls.crt" \
		--key="/tmp/k8s-webhook-server/serving-certs/tls.key" > "config/local/certificate.yaml"
	$(KUSTOMIZE) build config/local | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ Build Dependencies

.PHONY: bundle
bundle: manifests kustomize operator-sdk ## Generate bundle manifests and metadata, then validate generated files.
	$(OPERATOR_SDK) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: bundle-check
bundle-check: bundle
	git diff --exit-code bundle config jsonnet/generated jsonnet/examples

.PHONY: bundle-build
bundle-build: generate bundle ## Build the bundle image.
	$(CONTAINER_RUNTIME) build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) image-push IMG=$(BUNDLE_IMG)

# A comma-separated list of bundle images (e.g. make catalog-build BUNDLE_IMGS=example.com/operator-bundle:v0.1.0,example.com/operator-bundle:v0.2.0).
# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS ?= $(BUNDLE_IMG)

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: check-container-runtime opm ## Build a catalog image.
	$(OPM) index add --container-tool $(CONTAINER_RUNTIME) --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) image-push IMG=$(CATALOG_IMG)

.PHONY: generate-goreleaser
generate-goreleaser:
	go run ./scripts/generate-goreleaser/generate-goreleaser.go

.PHONY: push-main-docker-image
push-main-docker-image:
	go run ./scripts/push-main-docker-image/push-main-docker-image.go

## Cross build binaries for all platforms (Use "make image-build" in development)
.PHONY: cross-build
cross-build: generate-goreleaser manifests generate fmt vet ## Cross build binaries for all platforms (Use "make build" in development)
	goreleaser release --snapshot --clean

.PHONY: cross-release
cross-release: generate-goreleaser manifests generate fmt vet
	goreleaser release --clean

.PHONY: bin
bin: 
	go build $(BUILD_OPTS) -mod=readonly -o bin/manager main.go

.PHONY: generate-changelog
generate-changelog:
	$(GO) run ./scripts/generate-changelog/generate-changelog.go --version="${VERSION}"

.PHONY: tag
tag:
	./scripts/release.sh --tag "${VERSION}"
