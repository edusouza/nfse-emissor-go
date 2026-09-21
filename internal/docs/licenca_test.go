// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

package docs

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/edusouza/nfse-emissor-go/internal/cli"
)

// spdxLine matches the identifier in a source file header and in LICENSE.md.
var spdxLine = regexp.MustCompile(`SPDX-License-Identifier:\s*(\S+)`)

// licenseFile is the notice that travels with every copy of the software.
const licenseFile = "LICENSE.md"

// TestTodoFonteTemCabecalhoDeLicenca checks that every Go file carries the
// notice.
//
// A file added later without it would ship with no statement of terms at all,
// and nothing else in the build would complain: the code compiles, the tests
// pass, and the omission only shows up when someone downstream asks under
// which license they got that file.
func TestTodoFonteTemCabecalhoDeLicenca(t *testing.T) {
	root := repoRoot(t)
	identificador := identificadorDaLicenca(t, root)

	var semCabecalho []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}

		conteudo, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Only the top of the file counts. The identifier appearing somewhere
		// in the middle — in a test fixture, say — is not a header.
		linhas := strings.SplitN(string(conteudo), "\n", 6)
		topo := strings.Join(linhas[:min(len(linhas), 5)], "\n")

		achado := spdxLine.FindStringSubmatch(topo)
		relativo, _ := filepath.Rel(root, path)

		switch {
		case achado == nil:
			semCabecalho = append(semCabecalho, relativo+" (sem cabecalho)")
		case achado[1] != identificador:
			semCabecalho = append(semCabecalho,
				relativo+" declara "+achado[1]+", mas "+licenseFile+" e "+identificador)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(semCabecalho) > 0 {
		t.Errorf("%d arquivo(s) fora da regra de licenca:\n  %s\n\n"+
			"Todo .go comeca com:\n  // Copyright <ano> <titular>\n  // SPDX-License-Identifier: %s",
			len(semCabecalho), strings.Join(semCabecalho, "\n  "), identificador)
	}
}

// TestBinarioDeclaraALicencaDoArquivo keeps `nfse versao` honest: it is the
// only place a distributed binary states its terms, and a relicense that
// forgot it would have every copy announcing the old license.
func TestBinarioDeclaraALicencaDoArquivo(t *testing.T) {
	if got := cli.License; got != identificadorDaLicenca(t, repoRoot(t)) {
		t.Errorf("`nfse versao` anuncia %q, diferente do que esta em %s", got, licenseFile)
	}
}

// identificadorDaLicenca reads the identifier out of LICENSE.md, so that the
// headers are checked against the licence itself rather than against a string
// repeated in the test.
func identificadorDaLicenca(t *testing.T, root string) string {
	t.Helper()

	conteudo, err := os.ReadFile(filepath.Join(root, licenseFile))
	if err != nil {
		t.Fatalf("%s nao foi encontrado: %v", licenseFile, err)
	}

	achado := spdxLine.FindStringSubmatch(string(conteudo))
	if achado == nil {
		// The FSL states its abbreviation under an "Abbreviation" heading
		// rather than as an SPDX line.
		if abrev := regexp.MustCompile(`(?m)^##\s*Abbreviation\s*$\s+(\S+)`).
			FindStringSubmatch(string(conteudo)); abrev != nil {
			return abrev[1]
		}
		t.Fatalf("%s nao declara um identificador de licenca", licenseFile)
	}
	return achado[1]
}
