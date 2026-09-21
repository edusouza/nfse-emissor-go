// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

// Command nfse is a command-line emitter for Brazilian NFS-e (Nota Fiscal de
// Servico eletronica) through the Sistema Nacional NFS-e.
package main

import (
	"fmt"
	"os"

	"github.com/edusouza/nfse-emissor-go/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "erro: %v\n", err)
		os.Exit(1)
	}
}
