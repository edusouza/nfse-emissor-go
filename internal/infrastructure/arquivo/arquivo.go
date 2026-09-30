// Package arquivo holds what the emitter's on-disk caches share: where they
// live and how they are written.
package arquivo

import (
	"os"
	"path/filepath"
)

// DiretorioCache returns the directory the emitter's caches belong in,
// following whatever convention the operating system has for caches.
func DiretorioCache() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "nfse"), nil
}

// GravarAtomico writes through a temporary file in the same directory and
// renames it over the destination, creating the directory if needed.
//
// Writing in place truncates first: an interrupted run leaves half a JSON
// document, which the reader then throws away whole, and two runs at once
// interleave their bytes. A rename is atomic on the same filesystem, so a
// reader sees the old file or the new one, never a mix. Two concurrent runs
// still race — the last one wins — but each leaves a file that parses.
//
// The caches hold public data, so the file is left readable by others, as
// they always were (os.CreateTemp alone would make it 0600).
func GravarAtomico(caminho string, conteudo []byte) error {
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		return err
	}

	temporario, err := os.CreateTemp(filepath.Dir(caminho), filepath.Base(caminho)+".*.tmp")
	if err != nil {
		return err
	}
	nome := temporario.Name()

	if _, err := temporario.Write(conteudo); err != nil {
		temporario.Close()
		os.Remove(nome)
		return err
	}
	if err := temporario.Close(); err != nil {
		os.Remove(nome)
		return err
	}
	if err := os.Chmod(nome, 0o644); err != nil {
		os.Remove(nome)
		return err
	}
	if err := os.Rename(nome, caminho); err != nil {
		os.Remove(nome)
		return err
	}
	return nil
}
