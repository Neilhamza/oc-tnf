#!/bin/bash

set -euo pipefail

TAG=$(git describe --tags --exact-match 2>/dev/null) \
  || { echo "ERROR: HEAD is not tagged"; exit 1; }

FORK_REMOTE="${FORK_REMOTE:-origin}"
BRANCH="krew-update-${TAG}"
START_BRANCH=$(git rev-parse --abbrev-ref HEAD)

trap 'git checkout -q "${START_BRANCH}" 2>/dev/null || true' EXIT

gh auth status >/dev/null 2>&1 \
  || { echo "ERROR: gh is not authenticated — run 'gh auth login' or export GITHUB_TOKEN"; exit 1; }

FORK_URL=$(git remote get-url "${FORK_REMOTE}")
if echo "${FORK_URL}" | grep -qE '[:/]openshift/oc-tnf(\.git)?$'; then
  echo "ERROR: FORK_REMOTE (${FORK_REMOTE}) points at openshift/oc-tnf — set FORK_REMOTE to your personal fork"
  exit 1
fi

if git diff HEAD --quiet -- plugins/tnf.yaml 2>/dev/null; then
  echo "ERROR: plugins/tnf.yaml is unchanged — nothing to open a PR for"
  exit 1
fi

git branch -D "${BRANCH}" 2>/dev/null || true
git checkout -b "${BRANCH}"
git add plugins/tnf.yaml
git commit -m "Update Krew manifest for ${TAG}"
git push --force-with-lease "${FORK_REMOTE}" "${BRANCH}"

FORK_OWNER=$(echo "${FORK_URL}" | sed -E 's|/+$||; s|.*[:/]([^/]+)/[^/]+(\.git)?$|\1|')

gh pr create \
  --repo openshift/oc-tnf \
  --head "${FORK_OWNER}:${BRANCH}" \
  --title "Update Krew manifest for ${TAG}" \
  --body "Updates \`plugins/tnf.yaml\` with ${TAG} release URLs and checksums.

Checksums fetched from the published GitHub release — verified against uploaded artifacts." \
  || { gh pr view "${BRANCH}" --repo openshift/oc-tnf >/dev/null 2>&1 \
    && echo ">>> PR already exists for ${BRANCH}"; }

echo ""
echo ">>> Manifest PR opened — ping a teammate for /lgtm"
