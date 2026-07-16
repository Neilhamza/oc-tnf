#!/bin/bash

set -euo pipefail

MANIFEST="plugins/tnf.yaml"

if [ ! -f "${MANIFEST}" ]; then
  echo "ERROR: ${MANIFEST} not found"
  exit 1
fi

errors=0

echo -n "Running yamllint... "
if yamllint -d '{extends: default, rules: {line-length: disable}}' "${MANIFEST}" >/dev/null 2>&1; then
  echo "OK"
elif ! command -v yamllint >/dev/null 2>&1 || ! yamllint --version >/dev/null 2>&1; then
  echo "SKIP (not installed or broken)"
else
  echo "FAIL"
  yamllint -d '{extends: default, rules: {line-length: disable}}' "${MANIFEST}" || true
  errors=$((errors + 1))
fi

echo -n "Running validate-krew-manifest... "
if validate-krew-manifest -manifest "${MANIFEST}" >/dev/null 2>&1; then
  echo "OK"
elif ! command -v validate-krew-manifest >/dev/null 2>&1; then
  echo "SKIP (go install sigs.k8s.io/krew/cmd/validate-krew-manifest@latest)"
else
  echo "FAIL"
  validate-krew-manifest -manifest "${MANIFEST}" || true
  errors=$((errors + 1))
fi

VERSION=$(grep '  version:' "${MANIFEST}" | awk '{print $2}')
if [ -z "${VERSION}" ]; then
  echo "ERROR: could not extract version from ${MANIFEST}"
  exit 1
fi

REPO="openshift/oc-tnf"
BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"

PLATFORMS=(linux_amd64 linux_arm64 darwin_amd64 darwin_arm64)

TMPDIR=$(mktemp -d)
trap 'rm -rf "${TMPDIR}"' EXIT

for platform in "${PLATFORMS[@]}"; do
  artifact="oc-tnf_${platform}.tar.gz"
  url="${BASE_URL}/${artifact}"

  # Field order depends on the generated manifest template — uri then sha256 on the next line
  manifest_sha=$( (grep -A2 "/${artifact}" "${MANIFEST}" | grep 'sha256:' | awk '{print $2}' | tr -d '"') || true )
  if [ -z "${manifest_sha}" ]; then
    echo "FAIL: no sha256 found in manifest for ${artifact}"
    errors=$((errors + 1))
    continue
  fi

  echo -n "Verifying ${artifact}... "
  if ! curl -fsSL --retry 3 --retry-delay 2 -o "${TMPDIR}/${artifact}" "${url}" 2>/dev/null; then
    echo "FAIL: could not download ${url}"
    errors=$((errors + 1))
    continue
  fi

  actual_sha=$(sha256sum "${TMPDIR}/${artifact}" 2>/dev/null || shasum -a 256 "${TMPDIR}/${artifact}")
  actual_sha=$(echo "${actual_sha}" | awk '{print $1}')
  if [ "${manifest_sha}" = "${actual_sha}" ]; then
    echo "OK"
  else
    echo "FAIL: sha256 mismatch"
    echo "  manifest: ${manifest_sha}"
    echo "  actual:   ${actual_sha}"
    errors=$((errors + 1))
  fi
done

if [ "${errors}" -gt 0 ]; then
  echo ""
  echo "FAILED: ${errors} verification error(s)"
  exit 1
fi

echo ""
echo "All checksums verified for ${VERSION}"
