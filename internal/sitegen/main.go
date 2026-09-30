// Command sitegen writes the pages of the documentation site that are derived
// from the repository rather than written for it.
//
//	go run ./internal/sitegen [-raiz .] [-saida site/conteudo]
//
// Three kinds of page come out of it: the rejection codes, read from the
// business-rules annex the government publishes; the architecture decision
// records; and the changelog. None of them is committed. CI runs this before
// every build of the site, so a page cannot disagree with the file it came
// from — which is what a copy kept by hand eventually does (ADR 0016).
package main

import (
	"flag"
	"log"
	"path/filepath"
)

func main() {
	root := flag.String("raiz", ".", "raiz do repositorio")
	out := flag.String("saida", filepath.Join("site", "conteudo"), "diretorio de conteudo do site")
	flag.Parse()

	if err := run(*root, *out); err != nil {
		log.Fatal(err)
	}
}

func run(root, out string) error {
	if err := writeRejections(root, out); err != nil {
		return err
	}
	if err := writeDecisions(root, out); err != nil {
		return err
	}
	return writeChangelog(root, out)
}
