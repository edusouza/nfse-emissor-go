package mtls

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

// The Sefin requests the client certificate through TLS renegotiation, and Go
// refuses it unless asked; the default cost the first real emission a "local
// error: tls: no renegotiation". Go's own TLS server cannot renegotiate, so no
// httptest server reproduces it: the setting is asserted so that it is not
// tidied away by someone who has never seen that error.
func TestNewHTTPClient(t *testing.T) {
	cert := &tls.Certificate{}

	client, err := NewHTTPClient(cert, 42*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if client.Timeout != 42*time.Second {
		t.Errorf("Timeout = %s, esperava 42s", client.Timeout)
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transporte = %T, esperava *http.Transport", client.Transport)
	}
	config := transport.TLSClientConfig
	if got := config.Renegotiation; got != tls.RenegotiateFreelyAsClient {
		t.Errorf("Renegotiation = %v, esperava RenegotiateFreelyAsClient", got)
	}
	if got := config.MinVersion; got != tls.VersionTLS12 {
		t.Errorf("MinVersion = %v, esperava TLS 1.2", got)
	}
	if len(config.Certificates) != 1 {
		t.Errorf("%d certificados no transporte, esperava 1", len(config.Certificates))
	}
}

func TestNewHTTPClientSemCertificado(t *testing.T) {
	if _, err := NewHTTPClient(nil, time.Second); err == nil {
		t.Fatal("esperava erro sem certificado")
	}
}

func TestIsDialError(t *testing.T) {
	dial := &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}
	read := &net.OpError{Op: "read", Net: "tcp", Err: errors.New("connection reset")}

	casos := map[string]struct {
		err  error
		quer bool
	}{
		"dial":            {dial, true},
		"dial embrulhado": {fmt.Errorf("Get: %w", dial), true},
		"leitura":         {read, false},
		"outro":           {errors.New("tls: handshake failure"), false},
		"nil":             {nil, false},
	}
	for nome, c := range casos {
		if got := IsDialError(c.err); got != c.quer {
			t.Errorf("%s: IsDialError = %v, esperava %v", nome, got, c.quer)
		}
	}
}
