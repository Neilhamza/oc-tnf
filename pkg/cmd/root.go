package cmd

import (
	"fmt"

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
	cmd.AddCommand(newCompletionCmd())

	return cmd
}

func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "completion [bash|zsh|fish|powershell]",
		Short:     "Generate shell completion script",
		Long:      "Generate shell completion script. For bash, registers completion for both oc-tnf and kubectl-tnf binary names.",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			switch args[0] {
			case "bash":
				if err := root.GenBashCompletionV2(cmd.OutOrStdout(), true); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout())
				fmt.Fprintln(cmd.OutOrStdout(), `# Also register for kubectl-tnf so completion works regardless of binary name`)
				fmt.Fprintln(cmd.OutOrStdout(), `complete -o default -F __start_oc-tnf kubectl-tnf`)
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			}
			return nil
		},
	}
}
