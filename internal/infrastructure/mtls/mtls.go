// Package mtls builds the HTTP client the national system's services require:
// mutual TLS with the caller's A1 certificate, which is how they identify
// whoever is calling. There are no API keys.
//
// The Sefin Nacional and the ADN sit behind the same infrastructure and refuse
// connections the same way, so the handshake settings live here once rather
// than drifting apart in each client.
package mtls

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"time"
)

// DialTimeout bounds opening the connection, apart from the request.
//
// A request timeout can be generous — an emission is processed inside the
// request — but opening a connection is not where that time goes. Left to the
// operating system, a host that never answers the handshake takes ~21s on
// Windows and up to two minutes on Linux, where the SYN is retried, and that is
// per attempt. Ten seconds is ample for a reachable server.
const DialTimeout = 10 * time.Second

// NewHTTPClient returns a client that presents cert on every connection.
func NewHTTPClient(cert *tls.Certificate, timeout time.Duration) (*http.Client, error) {
	if cert == nil {
		return nil, errors.New("certificado obrigatorio: o Sistema Nacional identifica quem consulta por TLS mutuo")
	}

	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: DialTimeout, KeepAlive: 30 * time.Second}).DialContext,
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{*cert},
				MinVersion:   tls.VersionTLS12,

				// The Sefin does not ask for the client certificate in the
				// initial handshake: it renegotiates afterwards to request it.
				// Go refuses renegotiation by default, which surfaces as "local
				// error: tls: no renegotiation" on the first request and made
				// every emission impossible.
				//
				// Freely rather than once, because a keep-alive connection
				// serves several requests and the server may re-request the
				// certificate on each. Go only accepts renegotiation from a
				// server that advertises RFC 5746 secure renegotiation, so this
				// does not reopen CVE-2009-3555.
				//
				// Ignored when the connection lands on TLS 1.3, where the same
				// need is served by post-handshake authentication.
				Renegotiation: tls.RenegotiateFreelyAsClient,
			},
		},
	}, nil
}

// IsDialError reports whether err happened while opening the connection:
// resolving the name or completing the TCP handshake. It is the one transport
// failure that proves no request reached the server.
func IsDialError(err error) bool {
	var opErr *net.OpError
	return errors.As(err, &opErr) && opErr.Op == "dial"
}
