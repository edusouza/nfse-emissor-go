// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

package query

import (
	"errors"
	"strings"
	"testing"
)

// The access key format is [0-9]{50}, per TSChaveNFSe in the official XSD and
// the Sefin Nacional specification. An earlier implementation demanded an
// "NFSe" prefix that appears in neither, and so would have rejected every real
// key — these tests pin the published format instead.

func TestValidateAccessKey(t *testing.T) {
	valida := strings.Repeat("1", 50)

	cases := []struct {
		name string
		key  string
		want error
	}{
		{name: "50 digitos", key: valida},
		{name: "com espacos em volta", key: "  " + valida + "  "},
		{name: "vazia", key: "", want: ErrAccessKeyEmpty},
		{name: "so espacos", key: "   ", want: ErrAccessKeyEmpty},
		{name: "curta demais", key: strings.Repeat("1", 49), want: ErrAccessKeyInvalidLength},
		{name: "longa demais", key: strings.Repeat("1", 51), want: ErrAccessKeyInvalidLength},
		{
			name: "com prefixo NFSe",
			key:  "NFSe" + strings.Repeat("1", 46),
			want: ErrAccessKeyInvalidCharacters,
		},
		{
			name: "com letra no meio",
			key:  strings.Repeat("1", 25) + "A" + strings.Repeat("1", 24),
			want: ErrAccessKeyInvalidCharacters,
		},
		{
			name: "com hifen",
			key:  strings.Repeat("1", 25) + "-" + strings.Repeat("1", 24),
			want: ErrAccessKeyInvalidCharacters,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateAccessKey(tc.key)

			if tc.want == nil {
				if err != nil {
					t.Fatalf("esperava chave valida, obtive: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("erro = %v, esperava %v", err, tc.want)
			}
		})
	}
}

func TestIsValidAccessKey(t *testing.T) {
	if !IsValidAccessKey(strings.Repeat("9", 50)) {
		t.Error("50 digitos deveria ser valido")
	}
	if IsValidAccessKey("NFSe" + strings.Repeat("9", 46)) {
		t.Error("o prefixo NFSe nao faz parte do formato")
	}
}

func TestNormalizeAccessKey(t *testing.T) {
	valida := strings.Repeat("7", 50)

	got, err := NormalizeAccessKey("\t " + valida + "\n")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got != valida {
		t.Errorf("chave = %q, esperava %q", got, valida)
	}

	if _, err := NormalizeAccessKey("123"); err == nil {
		t.Error("esperava erro para chave invalida")
	}
}
