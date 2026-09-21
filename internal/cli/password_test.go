// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

package cli

import (
	"errors"
	"io"
	"testing"
)

func TestResolveCertPassword(t *testing.T) {
	t.Run("a flag tem precedencia sobre o ambiente", func(t *testing.T) {
		t.Setenv(envCertPassword, "do-ambiente")

		got, err := resolveCertPassword("da-flag", true, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		if got != "da-flag" {
			t.Errorf("senha = %q, esperava %q", got, "da-flag")
		}
	})

	t.Run("flag vazia explicita e respeitada", func(t *testing.T) {
		t.Setenv(envCertPassword, "do-ambiente")

		// A certificate with no password is unusual but legal; an explicit
		// --senha="" must not silently fall through to the environment.
		got, err := resolveCertPassword("", true, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		if got != "" {
			t.Errorf("senha = %q, esperava vazia", got)
		}
	})

	t.Run("cai para o ambiente quando a flag nao e usada", func(t *testing.T) {
		t.Setenv(envCertPassword, "do-ambiente")

		got, err := resolveCertPassword("", false, io.Discard)
		if err != nil {
			t.Fatal(err)
		}
		if got != "do-ambiente" {
			t.Errorf("senha = %q, esperava %q", got, "do-ambiente")
		}
	})

	t.Run("erro util quando nao ha fonte de senha", func(t *testing.T) {
		// Under `go test` stdin is not a terminal, so the prompt path is skipped.
		if _, err := resolveCertPassword("", false, io.Discard); !errors.Is(err, errNoPassword) {
			t.Fatalf("erro = %v, esperava errNoPassword", err)
		}
	})
}
