#!/bin/bash

set -euo pipefail

TAG=$(git describe --tags --exact-match 2>/dev/null) \
  || { echo "ERROR: HEAD is not tagged — tag the release first"; exit 1; }

REPO="openshift/oc-tnf"

CHECKSUMS=$(curl -fsSL --retry 3 --retry-delay 2 "https://github.com/${REPO}/releases/download/${TAG}/checksums.txt") \
  || { echo "ERROR: release ${TAG} not published or checksums.txt missing"; exit 1; }

sha() { echo "$CHECKSUMS" | grep -E "oc-tnf_${1}\.tar\.gz$" | awk '{print $1}' || true; }

SHA_LINUX_AMD64=$(sha linux_amd64)
SHA_LINUX_ARM64=$(sha linux_arm64)
SHA_DARWIN_AMD64=$(sha darwin_amd64)
SHA_DARWIN_ARM64=$(sha darwin_arm64)

for var in SHA_LINUX_AMD64 SHA_LINUX_ARM64 SHA_DARWIN_AMD64 SHA_DARWIN_ARM64; do
  if [ -z "${!var}" ]; then
    echo "ERROR: could not extract checksum for ${var} from release ${TAG}"
    exit 1
  fi
done

cat > plugins/tnf.yaml <<EOF
apiVersion: krew.googlecontainertools.github.com/v1alpha2
kind: Plugin
metadata:
  name: tnf
spec:
  version: ${TAG}
  homepage: https://github.com/${REPO}
  shortDescription: Validate STONITH fencing on TNF clusters
  description: |
    oc-tnf provides tooling for OpenShift Two Node with Fencing (TNF)
    clusters. The validate-fencing subcommand fences each cluster node
    sequentially via STONITH and verifies recovery, including Pacemaker
    rejoin and etcd quorum restoration.

    WARNING: validate-fencing is disruptive. It power-fences real nodes.
    Only run it against a TNF cluster you intend to test.
  caveats: |
    Requires SSH access to the cluster nodes (--ssh-key) and a valid
    kubeconfig for the TNF cluster. This plugin only works on OpenShift
    DualReplica (TNF) clusters.
  platforms:
    - selector:
        matchLabels:
          os: linux
          arch: amd64
      uri: https://github.com/${REPO}/releases/download/${TAG}/oc-tnf_linux_amd64.tar.gz
      sha256: "${SHA_LINUX_AMD64}"
      bin: oc-tnf
    - selector:
        matchLabels:
          os: linux
          arch: arm64
      uri: https://github.com/${REPO}/releases/download/${TAG}/oc-tnf_linux_arm64.tar.gz
      sha256: "${SHA_LINUX_ARM64}"
      bin: oc-tnf
    - selector:
        matchLabels:
          os: darwin
          arch: amd64
      uri: https://github.com/${REPO}/releases/download/${TAG}/oc-tnf_darwin_amd64.tar.gz
      sha256: "${SHA_DARWIN_AMD64}"
      bin: oc-tnf
    - selector:
        matchLabels:
          os: darwin
          arch: arm64
      uri: https://github.com/${REPO}/releases/download/${TAG}/oc-tnf_darwin_arm64.tar.gz
      sha256: "${SHA_DARWIN_ARM64}"
      bin: oc-tnf
EOF

echo "Updated plugins/tnf.yaml for ${TAG}"
