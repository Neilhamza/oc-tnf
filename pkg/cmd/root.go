package cmd

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/openshift/oc-tnf/pkg/cmd/validatefencing"
)

func NewCmdTNF(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "oc-tnf",
		Short:         "OpenShift Two Node with Fencing utilities",
		Long:          "oc-tnf provides utilities for managing and validating Two Node with Fencing (TNF) clusters.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(validatefencing.NewCmdValidateFencing(streams))

	return cmd
}
