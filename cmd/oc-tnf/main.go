package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/openshift/oc-tnf/pkg/cmd"
)

var (
	version = "unreleased"
	date    = "unknown"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() { <-ctx.Done(); stop() }()

	streams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	root := cmd.NewCmdTNF(streams)
	root.Use = filepath.Base(os.Args[0])
	root.Version = fmt.Sprintf("%s (%s)", version, date)
	if err := root.ExecuteContext(ctx); err != nil {
		if ctx.Err() != nil {
			fmt.Fprintln(os.Stderr, "Interrupted — if a node was fenced, the cluster should self-recover. Verify with: pcs status")
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
