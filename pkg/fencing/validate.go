// Package fencing validates fencing on DualReplica (Two Node with Fencing) clusters
// by fencing both nodes sequentially and verifying recovery.
package fencing

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	gossh "golang.org/x/crypto/ssh"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/openshift/oc-tnf/pkg/ssh"
)

const (
	sshPort          = "22"
	pollInterval     = 5 * time.Second
	fenceWarnSeconds = 60
	sshCmdTimeout    = 120 // seconds, per-command timeout for SSH commands
)

// NodeInfo holds resolved information about a cluster node.
type NodeInfo struct {
	Name          string
	PacemakerName string
	IP            string
}

// Config holds all inputs needed by the fencing validator.
type Config struct {
	KubeClient      kubernetes.Interface
	SSHUser         string
	SSHKeys         []string
	InsecureHostKey bool
	Nodes           [2]NodeInfo
	ReadyTimeout    time.Duration
	NotReadyTimeout time.Duration
}

// DiscoverNodes finds the two control-plane nodes and returns their info.
func DiscoverNodes(ctx context.Context, kube kubernetes.Interface) ([2]NodeInfo, error) {
	return discoverNodes(ctx, kube)
}

// Run executes the full fencing validation sequence.
func Run(ctx context.Context, cfg Config) error {
	var nodes [2]NodeInfo
	var err error
	if cfg.Nodes[0].Name != "" {
		nodes = cfg.Nodes
	} else {
		nodes, err = discoverNodes(ctx, cfg.KubeClient)
		if err != nil {
			return fmt.Errorf("node discovery failed: %w", err)
		}
	}

	logrus.Infof("Connecting to %s (%s)", nodes[0].Name, nodes[0].IP)
	clientA, err := sshConnect(cfg, nodes[0].IP)
	if err != nil {
		return fmt.Errorf("ssh to %s failed: %w", nodes[0].Name, err)
	}
	defer clientA.Close()

	if err := resolvePacemakerNames(clientA, nodes[:]); err != nil {
		return err
	}

	logrus.Info("Running pre-flight checks")
	if err := runPreFlight(clientA, nodes[:]); err != nil {
		return fmt.Errorf("pre-flight check failed: %w", err)
	}
	logrus.Info("Pre-flight checks passed")

	if err := fenceAndRecover(ctx, cfg, clientA, nodes[:], 1); err != nil {
		return err
	}

	logrus.Infof("Connecting to %s (%s)", nodes[1].Name, nodes[1].IP)
	clientB, err := sshConnect(cfg, nodes[1].IP)
	if err != nil {
		return fmt.Errorf("ssh to %s failed: %w", nodes[1].Name, err)
	}
	defer clientB.Close()

	if err := fenceAndRecover(ctx, cfg, clientB, nodes[:], 0); err != nil {
		return err
	}

	logrus.Info("Fencing validation passed")
	return nil
}

func fenceAndRecover(ctx context.Context, cfg Config, survivorClient *gossh.Client, nodes []NodeInfo, targetIdx int) error {
	target := nodes[targetIdx]
	survivor := nodes[1-targetIdx]

	logrus.Infof("Fencing %s (pacemaker: %s) from %s", target.Name, target.PacemakerName, survivor.Name)
	fenceStart := time.Now()

	if err := fenceNode(survivorClient, target.PacemakerName); err != nil {
		return fmt.Errorf("failed to fence %s: %w", target.Name, err)
	}

	fenceCmdDuration := time.Since(fenceStart)

	logrus.Infof("Waiting for %s to become NotReady", target.Name)
	if err := waitNotReady(ctx, cfg.KubeClient, target.Name, cfg.NotReadyTimeout); err != nil {
		return fmt.Errorf("%s did not become NotReady: %w", target.Name, err)
	}

	notReadyDuration := time.Since(fenceStart)
	if fenceCmdDuration.Seconds() > fenceWarnSeconds {
		logrus.Warnf("Fence command for %s took %s (threshold is %ds). BMC may be performing graceful shutdown instead of power-off. Check BMC configuration.",
			target.Name, fenceCmdDuration.Round(time.Second), fenceWarnSeconds)
	}
	logrus.Infof("Node %s: fence command %s, observed NotReady after %s",
		target.Name, fenceCmdDuration.Round(time.Second), notReadyDuration.Round(time.Second))

	logrus.Infof("Waiting for %s to become Ready", target.Name)
	if err := waitReady(ctx, cfg.KubeClient, target.Name, cfg.ReadyTimeout); err != nil {
		return fmt.Errorf("%s did not become Ready: %w", target.Name, err)
	}

	logrus.Infof("Waiting for %s to rejoin Pacemaker", target.Name)
	if err := pollPacemakerOnline(ctx, survivorClient, nodes); err != nil {
		return fmt.Errorf("%s did not rejoin Pacemaker: %w", target.Name, err)
	}

	logrus.Info("Waiting for etcd quorum")
	if err := pollEtcdHealth(ctx, survivorClient, nodes); err != nil {
		return fmt.Errorf("etcd quorum not restored after fencing %s: %w", target.Name, err)
	}

	if err := checkDaemons(survivorClient); err != nil {
		return fmt.Errorf("daemon check failed after fencing %s: %w", target.Name, err)
	}

	logrus.Infof("Fencing %s: full recovery completed in %s", target.Name, time.Since(fenceStart).Round(time.Second))
	return nil
}

func runPreFlight(client *gossh.Client, nodes []NodeInfo) error {
	if err := checkStonith(client); err != nil {
		return err
	}
	if err := checkPacemakerOnline(client, nodes); err != nil {
		return err
	}
	if err := checkDaemons(client); err != nil {
		return err
	}
	return checkEtcdHealth(client, nodes)
}

func discoverNodes(ctx context.Context, kube kubernetes.Interface) ([2]NodeInfo, error) {
	nodeList, err := kube.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/master=",
	})
	if err != nil {
		return [2]NodeInfo{}, fmt.Errorf("listing control-plane nodes: %w", err)
	}

	var nodes []NodeInfo
	for i := range nodeList.Items {
		n := &nodeList.Items[i]
		ip := nodeInternalIP(n)
		if ip == "" {
			return [2]NodeInfo{}, fmt.Errorf("node %s has no InternalIP", n.Name)
		}
		nodes = append(nodes, NodeInfo{Name: n.Name, IP: ip})
	}

	if len(nodes) != 2 {
		return [2]NodeInfo{}, fmt.Errorf("expected 2 control-plane nodes, found %d", len(nodes))
	}

	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })
	logrus.Infof("Discovered nodes: %s (%s), %s (%s)", nodes[0].Name, nodes[0].IP, nodes[1].Name, nodes[1].IP)
	return [2]NodeInfo{nodes[0], nodes[1]}, nil
}

func sshConnect(cfg Config, ip string) (*gossh.Client, error) {
	addr := net.JoinHostPort(ip, sshPort)
	return ssh.NewClient(cfg.SSHUser, addr, cfg.SSHKeys, cfg.InsecureHostKey)
}

func sshRun(client *gossh.Client, cmd string) (string, error) {
	return sshRunTimeout(client, cmd, sshCmdTimeout)
}

func sshRunTimeout(client *gossh.Client, cmd string, timeoutSec int) (string, error) {
	wrapped := fmt.Sprintf("sudo timeout %d bash -lc %s", timeoutSec, shellQuote(cmd))
	stdout, stderr, err := ssh.RunOutput(client, wrapped)
	if err != nil {
		return stdout, fmt.Errorf("command failed: %s\nstderr: %s: %w", cmd, stderr, err)
	}
	return stdout, nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
