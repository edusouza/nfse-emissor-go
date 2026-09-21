// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

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

// maxAppVersionLength is the limit TSVerAplic imposes on verAplic in the DPS
// and in event requests.
const maxAppVersionLength = 20

// AppVersion returns the identifier written to verAplic.
//
// The field is capped at 20 characters by the schema, while Version() can be a
// Go pseudo-version like "v0.0.0-20260918131458-fed93cba325f+dirty" — 40
// characters on its own. Overflowing it gets the whole declaration rejected,
// so the version is truncated rather than the name dropped: knowing the
// document came from this tool matters more than knowing the exact build.
func AppVersion() string {
	const prefix = "nfse-cli "

	version := Version()
	if len(prefix)+len(version) <= maxAppVersionLength {
		return prefix + version
	}
	return prefix + version[:maxAppVersionLength-len(prefix)]
}

// License and licenseURL are printed by `nfse versao`.
//
// A binary travels without the repository around it: whoever receives a copy
// has no LICENSE.md next to it, and the FSL asks that a copy of the terms or a
// link to them go along with every copy. One line in `versao` is the only
// place the binary itself can say it.
const (
	License    = "FSL-1.1-MIT"
	licenseURL = "https://github.com/edusouza/nfse-emissor-go/blob/master/LICENSE.md"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "versao",
		Short: "Mostra a versao e a licenca do nfse",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()

			if _, err := fmt.Fprintf(out, "nfse %s (%s/%s, %s)\n",
				Version(), runtime.GOOS, runtime.GOARCH, runtime.Version()); err != nil {
				return err
			}
			_, err := fmt.Fprintf(out,
				"Licenca %s: uso livre, inclusive comercial e interno, exceto\n"+
					"para oferecer produto ou servico concorrente. Vira MIT dois anos\n"+
					"apos a publicacao de cada versao.\n%s\n",
				License, licenseURL)
			return err
		},
	}
}
