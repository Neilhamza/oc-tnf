# oc-tnf

OpenShift CLI plugin for Two Node with Fencing (TNF) cluster utilities.

## Install

### Direct download

Download the binary for your platform from [GitHub Releases](https://github.com/openshift/oc-tnf/releases/latest) and place it on your PATH:

```bash
# Linux (amd64)
curl -LO https://github.com/openshift/oc-tnf/releases/latest/download/oc-tnf_linux_amd64.tar.gz
tar xzf oc-tnf_linux_amd64.tar.gz
sudo install oc-tnf /usr/local/bin/

# macOS (Apple Silicon)
curl -LO https://github.com/openshift/oc-tnf/releases/latest/download/oc-tnf_darwin_arm64.tar.gz
tar xzf oc-tnf_darwin_arm64.tar.gz
sudo install oc-tnf /usr/local/bin/
```

### Via Krew

```bash
kubectl krew index add openshift-tnf https://github.com/openshift/oc-tnf.git
kubectl krew install openshift-tnf/tnf
```

Krew will print a trust warning for custom indexes — this is expected. After install, the command is `oc tnf validate-fencing`.

### From source

```bash
git clone https://github.com/openshift/oc-tnf.git
cd oc-tnf
make build
make install
```

## Commands

### validate-fencing

Validates fencing on a TNF (DualReplica) cluster by power-cycling both nodes sequentially via STONITH and verifying full recovery.

**WARNING: This is a DISRUPTIVE operation.** Nodes will be forcibly powered off via STONITH and must recover automatically. Only run this against a TNF cluster you intend to test.

**What it checks:**

1. Pre-flight: STONITH configured and enabled, both nodes online in Pacemaker, corosync/pacemaker/pcsd daemons active, etcd has 2 healthy voter members
2. Fences each node sequentially via `pcs stonith fence`
3. Waits for fenced node to go NotReady, then recover to Ready
4. Verifies Pacemaker rejoin and etcd quorum restoration
5. Warns if fencing takes >60s (possible BMC graceful shutdown)

**Usage:**

```bash
# Using default kubeconfig and SSH agent
oc tnf validate-fencing

# With explicit SSH key
oc tnf validate-fencing --ssh-key ~/.ssh/id_rsa

# With explicit kubeconfig
oc tnf validate-fencing --kubeconfig /path/to/kubeconfig --ssh-key ~/.ssh/id_rsa
```

**Prerequisites:**

- A deployed DualReplica (TNF) cluster — rejects non-TNF clusters with a clear error
- SSH access to both control-plane nodes as `core` user
- Cluster-admin kubeconfig

## Development

```bash
make build            # Build for current platform
make test             # Run unit tests
make golangci-lint    # Run linter
make cross-build      # Build for all platforms
make release-dry-run  # Test GoReleaser without publishing
make krew-manifest    # Regenerate plugins/tnf.yaml from dist/checksums.txt
```
