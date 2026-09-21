// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

package sefin

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Environment names accepted by Config.
const (
	EnvProduction           = "producao"
	EnvRestrictedProduction = "producao-restrita"
)

// maxResponseSize caps how much of a response body is read. The bodies carry a
// compressed NFS-e, so they are small; an unbounded read of an unexpected
// response is a needless risk.
const maxResponseSize = 8 << 20

// DefaultTimeout is the per-request timeout. Emission is synchronous — the
// government validates and issues the invoice within the request — so it needs
// more room than a plain lookup.
const DefaultTimeout = 60 * time.Second

// Config configures a Client.
type Config struct {
	// Environment is EnvProduction or EnvRestrictedProduction. An empty value
	// means the restricted environment: defaulting to the one that cannot issue
	// a real invoice is the safe way to be wrong.
	Environment string

	// Certificate is the A1 certificate used for mutual TLS. The national
	// system identifies the caller by it; there are no API keys.
	Certificate *tls.Certificate

	// Timeout bounds each request. Zero means DefaultTimeout.
	Timeout time.Duration

	// BaseURL overrides the environment's URL. Used by tests.
	BaseURL string

	// HTTPClient overrides the constructed client. Used by tests.
	HTTPClient *http.Client
}

// Client calls the Sefin Nacional emission service.
type Client struct {
	httpClient *http.Client
	baseURL    string
	env        string
}

// New builds a client for the configured environment.
func New(cfg Config) (*Client, error) {
	env := cfg.Environment
	if env == "" {
		env = EnvRestrictedProduction
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		switch env {
		case EnvProduction:
			baseURL = ProductionBaseURL
		case EnvRestrictedProduction:
			baseURL = RestrictedProductionBaseURL
		default:
			return nil, fmt.Errorf("ambiente %q invalido (use %q ou %q)", env, EnvProduction, EnvRestrictedProduction)
		}
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		if cfg.Certificate == nil {
			return nil, fmt.Errorf("certificado obrigatorio: a Sefin Nacional identifica o emitente por TLS mutuo")
		}

		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = DefaultTimeout
		}

		httpClient = &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					Certificates: []tls.Certificate{*cfg.Certificate},
					MinVersion:   tls.VersionTLS12,

					// The Sefin does not ask for the client certificate in the
					// initial handshake: it renegotiates afterwards to request
					// it. Go refuses renegotiation by default, which surfaces as
					// "local error: tls: no renegotiation" on the first request
					// and makes every emission impossible.
					//
					// Freely rather than once, because a keep-alive connection
					// serves several requests and the server may re-request the
					// certificate on each. Go only accepts renegotiation from a
					// server that advertises RFC 5746 secure renegotiation, so
					// this does not reopen CVE-2009-3555.
					//
					// Ignored when the connection lands on TLS 1.3, where the
					// same need is served by post-handshake authentication.
					Renegotiation: tls.RenegotiateFreelyAsClient,
				},
			},
		}
	}

	return &Client{httpClient: httpClient, baseURL: baseURL, env: env}, nil
}

// Environment reports which environment this client talks to.
func (c *Client) Environment() string { return c.env }

// BaseURL reports the service URL in use, so that a caller can show the user
// exactly where an invoice is going.
func (c *Client) BaseURL() string { return c.baseURL }

// Emit submits a signed DPS and returns the authorised NFS-e.
//
// The call is synchronous: the government validates the declaration and either
// issues the invoice or rejects it within this request. A rejection comes back
// as *RejectionError, which matches errors.Is(err, ErrRejected) and lists every
// reason at once.
func (c *Client) Emit(ctx context.Context, signedDPS []byte) (*EmissionResult, error) {
	encoded, err := encodeGzipBase64(signedDPS)
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(map[string]string{fieldSignedDPS: encoded})
	if err != nil {
		return nil, fmt.Errorf("falha ao montar a requisicao: %w", err)
	}

	body, status, err := c.post(ctx, c.baseURL+pathEmit, payload)
	if err != nil {
		return nil, err
	}

	if status < 200 || status > 299 {
		return nil, c.classify(status, body)
	}

	return parseEmission(body)
}

// post performs a single JSON request and returns the body and status.
//
// It deliberately does not retry. Emission is not idempotent: a request that
// reached the government and issued an invoice, then failed on the way back,
// would produce a second invoice on retry. Deciding what to do belongs to the
// caller, who can look the DPS identifier up.
func (c *Client) post(ctx context.Context, url string, payload []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, fmt.Errorf("falha ao montar a requisicao: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("falha na comunicacao com a Sefin Nacional: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("falha ao ler a resposta da Sefin Nacional: %w", err)
	}

	return body, resp.StatusCode, nil
}

// classify maps an error status onto the sentinel a caller can branch on.
//
// The body is consulted first: the government explains a refusal in its own
// envelope, and that explanation is more useful than the status alone.
func (c *Client) classify(status int, body []byte) error {
	if rejection := asRejection(body); rejection != nil {
		return rejection
	}

	switch status {
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusForbidden, http.StatusUnauthorized:
		// A 403 with no Sefin envelope did not come from the Sefin's business
		// rules: it was refused at the edge. On a mutual-TLS endpoint the usual
		// reason is the client certificate — not an ICP-Brasil A1, expired, or
		// issued to someone who is not a party to the document. A proxy is the
		// second hypothesis, and naming it first sent users hunting through
		// firewall rules for a problem sitting in their certificate.
		if !looksLikeSefinBody(body) {
			return fmt.Errorf("%w (resposta %d sem corpo de erro da Sefin: a conexao foi "+
				"recusada na borda. Confira se o certificado e um A1 valido da ICP-Brasil "+
				"e se pertence a uma parte da nota — um certificado de teste autoassinado "+
				"nao serve para enviar. Se o certificado estiver correto, verifique se um "+
				"proxy ou firewall esta interceptando a conexao)",
				ErrForbidden, status)
		}
		return ErrForbidden
	case http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusGatewayTimeout:
		return ErrUnavailable
	default:
		return parseErrorBody(status, body)
	}
}

// FetchNFSe retrieves an issued invoice by its 50-character access key.
//
// Fiscal secrecy applies: the government only answers for a certificate that
// identifies the provider, the taker or the intermediary named on the invoice.
// Anyone else gets ErrForbidden.
func (c *Client) FetchNFSe(ctx context.Context, accessKey string) (*NFSeResult, error) {
	body, status, err := c.get(ctx, c.baseURL+pathNFSe+url.PathEscape(accessKey))
	if err != nil {
		return nil, err
	}
	if status < 200 || status > 299 {
		return nil, c.classify(status, body)
	}
	return parseNFSe(body)
}

// LookupDPS returns the access key of the invoice generated from a declaration
// identifier, which is how an interrupted emission is reconciled: if the
// government issued the invoice before the connection dropped, the key is here.
func (c *Client) LookupDPS(ctx context.Context, dpsID string) (*DPSLookup, error) {
	body, status, err := c.get(ctx, c.baseURL+pathDPS+url.PathEscape(dpsID))
	if err != nil {
		return nil, err
	}
	if status < 200 || status > 299 {
		return nil, c.classify(status, body)
	}
	return parseDPSLookup(body)
}

// DPSExists reports whether an invoice was issued from a declaration
// identifier, without returning the access key.
//
// The government serves this to any valid certificate, while LookupDPS is
// restricted to the parties on the invoice — so this answers "did my emission
// go through?" even when the caller cannot read the document itself.
func (c *Client) DPSExists(ctx context.Context, dpsID string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, c.baseURL+pathDPS+url.PathEscape(dpsID), nil)
	if err != nil {
		return false, fmt.Errorf("falha ao montar a requisicao: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("falha na comunicacao com a Sefin Nacional: %w", err)
	}
	defer resp.Body.Close()
	// A HEAD response carries no body, but draining keeps the connection reusable.
	_, _ = io.Copy(io.Discard, resp.Body)

	switch {
	case resp.StatusCode == http.StatusOK:
		return true, nil
	case resp.StatusCode == http.StatusNotFound:
		return false, nil
	case resp.StatusCode == http.StatusBadRequest:
		return false, fmt.Errorf("identificador de DPS invalido: %q", dpsID)
	default:
		return false, c.classify(resp.StatusCode, nil)
	}
}

// get performs a single GET and returns the body and status.
func (c *Client) get(ctx context.Context, url string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("falha ao montar a requisicao: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("falha na comunicacao com a Sefin Nacional: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("falha ao ler a resposta da Sefin Nacional: %w", err)
	}
	return body, resp.StatusCode, nil
}

// EventResult is a registered event.
type EventResult struct {
	// EventXML is the event document the government generated, decompressed.
	EventXML []byte

	EnvironmentCode int
	AppVersion      string
	ProcessedAt     time.Time
}

// RegisterEvent submits a signed event request against an issued invoice.
//
// Like emission, this is synchronous and not retried: the government either
// registers the event or refuses it within the request, and a retry after a
// dropped connection could register the same event twice.
func (c *Client) RegisterEvent(ctx context.Context, accessKey string, signedRequest []byte) (*EventResult, error) {
	encoded, err := encodeGzipBase64(signedRequest)
	if err != nil {
		return nil, err
	}

	payload, err := json.Marshal(map[string]string{fieldEventRequest: encoded})
	if err != nil {
		return nil, fmt.Errorf("falha ao montar a requisicao: %w", err)
	}

	url := c.baseURL + pathNFSe + url.PathEscape(accessKey) + pathEvents
	body, status, err := c.post(ctx, url, payload)
	if err != nil {
		return nil, err
	}
	if status < 200 || status > 299 {
		return nil, c.classify(status, body)
	}

	return parseEvent(body)
}
