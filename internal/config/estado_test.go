// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestState_Roundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), StateFileName)

	// A missing file means nothing was emitted here yet, not a failure.
	state, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState em arquivo inexistente falhou: %v", err)
	}
	if got := state.NextNumber("00001"); got != "1" {
		t.Errorf("primeiro numero = %q, esperava 1", got)
	}

	state.Record("00001", "7")
	if err := state.Save(path); err != nil {
		t.Fatal(err)
	}

	reloaded, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.NextNumber("00001"); got != "8" {
		t.Errorf("proximo numero = %q, esperava 8", got)
	}
	// Series are tracked independently: the DPS identifier is built from the
	// series and the number together.
	if got := reloaded.NextNumber("00002"); got != "1" {
		t.Errorf("outra serie = %q, esperava 1", got)
	}
}

// TestState_RecordNeverGoesBackwards pins the rule that keeps an explicitly
// numbered emission from dragging the counter down: the next automatic number
// would then collide with one already issued.
func TestState_RecordNeverGoesBackwards(t *testing.T) {
	state := &State{Series: map[string]SeriesState{}}

	state.Record("00001", "10")
	state.Record("00001", "3")

	if got := state.LastNumber("00001"); got != 10 {
		t.Errorf("ultimo numero = %d, esperava 10", got)
	}
}

func TestState_RecordIgnoresNonNumeric(t *testing.T) {
	state := &State{Series: map[string]SeriesState{}}
	state.Record("00001", "abc")

	if got := state.LastNumber("00001"); got != 0 {
		t.Errorf("ultimo numero = %d, esperava 0", got)
	}
}

// TestState_SetLastNumber covers the migration case, where the counter must be
// forced to continue a numbering started elsewhere — including downwards.
func TestState_SetLastNumber(t *testing.T) {
	state := &State{Series: map[string]SeriesState{}}

	state.Record("00001", "50")
	state.SetLastNumber("00001", 20)

	if got := state.LastNumber("00001"); got != 20 {
		t.Errorf("ultimo numero = %d, esperava 20", got)
	}
}

func TestLoadState_CorruptedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), StateFileName)
	if err := os.WriteFile(path, []byte("{isto nao e json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadState(path)
	if err == nil {
		t.Fatal("esperava erro para arquivo corrompido")
	}
	// The message must say how to recover; a bare JSON error leaves the user
	// with a broken counter and no idea what to do.
	if !strings.Contains(err.Error(), "nfse numero definir") {
		t.Errorf("a mensagem deveria indicar como recuperar: %v", err)
	}
}

// TestState_SaveIsAtomic checks that a write leaves no temporary file behind
// and replaces the previous contents wholesale.
func TestState_SaveIsAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, StateFileName)

	state := &State{Series: map[string]SeriesState{}}
	state.Record("00001", "999999")
	if err := state.Save(path); err != nil {
		t.Fatal(err)
	}
	state.Record("00001", "1000000")
	if err := state.Save(path); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("arquivo temporario deixado para tras: %s", e.Name())
		}
	}

	reloaded, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.LastNumber("00001"); got != 1000000 {
		t.Errorf("ultimo numero = %d, esperava 1000000", got)
	}
}
