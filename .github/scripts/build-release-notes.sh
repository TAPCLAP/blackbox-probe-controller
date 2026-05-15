#!/usr/bin/env bash
# Builds release-notes.md for a version tag push (expects fetch-depth: 0 checkout).
set -euo pipefail

: "${GITHUB_REPOSITORY:?}"
: "${GITHUB_REF:?}"
: "${GITHUB_SHA:?}"

REGISTRY="${REGISTRY:-ghcr.io}"
REPO_LC="${GITHUB_REPOSITORY,,}"
OWNER_LC="${GITHUB_REPOSITORY_OWNER,,}"
CURRENT_TAG="${GITHUB_REF#refs/tags/}"
VERSION="${CURRENT_TAG}"
IMAGE="${REGISTRY}/${REPO_LC}:${CURRENT_TAG}"
IMAGE_SHA="${REGISTRY}/${REPO_LC}:sha-${GITHUB_SHA:0:7}"
CHART_OCI="oci://${REGISTRY}/${REPO_LC}/helm-charts"
REPO_URL="https://github.com/${GITHUB_REPOSITORY}"

PREV_TAG=""
while IFS= read -r tag; do
  if [[ "${tag}" == "${CURRENT_TAG}" ]]; then
    break
  fi
  PREV_TAG="${tag}"
done < <(git tag -l 'v*' --sort=v:refname)

changelog() {
  local range=()
  if [[ -n "${PREV_TAG}" ]]; then
    range=("${PREV_TAG}..${CURRENT_TAG}")
  fi

  local entries
  if [[ ${#range[@]} -gt 0 ]]; then
    entries="$(git log "${range[@]}" \
      --pretty=format:'- %s ([%h]('"${REPO_URL}"'/commit/%H))' \
      --no-merges 2>/dev/null || true)"
  else
    entries="$(git log \
      --pretty=format:'- %s ([%h]('"${REPO_URL}"'/commit/%H))' \
      --no-merges 2>/dev/null || true)"
  fi

  if [[ -z "${entries}" ]]; then
    echo "_No commits since the previous tag (empty range or merge-only)._"
  else
    printf '%s\n' "${entries}"
  fi
}

{
  echo "## Installation"
  echo
  echo "### Helm"
  echo
  echo '```bash'
  echo "helm upgrade --install blackbox-probe-controller \\"
  echo "  ${CHART_OCI} \\"
  echo "  --version ${VERSION} \\"
  echo "  --namespace blackbox-probe-controller-system \\"
  echo "  --create-namespace"
  echo '```'
  echo
  echo "### Container image"
  echo
  echo "| Tag | Image |"
  echo "|-----|-------|"
  echo "| \`${CURRENT_TAG}\` | \`${IMAGE}\` |"
  echo "| \`sha-${GITHUB_SHA:0:7}\` | \`${IMAGE_SHA}\` |"
  echo
  echo "## Changelog"
  echo
  if [[ -n "${PREV_TAG}" ]]; then
    echo "Changes since [\`${PREV_TAG}\`](${REPO_URL}/releases/tag/${PREV_TAG}):"
  else
    echo "Initial release:"
  fi
  echo
  changelog
  echo
  if [[ -n "${PREV_TAG}" ]]; then
    echo "**Full diff:** ${REPO_URL}/compare/${PREV_TAG}...${CURRENT_TAG}"
  else
    echo "**Tag commit:** ${REPO_URL}/commit/${GITHUB_SHA}"
  fi
} > release-notes.md
