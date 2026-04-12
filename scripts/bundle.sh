#!/usr/bin/env bash

set -eu -o pipefail

PROJECT_ROOT="$(git rev-parse --show-toplevel)"
declare -r PROJECT_ROOT
declare -r LOCAL_BIN="$PROJECT_ROOT/bin"
declare -r CSV_FILE=bundle/manifests/perses-operator.clusterserviceversion.yaml

VERSION=${VERSION:-"$(cat "$PROJECT_ROOT/VERSION")"}
VERSION_REPLACED=${VERSION_REPLACED:-}
BUNDLE_GEN_FLAGS=${BUNDLE_GEN_FLAGS:-}
IMG=${IMG:-}

declare -r VERSION BUNDLE_GEN_FLAGS IMG

main() {
	cd "$PROJECT_ROOT"
	export PATH="$LOCAL_BIN:$PATH"

	# NOTE: fetch the current version in the bundle csv, which is the
	# the version that this generation replaces

	local version_replaced="$VERSION_REPLACED"

	[[ -z "$version_replaced" ]] && {
		version_replaced="$(yq -r .spec.version "$CSV_FILE")"
	}

	local old_bundle_version="perses-operator.v${version_replaced}"

	# NOTE: if regenerating the same version, preserve the existing replaces value
	[[ "$version_replaced" == "$VERSION" ]] && {
		old_bundle_version=$(yq -r .spec.replaces "$CSV_FILE")
	}

	echo "Generating bundle version $VERSION (replaces: $old_bundle_version)"

	operator-sdk generate kustomize manifests --verbose
	(cd config/manager && kustomize edit set image "controller=${IMG}")

	local gen_opts=()
	[[ -n "$BUNDLE_GEN_FLAGS" ]] && {
		read -r -a gen_opts <<<"$BUNDLE_GEN_FLAGS"
	}

	kustomize build config/manifests |
		sed "s|<OLD_BUNDLE_VERSION>|${old_bundle_version}|g" |
		operator-sdk generate bundle "${gen_opts[@]}"

	# NOTE: operator-sdk may not preserve replaces from piped input, so fix it post-generation
	[[ "$version_replaced" != "$VERSION" ]] && {
		sed -e "s|replaces: .*|replaces: $old_bundle_version|g" \
			"$CSV_FILE" >"$CSV_FILE.tmp"
		mv "$CSV_FILE.tmp" "$CSV_FILE"
	}

	tree bundle/

	cat <<-EOF >bundle/ci.yaml
		---
		# Use replaces-mode or semver-mode. Once you switch to semver-mode, there is no easy way back.
		updateGraph: replaces-mode
		reviewers:
		  - jgbernalp
		  - Nexucis
	EOF

	operator-sdk bundle validate ./bundle \
		--select-optional name=operatorhub \
		--optional-values=k8s-version=1.25 \
		--select-optional suite=operatorframework

	# Reset CSV if only the createdAt timestamp changed
	if git diff --ignore-matching-lines='createdAt:' --exit-code "$CSV_FILE" >/dev/null 2>&1; then
		echo "No changes to $(basename "$CSV_FILE") detected; resetting it"
		git checkout -- "$CSV_FILE"
	fi

}

main "$@"
