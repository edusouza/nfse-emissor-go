package sefin

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// serveFlaky starts a stub Sefin that cannot be reached for the first failures
// connection attempts, the way an outage looks from the caller's side: the
// handshake never completes, so no request is ever written.
//
// It returns the client and a counter of connection attempts. Retry delays are
// zeroed so the tests do not wait on the clock.
func serveFlaky(t *testing.T, failures int, handler http.HandlerFunc) (*Client, *atomic.Int32) {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	dials := new(atomic.Int32)
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if int(dials.Add(1)) <= failures {
				// The shape net.Dialer returns when a SYN goes unanswered.
				return nil, &net.OpError{Op: "dial", Net: network, Err: errors.New("connectex: the host did not respond")}
			}
			var d net.Dialer
			return d.DialContext(ctx, network, addr)
		},
	}
	t.Cleanup(transport.CloseIdleConnections)

	client, err := New(Config{BaseURL: srv.URL, HTTPClient: &http.Client{Transport: transport}})
	if err != nil {
		t.Fatal(err)
	}
	client.retryDelays = make([]time.Duration, len(client.retryDelays))
	return client, dials
}

func lookupHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]any{
		"tipoAmbiente":          EnvCodeRestrictedProduction,
		"versaoAplicativo":      "1.0.0",
		"dataHoraProcessamento": "2026-09-18T09:57:36-03:00",
		"chaveAcesso":           strings.Repeat("4", 50),
	})
}

func TestLookupDPS_RetriesWhenTheConnectionCannotBeOpened(t *testing.T) {
	client, dials := serveFlaky(t, 2, lookupHandler)

	result, err := client.LookupDPS(context.Background(), "DPS123")
	if err != nil {
		t.Fatalf("LookupDPS falhou: %v", err)
	}
	if result.AccessKey != strings.Repeat("4", 50) {
		t.Errorf("chave = %q", result.AccessKey)
	}
	if got := dials.Load(); got != 3 {
		t.Errorf("tentativas de conexao = %d, esperava 3", got)
	}
}

func TestLookupDPS_GivesUpAfterThreeAttempts(t *testing.T) {
	client, dials := serveFlaky(t, 99, lookupHandler)

	_, err := client.LookupDPS(context.Background(), "DPS123")
	if !errors.Is(err, ErrUnreachable) {
		t.Fatalf("erro = %v, esperava ErrUnreachable", err)
	}
	if got := dials.Load(); got != 3 {
		t.Errorf("tentativas de conexao = %d, esperava 3", got)
	}
	if !strings.Contains(err.Error(), "3 tentativas") {
		t.Errorf("a mensagem nao diz quantas vezes tentou: %v", err)
	}
}

func TestDPSExists_RetriesWhenTheConnectionCannotBeOpened(t *testing.T) {
	client, dials := serveFlaky(t, 1, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	exists, err := client.DPSExists(context.Background(), "DPS123")
	if err != nil {
		t.Fatalf("DPSExists falhou: %v", err)
	}
	if !exists {
		t.Error("esperava que a DPS existisse")
	}
	if got := dials.Load(); got != 2 {
		t.Errorf("tentativas de conexao = %d, esperava 2", got)
	}
}

// TestLookupDPS_DoesNotRetryOnceTheServerAnswered keeps the retry narrow: a
// server that answered, even with an error, is not an outage of the network.
func TestLookupDPS_DoesNotRetryOnceTheServerAnswered(t *testing.T) {
	var calls atomic.Int32
	client, _ := serveFlaky(t, 0, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	_, err := client.LookupDPS(context.Background(), "DPS123")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("erro = %v, esperava ErrUnavailable", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("a Sefin foi chamada %d vezes, esperava 1", got)
	}
}

// TestEmit_DoesNotRetryWhenTheConnectionCannotBeOpened keeps emission out of
// the retry, even though a refused connection sent nothing: repeating is the
// user's decision, and the message tells them how.
func TestEmit_DoesNotRetryWhenTheConnectionCannotBeOpened(t *testing.T) {
	client, dials := serveFlaky(t, 99, func(w http.ResponseWriter, r *http.Request) {
		t.Error("a requisicao nao deveria ter chegado ao servidor")
	})

	_, err := client.Emit(context.Background(), []byte(testSignedDPS))
	if !errors.Is(err, ErrUnreachable) {
		t.Fatalf("erro = %v, esperava ErrUnreachable", err)
	}
	if got := dials.Load(); got != 1 {
		t.Errorf("tentativas de conexao = %d; emissao nao pode ser repetida automaticamente", got)
	}
	if !strings.Contains(err.Error(), "nao chegou ao servidor") {
		t.Errorf("a mensagem nao diz que nada foi enviado: %v", err)
	}
}

// TestEmit_RefusedConnectionIsUnreachable uses a real refused connection rather
// than a simulated one, to pin that the detection matches what net/http
// actually returns.
func TestEmit_RefusedConnectionIsUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	baseURL := srv.URL
	srv.Close()

	client, err := New(Config{BaseURL: baseURL, HTTPClient: &http.Client{}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Emit(context.Background(), []byte(testSignedDPS))
	if !errors.Is(err, ErrUnreachable) {
		t.Fatalf("erro = %v, esperava ErrUnreachable", err)
	}
}

// TestEmit_ConnectionDroppedAfterSendingIsNotUnreachable guards the claim the
// unreachable message makes. Once the request is written, the government may
// have issued the invoice, and telling the user nothing happened would invite
// a duplicate.
func TestEmit_ConnectionDroppedAfterSendingIsNotUnreachable(t *testing.T) {
	client, _ := serveFlaky(t, 0, func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Fatal(err)
		}
		conn.Close()
	})

	_, err := client.Emit(context.Background(), []byte(testSignedDPS))
	if err == nil {
		t.Fatal("esperava erro")
	}
	if errors.Is(err, ErrUnreachable) {
		t.Errorf("a conexao caiu depois do envio, mas o erro diz que nada chegou: %v", err)
	}
}

// Retries are announced, so that the pause between them does not look like a
// hang. The callback sees each pause, once per retry — never on the last
// failure, when there is nothing left to wait for.
func TestLookupDPS_AnnouncesEachRetry(t *testing.T) {
	client, _ := serveFlaky(t, 99, lookupHandler)

	var waits []time.Duration
	client.onRetry = func(wait time.Duration, err error) {
		if !isDialError(err) {
			t.Errorf("OnRetry recebeu um erro que nao e de conexao: %v", err)
		}
		waits = append(waits, wait)
	}

	if _, err := client.LookupDPS(context.Background(), "DPS123"); !errors.Is(err, ErrUnreachable) {
		t.Fatalf("erro = %v, esperava ErrUnreachable", err)
	}
	if len(waits) != len(client.retryDelays) {
		t.Errorf("OnRetry chamado %d vezes, esperava %d", len(waits), len(client.retryDelays))
	}
}
