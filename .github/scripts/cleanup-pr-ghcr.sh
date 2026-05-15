#!/usr/bin/env bash
# Delete GHCR package versions tagged for a closed pull request.
set -euo pipefail

PR_NUMBER="${1:?PR number required}"
OWNER="${2:?GitHub owner required}"
IMAGE_PACKAGE="${3:-blackbox-probe-controller}"
CHART_PACKAGE="${4:-blackbox-probe-controller/helm-charts}"

api_root() {
  if gh api "orgs/${OWNER}" &>/dev/null; then
    echo "orgs/${OWNER}"
  else
    echo "user/${OWNER}"
  fi
}

delete_matching_versions() {
  local package_name="$1"
  local root
  root="$(api_root)"

  echo "Cleaning package ${package_name}"

  mapfile -t version_ids < <(
    gh api "${root}/packages/container/${package_name}/versions" --paginate \
      --jq ".[] | select(
        ([.metadata.container.tags[]? | startswith(\"pr-${PR_NUMBER}-\")] | any)
        or ([.metadata.container.tags[]? | startswith(\"0.0.0-pr.${PR_NUMBER}.\")] | any)
      ) | .id" \
      2>/dev/null || true
  )

  if [[ ${#version_ids[@]} -eq 0 ]]; then
    echo "  No matching versions"
    return 0
  fi

  for id in "${version_ids[@]}"; do
    echo "  Deleting version id ${id}"
    gh api --method DELETE "${root}/packages/container/${package_name}/versions/${id}" || true
  done
}

delete_matching_versions "${IMAGE_PACKAGE}"
delete_matching_versions "${CHART_PACKAGE}"

echo "Cleanup finished for PR #${PR_NUMBER}"
