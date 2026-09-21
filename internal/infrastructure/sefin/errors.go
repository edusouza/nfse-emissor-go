// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

package sefin

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors for the conditions a caller is likely to branch on.
var (
	// ErrRejected means the government validated the document and refused it.
	// The rejection reasons are carried by RejectionError.
	ErrRejected = errors.New("documento rejeitado pela Sefin Nacional")

	// ErrNotFound means the requested document does not exist.
	ErrNotFound = errors.New("documento nao encontrado")

	// ErrForbidden means the certificate does not identify a party to the
	// document. Fiscal secrecy rules restrict lookups to the provider, the
	// taker and the intermediary named on the invoice.
	ErrForbidden = errors.New("acesso negado: o certificado nao identifica uma parte do documento")

	// ErrUnavailable means the government service is temporarily down.
	ErrUnavailable = errors.New("servico da Sefin Nacional temporariamente indisponivel")
)

// Message is one entry of the erros or alertas arrays.
//
// It is the union of the two MensagemProcessamento definitions the national
// system publishes: the Sefin Nacional specification declares codigo,
// descricao and complemento, while the ADN one adds mensagem and parametros.
// Go matches JSON field names case-insensitively, so this one struct reads
// either service regardless of how each capitalises them.
type Message struct {
	Codigo      string   `json:"codigo"`
	Descricao   string   `json:"descricao"`
	Complemento string   `json:"complemento"`
	Mensagem    string   `json:"mensagem"`
	Parametros  []string `json:"parametros"`
}

// String renders a message the way a person needs to read it: the code, what it
// means, and whatever detail points at the offending field.
func (m Message) String() string {
	var b strings.Builder

	if m.Codigo != "" {
		fmt.Fprintf(&b, "[%s] ", m.Codigo)
	}

	text := m.Descricao
	if text == "" {
		text = m.Mensagem
	}
	b.WriteString(text)

	if m.Complemento != "" {
		fmt.Fprintf(&b, " (%s)", m.Complemento)
	}

	return b.String()
}

// RejectionError carries every reason the government gave for refusing a
// document. All of them are reported at once: fixing one at a time, with a
// network round-trip between each, is the slow way to get an invoice out.
type RejectionError struct {
	Rejections []Message
	Warnings   []Message
}

func (e *RejectionError) Error() string {
	var b strings.Builder
	b.WriteString(ErrRejected.Error())
	for _, r := range e.Rejections {
		fmt.Fprintf(&b, "\n  - %s", r)
	}
	return b.String()
}

// Unwrap lets callers match with errors.Is(err, ErrRejected).
func (e *RejectionError) Unwrap() error { return ErrRejected }

// Codes returns just the rejection codes, for programmatic handling.
func (e *RejectionError) Codes() []string {
	codes := make([]string, 0, len(e.Rejections))
	for _, r := range e.Rejections {
		if r.Codigo != "" {
			codes = append(codes, r.Codigo)
		}
	}
	return codes
}

// HTTPError is returned when the service answers with an unexpected status and
// no recognisable error envelope.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	body := strings.TrimSpace(e.Body)
	if len(body) > 300 {
		body = body[:300] + "..."
	}
	if body == "" {
		return fmt.Sprintf("resposta HTTP inesperada: %d", e.StatusCode)
	}
	return fmt.Sprintf("resposta HTTP inesperada: %d: %s", e.StatusCode, body)
}
