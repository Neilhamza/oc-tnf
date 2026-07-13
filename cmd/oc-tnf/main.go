package main

import (
	"fmt"
	"os"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/openshift/oc-tnf/pkg/cmd"
)

var (
	version = "unreleased"
	date    = "unknown"
)

func main() {
	streams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	root := cmd.NewCmdTNF(streams)
	root.Version = fmt.Sprintf("%s (%s)", version, date)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
