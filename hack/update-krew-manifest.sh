#!/bin/bash

set -euo pipefail

if [ ! -f dist/checksums.txt ]; then
    echo "dist/checksums.txt not found — run make release or make release-dry-run first"
    exit 1
fi

TAG=$(git describe --tags --exact-match 2>/dev/null) || { echo "HEAD is not tagged — tag the release first"; exit 1; }
REPO="openshift/oc-tnf"

SHA_LINUX_AMD64=$(grep -E 'oc-tnf_linux_amd64\.tar\.gz$' dist/checksums.txt | awk '{print $1}')
SHA_LINUX_ARM64=$(grep -E 'oc-tnf_linux_arm64\.tar\.gz$' dist/checksums.txt | awk '{print $1}')
SHA_DARWIN_AMD64=$(grep -E 'oc-tnf_darwin_amd64\.tar\.gz$' dist/checksums.txt | awk '{print $1}')
SHA_DARWIN_ARM64=$(grep -E 'oc-tnf_darwin_arm64\.tar\.gz$' dist/checksums.txt | awk '{print $1}')

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
