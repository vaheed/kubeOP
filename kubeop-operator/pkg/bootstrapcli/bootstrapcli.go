package bootstrapcli

import (
	"github.com/spf13/cobra"
	"github.com/vaheed/kubeOP/kubeop-operator/internal/cli/bootstrap"
)

// IOStreams exposes the bootstrap IOStreams type to external callers that
// cannot import the operator's internal packages.
type IOStreams = bootstrap.IOStreams

// NewCommand constructs the bootstrap CLI root command.
func NewCommand(streams IOStreams) *cobra.Command {
	return bootstrap.NewCommand(streams)
}
