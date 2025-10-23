package main

import (
	"os"

	"github.com/vaheed/kubeOP/kubeop-operator/internal/cli/bootstrap"
)

func main() {
	cmd := bootstrap.NewCommand(bootstrap.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
