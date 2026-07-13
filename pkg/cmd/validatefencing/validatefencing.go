package validatefencing

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/oc-tnf/pkg/fencing"
)

type ValidateFencingOptions struct {
	configFlags          *genericclioptions.ConfigFlags
	sshKeys              []string
	yes                  bool
	insecureHostKeyCheck bool

	kubeClient kubernetes.Interface
	cfgClient  configclient.Interface
	restConfig *rest.Config

	genericclioptions.IOStreams
}

func NewCmdValidateFencing(streams genericclioptions.IOStreams) *cobra.Command {
	o := &ValidateFencingOptions{
		configFlags: genericclioptions.NewConfigFlags(true),
		IOStreams:   streams,
	}

	cmd := &cobra.Command{
		Use:   "validate-fencing",
		Short: "Validate fencing by power-cycling both nodes sequentially (DISRUPTIVE)",
		Long: `Validate fencing on a Two Node with Fencing cluster.

This command connects to both control plane nodes via SSH, runs pre-flight
checks (STONITH, Pacemaker, etcd), then fences each node sequentially and
verifies recovery.

WARNING: This is a DISRUPTIVE operation — nodes will be forcibly powered off
via STONITH and must recover automatically.

Requires SSH access to both nodes as user "core" and a cluster-admin kubeconfig.`,
		Example: `  # Validate fencing using default kubeconfig and SSH agent
  oc tnf validate-fencing

  # Validate with explicit SSH key
  oc tnf validate-fencing --ssh-key ~/.ssh/id_rsa

  # Validate with explicit kubeconfig
  oc tnf validate-fencing --kubeconfig /path/to/kubeconfig --ssh-key ~/.ssh/id_rsa`,
		Args: cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(); err != nil {
				return err
			}
			if err := o.Validate(cmd.Context()); err != nil {
				return err
			}
			return o.Run(cmd.Context())
		},
	}

	cmd.Flags().StringArrayVar(&o.sshKeys, "ssh-key", nil,
		"Path to SSH private key files for node access. May be specified multiple times. If not provided, SSH agent or ~/.ssh/ defaults are used.")
	cmd.Flags().BoolVar(&o.yes, "yes", false,
		"Skip confirmation prompt and proceed with fencing immediately.")
	cmd.Flags().BoolVar(&o.insecureHostKeyCheck, "insecure-skip-host-key-check", false,
		"Skip SSH host key verification. By default, ~/.ssh/known_hosts is used.")
	o.configFlags.AddFlags(cmd.Flags())

	return cmd
}

func (o *ValidateFencingOptions) Complete() error {
	var err error
	o.restConfig, err = o.configFlags.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	o.kubeClient, err = kubernetes.NewForConfig(o.restConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	o.cfgClient, err = configclient.NewForConfig(o.restConfig)
	if err != nil {
		return fmt.Errorf("failed to create OpenShift config client: %w", err)
	}

	return nil
}

func (o *ValidateFencingOptions) Validate(ctx context.Context) error {
	infra, err := o.cfgClient.ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to read Infrastructure CR: %w", err)
	}
	if infra.Status.ControlPlaneTopology != configv1.DualReplicaTopologyMode {
		return fmt.Errorf("this command requires a Two Node with Fencing (DualReplica) cluster, found %q", infra.Status.ControlPlaneTopology)
	}
	return nil
}

func (o *ValidateFencingOptions) Run(ctx context.Context) error {
	nodes, err := fencing.DiscoverNodes(ctx, o.kubeClient)
	if err != nil {
		return fmt.Errorf("node discovery failed: %w", err)
	}

	if !o.yes {
		fmt.Fprintf(o.Out, "About to fence nodes [%s, %s] on %s\n",
			nodes[0].Name, nodes[1].Name, o.restConfig.Host)
		fmt.Fprint(o.Out, "This will forcibly power-cycle both control-plane nodes. Continue? [y/N] ")

		reader := bufio.NewReader(o.In)
		answer, _ := reader.ReadString('\n')
		if !strings.EqualFold(strings.TrimSpace(answer), "y") {
			return fmt.Errorf("aborted by user")
		}
	}

	return fencing.Run(ctx, fencing.Config{
		KubeClient:      o.kubeClient,
		SSHUser:         "core",
		SSHKeys:         o.sshKeys,
		InsecureHostKey: o.insecureHostKeyCheck,
		Nodes:           nodes,
	})
}
