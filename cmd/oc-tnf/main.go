package main

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
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
		logrus.Fatal(err)
	}
}
