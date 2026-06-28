package validatefencing

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/oc-tnf/pkg/fencing"
)

type ValidateFencingOptions struct {
	configFlags *genericclioptions.ConfigFlags
	sshKeys     []string

	kubeClient kubernetes.Interface
	cfgClient  configclient.Interface

	genericclioptions.IOStreams
}

func NewCmdValidateFencing(streams genericclioptions.IOStreams) *cobra.Command {
	o := &ValidateFencingOptions{
		configFlags: genericclioptions.NewConfigFlags(true),
		IOStreams:    streams,
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
	o.configFlags.AddFlags(cmd.Flags())

	return cmd
}

func (o *ValidateFencingOptions) Complete() error {
	restConfig, err := o.configFlags.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	o.kubeClient, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	o.cfgClient, err = configclient.NewForConfig(restConfig)
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
	return fencing.Run(ctx, fencing.Config{
		KubeClient: o.kubeClient,
		SSHUser:    "core",
		SSHKeys:    o.sshKeys,
	})
}
