// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// StateFileName is the file that remembers which DPS numbers have been used.
// It sits next to the configuration it belongs to.
const StateFileName = ".nfse-estado.json"

// State is the local record of emission progress.
//
// It is a convenience, not a source of truth: the government decides what was
// really issued. Emitting from two machines with the same configuration will
// drift, and the fix is to consult the Sefin, not to trust this file.
type State struct {
	// Series maps a DPS series to its progress. Keeping it per series matters
	// because the DPS identifier is built from series and number together.
	Series map[string]SeriesState `json:"series"`
}

// SeriesState is the progress of one series.
type SeriesState struct {
	// LastNumber is the highest DPS number used so far.
	LastNumber uint64 `json:"ultimo_numero"`

	// UpdatedAt is when that number was recorded.
	UpdatedAt time.Time `json:"atualizado_em"`
}

// StatePath returns the state file that accompanies a configuration file.
func StatePath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), StateFileName)
}

// LoadState reads the state file. A missing file is not an error: it simply
// means nothing has been emitted from this machine yet.
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &State{Series: map[string]SeriesState{}}, nil
		}
		return nil, fmt.Errorf("nao foi possivel ler %q: %w", path, err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("arquivo de estado %q esta corrompido: %w\n"+
			"apague-o para recomecar a contagem, ou corrija o numero com 'nfse numero definir'", path, err)
	}
	if state.Series == nil {
		state.Series = map[string]SeriesState{}
	}
	return &state, nil
}

// Save writes the state file.
func (s *State) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("falha ao serializar o estado: %w", err)
	}

	// Write to a temporary file and rename, so an interrupted write cannot
	// leave a truncated file that loses the counter entirely.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("nao foi possivel gravar %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("nao foi possivel gravar %q: %w", path, err)
	}
	return nil
}

// LastNumber returns the highest number recorded for a series.
func (s *State) LastNumber(series string) uint64 {
	return s.Series[series].LastNumber
}

// NextNumber returns the number that follows the last one recorded.
func (s *State) NextNumber(series string) string {
	return strconv.FormatUint(s.LastNumber(series)+1, 10)
}

// Record marks a number as used, keeping the highest seen.
//
// It never moves the counter backwards: emitting an explicit lower number —
// filling a gap, say — must not make the next automatic number collide with
// one already issued.
func (s *State) Record(series, number string) {
	n, err := strconv.ParseUint(number, 10, 64)
	if err != nil {
		// A non-numeric number cannot advance a counter; the schema requires
		// digits, and validation upstream rejects anything else.
		return
	}
	if s.Series == nil {
		s.Series = map[string]SeriesState{}
	}
	if n <= s.Series[series].LastNumber {
		return
	}
	s.Series[series] = SeriesState{LastNumber: n, UpdatedAt: time.Now()}
}

// SetLastNumber forces the counter for a series, for someone migrating from
// another emitter who must continue an existing numbering.
func (s *State) SetLastNumber(series string, number uint64) {
	if s.Series == nil {
		s.Series = map[string]SeriesState{}
	}
	s.Series[series] = SeriesState{LastNumber: number, UpdatedAt: time.Now()}
}
