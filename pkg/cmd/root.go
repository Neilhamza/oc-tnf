package cmd

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/openshift/oc-tnf/pkg/cmd/validatefencing"
)

func NewCmdTNF(streams genericclioptions.IOStreams) *cobra.Command {
	var debug bool

	cmd := &cobra.Command{
		Use:           "oc-tnf",
		Short:         "OpenShift Two Node with Fencing utilities",
		Long:          "oc-tnf provides utilities for managing and validating Two Node with Fencing (TNF) clusters.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if debug {
				logrus.SetLevel(logrus.DebugLevel)
			}
		},
	}

	cmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging")
	cmd.AddCommand(validatefencing.NewCmdValidateFencing(streams))

	return cmd
}
