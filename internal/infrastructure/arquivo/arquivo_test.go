package arquivo

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGravarAtomico(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), "sub", "dir", "cache.json")

	if err := GravarAtomico(caminho, []byte("primeiro")); err != nil {
		t.Fatalf("o diretorio deveria ser criado: %v", err)
	}
	if err := GravarAtomico(caminho, []byte("segundo")); err != nil {
		t.Fatal(err)
	}

	conteudo, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	if string(conteudo) != "segundo" {
		t.Errorf("conteudo = %q, esperava o da ultima gravacao", conteudo)
	}

	// Nothing but the file itself is left behind.
	entradas, err := os.ReadDir(filepath.Dir(caminho))
	if err != nil {
		t.Fatal(err)
	}
	if len(entradas) != 1 {
		t.Errorf("sobraram arquivos temporarios: %v", entradas)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(caminho)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o644 {
			t.Errorf("permissao = %o, esperava 644", perm)
		}
	}
}

func TestGravarAtomicoFalhaSemDeixarRastro(t *testing.T) {
	dir := t.TempDir()
	// A directory where the file should be makes the rename fail.
	caminho := filepath.Join(dir, "cache.json")
	if err := os.Mkdir(caminho, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caminho, "dentro"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := GravarAtomico(caminho, []byte("x")); err == nil {
		t.Fatal("esperava erro ao renomear sobre um diretorio")
	}
	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entradas) != 1 {
		t.Errorf("o temporario ficou para tras: %v", entradas)
	}
}

func TestDiretorioCache(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if runtime.GOOS != "linux" {
		t.Skip("XDG_CACHE_HOME so vale no Linux")
	}

	dir, err := DiretorioCache()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(dir) != "nfse" || filepath.Dir(dir) != os.Getenv("XDG_CACHE_HOME") {
		t.Errorf("diretorio = %q, esperava $XDG_CACHE_HOME/nfse", dir)
	}
}
