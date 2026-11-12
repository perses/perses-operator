#!/usr/bin/env bash
# Convert YAML files under a directory to JSON for the jsonnet library.
#
# Usage:
#   GOJSONTOYAML_BINARY=<path> scripts/yaml-to-jsonnet.sh <src-dir> [suffix] [--exclude <name>]...
#
# Example:
#   scripts/yaml-to-jsonnet.sh config/crd/bases -crd
#   scripts/yaml-to-jsonnet.sh config/manager '' --exclude namespace.yaml

set -euo pipefail

if [[ $# -lt 1 ]]; then
	echo "usage: $0 <src-dir> [suffix] [--exclude <name>]..." >&2
	exit 1
fi

src_dir=$1
shift

suffix=""
if [[ $# -gt 0 && $1 != --* ]]; then
	suffix=$1
	shift
fi

excludes=()
while [[ $# -gt 0 ]]; do
	case $1 in
	--exclude)
		excludes+=("$2")
		shift 2
		;;
	*)
		echo "unknown argument: $1" >&2
		exit 1
		;;
	esac
done

if [[ -z ${GOJSONTOYAML_BINARY:-} ]]; then
	echo "GOJSONTOYAML_BINARY must be set" >&2
	exit 1
fi

out_dir="$(pwd)/jsonnet/generated"
mkdir -p "${out_dir}"

find_args=("${src_dir}" -name '*.yaml' ! -name 'kustomization.yaml')
for name in "${excludes[@]}"; do
	find_args+=(! -name "${name}")
done

find "${find_args[@]}" -print0 | while IFS= read -r -d '' file; do
	filename=$(basename "${file}" .yaml)
	out_file="${out_dir}/${filename}${suffix}.json"
	"${GOJSONTOYAML_BINARY}" -yamltojson <"${file}" | jq >"${out_file}"
done
