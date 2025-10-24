package bootstrap

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	OutputTable = "table"
	OutputYAML  = "yaml"
)

// RenderOutput prints objects either as YAML or as table rows depending on format.
func RenderOutput(w io.Writer, format string, encode func(runtime.Object) ([]byte, error), table [][]string, objs []client.Object) error {
	switch strings.ToLower(format) {
	case "", OutputTable:
		return renderTable(w, table)
	case OutputYAML:
		return renderYAML(w, encode, objs)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func renderTable(w io.Writer, rows [][]string) error {
	if len(rows) == 0 {
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	return tw.Flush()
}

func renderYAML(w io.Writer, encode func(runtime.Object) ([]byte, error), objs []client.Object) error {
	for i, obj := range objs {
		if obj == nil {
			continue
		}
		data, err := encode(obj)
		if err != nil {
			return err
		}
		if i > 0 {
			if _, err := io.WriteString(w, "---\n"); err != nil {
				return err
			}
		}
		if _, err := io.Copy(w, bytes.NewReader(data)); err != nil {
			return err
		}
		if len(data) == 0 || data[len(data)-1] != '\n' {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}
