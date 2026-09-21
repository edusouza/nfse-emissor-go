// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// envCertPassword is the environment variable consulted for the certificate
// password. Preferred over the --senha flag, which is visible to any process
// that can read the process list.
const envCertPassword = "NFSE_CERT_SENHA"

// errNoPassword is returned when no password source is available.
var errNoPassword = errors.New("senha do certificado nao informada: use --senha, a variavel " +
	envCertPassword + ", ou execute o comando em um terminal interativo")

// resolveCertPassword returns the certificate password from, in order of
// precedence: the --senha flag, the NFSE_CERT_SENHA environment variable, or an
// interactive prompt when stdin is a terminal.
//
// flagSet reports whether --senha was given explicitly, so that an intentionally
// empty password is honoured instead of falling through to the next source.
func resolveCertPassword(flagValue string, flagSet bool, prompt io.Writer) (string, error) {
	if flagSet {
		return flagValue, nil
	}
	if v, ok := os.LookupEnv(envCertPassword); ok {
		return v, nil
	}

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", errNoPassword
	}

	fmt.Fprint(prompt, "Senha do certificado: ")
	secret, err := term.ReadPassword(fd)
	fmt.Fprintln(prompt)
	if err != nil {
		return "", fmt.Errorf("falha ao ler a senha: %w", err)
	}
	return string(secret), nil
}
