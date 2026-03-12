#!/usr/bin/env bash

# Copyright The Perses Authors
# Licensed under the Apache License, Version 2.0

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

COMMUNITY_OPERATORS_REPO="k8s-operatorhub/community-operators"
OPERATOR_NAME="perses-operator"

usage() {
  cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Prepare a commit for the community-operators repository to publish a new
version of perses-operator to OperatorHub.

Must be run from the release branch (e.g. release/v0.3) after generating
the bundle with: make bundle REPLACES_VERSION=<previous-version>

Options:
  --fork OWNER/REPO        Your fork of community-operators (e.g. myuser/community-operators)
  --remote REMOTE_NAME     Git remote name in community-operators repo to push to (default: origin)
  --workdir DIR            Path to an existing local clone of community-operators,
                           or directory to clone into (default: /tmp/community-operators)
  -h, --help               Show this help message

Examples:
  # Basic usage
  ./scripts/publish-to-operatorhub/publish-to-operatorhub.sh --fork myuser/community-operators

  # Use a different git remote name for pushing
  ./scripts/publish-to-operatorhub/publish-to-operatorhub.sh --fork myuser/community-operators --remote fork-remotename

  # Use an existing local clone of community-operators
  ./scripts/publish-to-operatorhub/publish-to-operatorhub.sh --fork myuser/community-operators --workdir ~/github/community-operators
EOF
  exit 0
}

FORK=""
REMOTE="origin"
WORKDIR="/tmp/community-operators"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --fork)
      FORK="$2"
      shift 2
      ;;
    --remote)
      REMOTE="$2"
      shift 2
      ;;
    --workdir)
      WORKDIR="$2"
      shift 2
      ;;
    -h|--help)
      usage
      ;;
    *)
      echo "Unknown option: $1"
      usage
      ;;
  esac
done

if [[ -z "${FORK}" ]]; then
  echo "Error: --fork is required (e.g. --fork myuser/community-operators)"
  exit 1
fi

BUNDLE_DIR="${REPO_ROOT}/bundle"

cd "${REPO_ROOT}"

# Fetch tags from upstream and determine the latest release version
OPERATOR_UPSTREAM="${GIT_REMOTE_UPSTREAM:-upstream}"
echo "==> Fetching tags from ${OPERATOR_UPSTREAM}..."
git fetch "${OPERATOR_UPSTREAM}" --tags

VERSION=$(git tag --list "v*" --sort=-v:refname | head -1 | sed 's/^v//')
if [[ -z "${VERSION}" ]]; then
  echo "Error: no release tags found. Has a release been created?"
  exit 1
fi

echo "==> Latest release version: ${VERSION}"

MAJOR_MINOR=$(echo "${VERSION}" | sed 's/\.[0-9]*$//')
EXPECTED_BRANCH="release/v${MAJOR_MINOR}"

# Verify the script is run from the correct release branch
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [[ "${CURRENT_BRANCH}" != "${EXPECTED_BRANCH}" ]]; then
  echo "Error: latest release is v${VERSION}, expected to be on branch '${EXPECTED_BRANCH}', but currently on '${CURRENT_BRANCH}'."
  echo "  Switch to the release branch first: git checkout ${EXPECTED_BRANCH}"
  exit 1
fi

# Check if the release branch is in sync with upstream
echo "==> Checking if ${CURRENT_BRANCH} is in sync with ${OPERATOR_UPSTREAM}/${EXPECTED_BRANCH}..."
git fetch "${OPERATOR_UPSTREAM}" "${EXPECTED_BRANCH}"
LOCAL_HEAD=$(git rev-parse HEAD)
UPSTREAM_HEAD=$(git rev-parse "${OPERATOR_UPSTREAM}/${EXPECTED_BRANCH}")
if [[ "${LOCAL_HEAD}" != "${UPSTREAM_HEAD}" ]]; then
  echo "WARNING: local '${CURRENT_BRANCH}' is not in sync with '${OPERATOR_UPSTREAM}/${EXPECTED_BRANCH}'."
  echo "  Local:    ${LOCAL_HEAD}"
  echo "  Upstream: ${UPSTREAM_HEAD}"
  echo "  To sync: git pull ${OPERATOR_UPSTREAM} ${EXPECTED_BRANCH}"
  echo ""
  read -r -p "Continue anyway? [y/N] " response
  if [[ ! "${response}" =~ ^[Yy]$ ]]; then
    echo "Aborting."
    exit 1
  fi
fi

TARGET_DIR="${WORKDIR}/operators/${OPERATOR_NAME}/${VERSION}"
BRANCH_NAME="update-${OPERATOR_NAME}-${VERSION}"

# Verify the bundle has been generated
if [[ ! -d "${BUNDLE_DIR}/manifests" ]] || [[ ! -d "${BUNDLE_DIR}/metadata" ]]; then
  echo "Error: bundle/manifests or bundle/metadata not found."
  echo "  Generate the bundle first: make bundle REPLACES_VERSION=<previous-version>"
  exit 1
fi

echo "==> Preparing ${OPERATOR_NAME} v${VERSION} bundle for OperatorHub submission"
echo "    Source: local bundle from branch ${CURRENT_BRANCH}"
echo "    Fork: ${FORK}"
echo "    Working directory: ${WORKDIR}"

# Clone or update the fork
if [[ -d "${WORKDIR}/.git" ]]; then
  cd "${WORKDIR}"
  REMOTE_URL=$(git remote get-url origin 2>/dev/null || echo "")
  if [[ "${REMOTE_URL}" != *"community-operators"* ]]; then
    echo "Error: ${WORKDIR} exists but does not appear to be a community-operators clone."
    echo "  origin URL: ${REMOTE_URL}"
    exit 1
  fi
  echo "==> Updating existing clone..."
  git fetch origin main
  git checkout main
  git reset --hard origin/main
else
  echo "==> Cloning fork..."
  git clone "git@github.com:${FORK}.git" "${WORKDIR}"
  cd "${WORKDIR}"
  git remote add upstream "git@github.com:${COMMUNITY_OPERATORS_REPO}.git" 2>/dev/null || true
  git fetch upstream main
  git reset --hard upstream/main
fi

# Create a branch
echo "==> Creating branch ${BRANCH_NAME}..."
git checkout -B "${BRANCH_NAME}"

# Copy bundle from local working directory
echo "==> Copying bundle to ${TARGET_DIR}..."
mkdir -p "${TARGET_DIR}"
cp -r "${BUNDLE_DIR}/manifests" "${TARGET_DIR}/manifests"
cp -r "${BUNDLE_DIR}/metadata" "${TARGET_DIR}/metadata"

# Verify the replaces field is present
if ! grep -q "replaces:" "${TARGET_DIR}/manifests/${OPERATOR_NAME}.clusterserviceversion.yaml"; then
  echo ""
  echo "WARNING: CSV does not contain a 'replaces' field."
  echo "  The community-operators ci.yaml uses replaces-mode."
  echo "  Re-generate the bundle with: make bundle REPLACES_VERSION=<previous-version>"
  echo ""
fi

echo "==> Committing changes..."
git add "operators/${OPERATOR_NAME}/${VERSION}/"
git commit -s -S -m "operator ${OPERATOR_NAME} (${VERSION})"

echo ""
echo "==> Bundle prepared at: ${TARGET_DIR}"
echo ""
echo "Next steps:"
echo "  1. Review the changes:"
echo "       cd ${WORKDIR}"
echo "       git diff HEAD~1"
echo ""
echo "  2. Push the branch to your fork:"
echo "       git push -u ${REMOTE} ${BRANCH_NAME}"
echo ""
echo "  3. Create a PR at:"
echo "       https://github.com/${COMMUNITY_OPERATORS_REPO}/compare/main...${FORK%%/*}:${BRANCH_NAME}"
