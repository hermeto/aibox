package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/hermeto/aibox/internal/version"
)

type versionInfo version.Info

func (v versionInfo) RenderText(w io.Writer) error {
	_, err := fmt.Fprintf(w, "aibox %s (commit %s, built %s)\n", v.Version, v.Commit, v.Date)
	return err
}

func newVersionCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return app.print(versionInfo(version.Get()))
		},
	}
}
