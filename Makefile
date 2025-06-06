GO				?= go
VERSION		?=$(shell cat VERSION)

# DATE defines the building date. It is used mainly for goreleaser when generating the GitHub release.
DATE := $(shell date +%Y-%m-%d)
export DATE

XARGS ?= $(shell which gxargs 2>/dev/null || which xargs)


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

# Set the Operator SDK version to use. By default, what is installed on the system is used.
# This is useful for CI or a project to utilize a specific version of the operator-sdk toolkit.
OPERATOR_SDK_VERSION ?= v1.32.0
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.31.0
# ENVTEST_VERSION refers to the version of the envtest binary.
ENVTEST_VERSION ?= release-0.19

# Image URL to use all building/pushing image targets
IMG ?= $(IMAGE_TAG_BASE):v$(VERSION)

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

.PHONY: checkunused
checkunused:
	@echo ">> running check for unused/missing packages in go.mod"
	go mod tidy
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
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: jsonnet-format ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)"  go test $(shell go list ./...) -v -coverprofile cover.out

.PHONY: lint-jsonnet
lint-jsonnet: $(JSONNETLINT_BINARY)
	@echo ">>>>> Running linter"
	echo ${JSONNET_SRC} | $(XARGS) -n 1 -- $(JSONNETLINT_BINARY)

.PHONY: lint
lint: lint-jsonnet ## Run linting.
	golangci-lint run

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./main.go $(ARGS)

# If you wish built the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64 ). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: image-build
image-build: check-container-runtime build test ## Build docker image with the manager.
	${CONTAINER_RUNTIME} build -f Dockerfile -t ${IMG} .

.PHONY: test-image-build
test-image-build: check-container-runtime test ## Build a testing docker image with the manager.
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
docker-buildx: test ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- docker buildx create --name project-v3-builder
	docker buildx use project-v3-builder
	- docker buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- docker buildx rm project-v3-builder
	rm Dockerfile.cross

.PHONY: podman-cross-build
podman-cross-build: test
	podman manifest create ${IMG}
	podman build --platform $(PLATFORMS) --manifest ${IMG} -f Dockerfile.dev
	podman manifest push ${IMG}

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
GOJSONTOYAML_BINARY ?= $(LOCALBIN)/gojsontoyaml
ENVTEST ?= $(LOCALBIN)/setup-envtest-$(ENVTEST_VERSION)
JSONNET_BINARY ?= $(LOCALBIN)/jsonnet
JSONNETFMT_BINARY ?= $(LOCALBIN)/jsonnetfmt
JSONNETLINT_BINARY ?= $(LOCALBIN)/jsonnet-lint

## Tool Versions
KUSTOMIZE_VERSION ?= v3.8.7
CONTROLLER_TOOLS_VERSION ?= v0.16.0
GOJSONTOYAML_VERSION ?= v0.1.0
JSONNET_VERSION ?= v0.21.0
JSONNETFMT_VERSION ?= v0.21.0
JSONNETLINT_VERSION ?= v0.21.0
KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kustomize && ! $(LOCALBIN)/kustomize version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(LOCALBIN)/kustomize version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/kustomize; \
	fi
	test -s $(LOCALBIN)/kustomize || { curl -Ss $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); }

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: gojsontoyaml
gojsontoyaml: $(GOJSONTOYAML_BINARY) ## Download gojsontoyaml locally if necessary. If wrong version is installed, it will be overwritten.
$(GOJSONTOYAML_BINARY): $(LOCALBIN)
	test -s $(LOCALBIN)/gojsontoyaml && $(LOCALBIN)/gojsontoyaml --version | grep -q $(GOJSONTOYAML_VERSION) || \
	GOBIN=$(LOCALBIN) go install github.com/brancz/gojsontoyaml@$(GOJSONTOYAML_VERSION)

.PHONY: jsonnet
jsonnet: $(JSONNET_BINARY) ## Download jsonnet locally if necessary. If wrong version is installed, it will be overwritten.
$(JSONNET_BINARY): $(LOCALBIN)
	test -s $(LOCALBIN)/jsonnet && $(LOCALBIN)/jsonnet --version | grep -q $(JSONNET_VERSION) || \
	GOBIN=$(LOCALBIN) go install github.com/google/go-jsonnet/cmd/jsonnet@$(JSONNET_VERSION)

.PHONY: jsonnetfmt
jsonnetfmt: $(JSONNETFMT_BINARY) ## Download jsonnetfmt locally if necessary. If wrong version is installed, it will be overwritten.
$(JSONNETFMT_BINARY): $(LOCALBIN)
	test -s $(LOCALBIN)/jsonnetfmt && $(LOCALBIN)/jsonnetfmt --version | grep -q $(JSONNETFMT_VERSION) || \
	GOBIN=$(LOCALBIN) go install github.com/google/go-jsonnet/cmd/jsonnetfmt@$(JSONNETFMT_VERSION)

.PHONY: jsonnet-lint
jsonnet-lint: $(JSONNETLINT_BINARY) ## Download jsonnetlint locally if necessary. If wrong version is installed, it will be overwritten.
$(JSONNETLINT_BINARY): $(LOCALBIN)
	test -s $(LOCALBIN)/jsonnet-lint && $(LOCALBIN)/jsonnet-lint --version | grep -q $(JSONNETLINT_VERSION) || \
	GOBIN=$(LOCALBIN) go install github.com/google/go-jsonnet/cmd/jsonnet-lint@$(JSONNETLINT_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: operator-sdk
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk
operator-sdk: ## Download operator-sdk locally if necessary.
ifeq (,$(wildcard $(OPERATOR_SDK)))
ifeq (, $(shell which operator-sdk 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPERATOR_SDK)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPERATOR_SDK) https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$${OS}_$${ARCH} ;\
	chmod +x $(OPERATOR_SDK) ;\
	}
else
OPERATOR_SDK = $(shell which operator-sdk)
endif
endif

.PHONY: bundle
bundle: manifests kustomize operator-sdk ## Generate bundle manifests and metadata, then validate generated files.
	$(OPERATOR_SDK) generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: bundle-check
bundle-check: bundle
	git diff --quiet --exit-code bundle config

.PHONY: bundle-build
bundle-build: generate bundle ## Build the bundle image.
	$(CONTAINER_RUNTIME) build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) image-push IMG=$(BUNDLE_IMG)

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.23.0/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

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
	
# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv "$$(echo "$(1)" | sed "s/-$(3)$$//")" $(1) ;\
}
endef

.PHONY: generate-goreleaser
generate-goreleaser:
	go run ./scripts/generate-goreleaser/generate-goreleaser.go

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
