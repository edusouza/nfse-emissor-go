package cli

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is overridden at build time via
// -ldflags "-X github.com/edusouza/nfse-emissor-go/internal/cli.version=v0.1.0".
// When empty, it is resolved from the embedded build info, which is what
// `go install` produces.
var version string

// Version returns the build version, falling back to the module version
// recorded by the Go toolchain and finally to "desenvolvimento".
func Version() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "desenvolvimento"
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "versao",
		Short: "Mostra a versao do nfse",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "nfse %s (%s/%s, %s)\n",
				Version(), runtime.GOOS, runtime.GOARCH, runtime.Version())
			return err
		},
	}
}
