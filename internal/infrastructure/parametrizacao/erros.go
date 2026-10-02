package parametrizacao

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// Sentinel errors for the conditions a caller branches on. Every answer other
// than a success comes back as *RespostaError, which matches one of them with
// errors.Is.
var (
	// ErrNaoEncontrado is a 404: the municipality has nothing parameterised
	// for what was asked, or the service does not know it.
	ErrNaoEncontrado = errors.New("o ADN nao tem esse parametro")

	// ErrConsultaInvalida is a 400: the service could not make sense of the
	// question, which on this client means a value it formats is wrong.
	ErrConsultaInvalida = errors.New("o ADN recusou a consulta")

	// ErrAcessoNegado is a 401 or 403: the certificate was not accepted.
	ErrAcessoNegado = errors.New("o ADN recusou o certificado")

	// ErrIndisponivel is a 502, 503 or 504: the service is down for now.
	ErrIndisponivel = errors.New("o ADN esta temporariamente indisponivel")

	// ErrRespostaInesperada is any other status.
	ErrRespostaInesperada = errors.New("resposta inesperada do ADN")

	// ErrInacessivel means no answer came back at all: no route, a refused
	// connection, a handshake that failed, a timeout.
	ErrInacessivel = errors.New("nao foi possivel consultar o ADN")
)

// RespostaError is an answer the service gave, other than a success.
type RespostaError struct {
	Status int

	// Mensagem is the service's explanation, when it gave one in either of its
	// two error formats.
	Mensagem string

	causa error
}

func (e *RespostaError) Error() string {
	if e.Mensagem == "" {
		return fmt.Sprintf("%s (HTTP %d)", e.causa, e.Status)
	}
	return fmt.Sprintf("%s (HTTP %d): %s", e.causa, e.Status, e.Mensagem)
}

// Unwrap lets callers match the sentinel with errors.Is.
func (e *RespostaError) Unwrap() error { return e.causa }

// novoRespostaError classifies a status and pulls the explanation out of the
// body, whichever of the two formats it came in.
func novoRespostaError(status int, body []byte) *RespostaError {
	e := &RespostaError{Status: status, Mensagem: explicacao(body)}

	switch {
	case status == http.StatusNotFound:
		e.causa = ErrNaoEncontrado
	case status == http.StatusBadRequest:
		e.causa = ErrConsultaInvalida
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		e.causa = ErrAcessoNegado
	case status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout:
		e.causa = ErrIndisponivel
	default:
		e.causa = ErrRespostaInesperada
	}
	return e
}

// explicacao reads the service's envelope ({"mensagem": ...}) or, failing
// that, ASP.NET's validation problem ({"title": ..., "errors": {...}}).
func explicacao(body []byte) string {
	if r, ok := decodificar[envelope](body); ok && strings.TrimSpace(r.Mensagem) != "" {
		return strings.TrimSpace(r.Mensagem)
	}

	p, ok := decodificar[problema](body)
	if !ok || (p.Title == "" && len(p.Errors) == 0) {
		return ""
	}

	// Sorted, so that the same refusal always reads the same way.
	campos := make([]string, 0, len(p.Errors))
	for campo := range p.Errors {
		campos = append(campos, campo)
	}
	sort.Strings(campos)

	partes := []string{}
	if p.Title != "" {
		partes = append(partes, p.Title)
	}
	for _, campo := range campos {
		partes = append(partes, campo+": "+strings.Join(p.Errors[campo], " "))
	}
	return strings.Join(partes, "; ")
}
