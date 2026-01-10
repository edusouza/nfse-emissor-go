// Package sefin provides client implementations for communicating with
// the SEFIN (Secretaria da Fazenda) government API for the Sistema Nacional NFS-e.
package sefin

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ================================================================================
// ProductionClient QueryNFSe Tests
// ================================================================================

func TestQueryNFSe_Success(t *testing.T) {
	// Create test server that returns a successful response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		if r.Method != http.MethodGet {
			t.Errorf("expected GET method, got %s", r.Method)
		}

		// Verify URL path contains /nfse/
		if !strings.Contains(r.URL.Path, "/nfse/") {
			t.Errorf("expected path to contain /nfse/, got %s", r.URL.Path)
		}

		// Verify Accept header
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("expected Accept: application/json, got %s", r.Header.Get("Accept"))
		}

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"chaveAcesso": "NFSe35012312345678000199990000100000000001234567890",
			"numero":      "000000001",
			"dataEmissao": "2024-01-15T10:30:00-03:00",
			"status":      "active",
			"xml":         "<NFSe>...</NFSe>",
			"prestador": map[string]string{
				"documento":       "12345678000199",
				"nome":            "Empresa Teste Ltda",
				"municipio":       "Sao Paulo",
				"municipioCodigo": "3550308",
			},
			"tomador": map[string]string{
				"documento":     "98765432000188",
				"tipoDocumento": "cnpj",
				"nome":          "Cliente Teste S.A.",
			},
			"servico": map[string]string{
				"codigoNacional":  "010201",
				"descricao":       "Consultoria em TI",
				"localPrestacao":  "Sao Paulo - SP",
				"municipioCodigo": "3550308",
			},
			"valores": map[string]float64{
				"valorServico": 1000.00,
				"baseCalculo":  1000.00,
				"aliquota":     5.00,
				"valorISSQN":   50.00,
				"valorLiquido": 950.00,
			},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	// Create client with test server URL
	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Execute query
	result, err := client.QueryNFSe(context.Background(), "NFSe35012312345678000199990000100000000001234567890", nil)
	if err != nil {
		t.Fatalf("QueryNFSe failed: %v", err)
	}

	// Verify result
	if result.ChaveAcesso != "NFSe35012312345678000199990000100000000001234567890" {
		t.Errorf("expected ChaveAcesso NFSe35012312345678000199990000100000000001234567890, got %s", result.ChaveAcesso)
	}
	if result.Numero != "000000001" {
		t.Errorf("expected Numero 000000001, got %s", result.Numero)
	}
	if result.Status != "active" {
		t.Errorf("expected Status active, got %s", result.Status)
	}
	if result.Prestador.Documento != "12345678000199" {
		t.Errorf("expected Prestador.Documento 12345678000199, got %s", result.Prestador.Documento)
	}
	if result.Tomador == nil {
		t.Error("expected Tomador to be present")
	} else if result.Tomador.Documento != "98765432000188" {
		t.Errorf("expected Tomador.Documento 98765432000188, got %s", result.Tomador.Documento)
	}
	if result.Valores.ValorServico != 1000.00 {
		t.Errorf("expected ValorServico 1000.00, got %f", result.Valores.ValorServico)
	}
}

func TestQueryNFSe_NotFound(t *testing.T) {
	// Create test server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.QueryNFSe(context.Background(), "NFSeNOTEXISTS", nil)
	if !errors.Is(err, ErrNFSeNotFound) {
		t.Errorf("expected ErrNFSeNotFound, got %v", err)
	}
}

func TestQueryNFSe_ServiceUnavailable(t *testing.T) {
	// Create test server that returns 503
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.QueryNFSe(context.Background(), "NFSe12345", nil)
	if !errors.Is(err, ErrServiceUnavailable) {
		t.Errorf("expected ErrServiceUnavailable, got %v", err)
	}
}

func TestQueryNFSe_Timeout(t *testing.T) {
	// Create test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second) // Longer than client timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     500 * time.Millisecond, // Short timeout
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err = client.QueryNFSe(ctx, "NFSe12345", nil)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
	// Should be ErrTimeout or context.DeadlineExceeded
	if !errors.Is(err, ErrTimeout) && !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "timeout") {
		t.Errorf("expected timeout error, got %v", err)
	}
}

func TestQueryNFSe_ParsesJSONCorrectly(t *testing.T) {
	// Create test server with complete JSON response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"chaveAcesso": "NFSe35012412345678000199990000100000000001234567890",
			"numero":      "000000123",
			"dataEmissao": "2024-01-20T14:30:00-03:00",
			"status":      "cancelled",
			"xml":         "<NFSe><cancelled>true</cancelled></NFSe>",
			"prestador": map[string]string{
				"documento":       "11111111000111",
				"nome":            "Prestador ABC",
				"municipio":       "Rio de Janeiro",
				"municipioCodigo": "3304557",
			},
			"servico": map[string]string{
				"codigoNacional":  "020101",
				"descricao":       "Desenvolvimento de software",
				"localPrestacao":  "Rio de Janeiro - RJ",
				"municipioCodigo": "3304557",
			},
			"valores": map[string]float64{
				"valorServico": 5000.00,
				"baseCalculo":  5000.00,
				"aliquota":     2.00,
				"valorISSQN":   100.00,
				"valorLiquido": 4900.00,
			},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	result, err := client.QueryNFSe(context.Background(), "NFSe35012412345678000199990000100000000001234567890", nil)
	if err != nil {
		t.Fatalf("QueryNFSe failed: %v", err)
	}

	// Verify all parsed fields
	if result.Numero != "000000123" {
		t.Errorf("expected Numero 000000123, got %s", result.Numero)
	}
	if result.Status != "cancelled" {
		t.Errorf("expected Status cancelled, got %s", result.Status)
	}
	if result.Prestador.Nome != "Prestador ABC" {
		t.Errorf("expected Prestador.Nome 'Prestador ABC', got %s", result.Prestador.Nome)
	}
	if result.Servico.CodigoNacional != "020101" {
		t.Errorf("expected Servico.CodigoNacional 020101, got %s", result.Servico.CodigoNacional)
	}
	if result.Valores.Aliquota != 2.00 {
		t.Errorf("expected Valores.Aliquota 2.00, got %f", result.Valores.Aliquota)
	}
	if result.Tomador != nil {
		t.Error("expected Tomador to be nil when not in response")
	}
}

func TestQueryNFSe_EmptyChaveAcesso(t *testing.T) {
	client, err := NewProductionClient(ClientConfig{
		BaseURL:     "http://localhost",
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.QueryNFSe(context.Background(), "", nil)
	if err == nil {
		t.Error("expected error for empty chaveAcesso")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' in error message, got %v", err)
	}
}

func TestQueryNFSe_WithCertificate(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"chaveAcesso": "NFSe12345",
			"numero":      "000000001",
			"dataEmissao": "2024-01-15T10:30:00-03:00",
			"status":      "active",
			"xml":         "<NFSe/>",
			"prestador":   map[string]string{"documento": "12345678000199", "nome": "Test", "municipio": "SP", "municipioCodigo": "3550308"},
			"servico":     map[string]string{"codigoNacional": "010201", "descricao": "Test", "localPrestacao": "SP", "municipioCodigo": "3550308"},
			"valores":     map[string]float64{"valorServico": 100, "baseCalculo": 100, "aliquota": 5, "valorISSQN": 5, "valorLiquido": 95},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Create a mock certificate (empty for testing - actual mTLS not tested with httptest)
	cert := &tls.Certificate{}

	result, err := client.QueryNFSe(context.Background(), "NFSe12345", cert)
	if err != nil {
		t.Fatalf("QueryNFSe with certificate failed: %v", err)
	}
	if result.ChaveAcesso != "NFSe12345" {
		t.Errorf("expected ChaveAcesso NFSe12345, got %s", result.ChaveAcesso)
	}
}

func TestQueryNFSe_UnexpectedStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.QueryNFSe(context.Background(), "NFSe12345", nil)
	if err == nil {
		t.Error("expected error for 500 status code")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain status code 500, got %v", err)
	}
}

func TestQueryNFSe_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json {"))
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.QueryNFSe(context.Background(), "NFSe12345", nil)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got %v", err)
	}
}

// ================================================================================
// ProductionClient LookupDPS Tests
// ================================================================================

func TestLookupDPS_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET method, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/dps/") {
			t.Errorf("expected path to contain /dps/, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]string{
			"chaveAcesso": "NFSe35012312345678000199990000100000000001234567890",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	result, err := client.LookupDPS(context.Background(), "DPS123456789012345678901234567890123456789012", nil)
	if err != nil {
		t.Fatalf("LookupDPS failed: %v", err)
	}

	if result.DPSID != "DPS123456789012345678901234567890123456789012" {
		t.Errorf("expected DPSID DPS123456789012345678901234567890123456789012, got %s", result.DPSID)
	}
	if result.ChaveAcesso != "NFSe35012312345678000199990000100000000001234567890" {
		t.Errorf("expected ChaveAcesso NFSe35012312345678000199990000100000000001234567890, got %s", result.ChaveAcesso)
	}
}

func TestLookupDPS_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.LookupDPS(context.Background(), "DPS_NOTEXISTS", nil)
	if !errors.Is(err, ErrDPSNotFound) {
		t.Errorf("expected ErrDPSNotFound, got %v", err)
	}
}

func TestLookupDPS_Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.LookupDPS(context.Background(), "DPS_FORBIDDEN", nil)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestLookupDPS_ServiceUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.LookupDPS(context.Background(), "DPS12345", nil)
	if !errors.Is(err, ErrServiceUnavailable) {
		t.Errorf("expected ErrServiceUnavailable, got %v", err)
	}
}

func TestLookupDPS_EmptyDPSID(t *testing.T) {
	client, err := NewProductionClient(ClientConfig{
		BaseURL:     "http://localhost",
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.LookupDPS(context.Background(), "", nil)
	if err == nil {
		t.Error("expected error for empty dpsID")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' in error message, got %v", err)
	}
}

func TestLookupDPS_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err = client.LookupDPS(ctx, "DPS12345", nil)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

// ================================================================================
// ProductionClient CheckDPSExists Tests
// ================================================================================

func TestCheckDPSExists_ReturnsTrue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("expected HEAD method, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/dps/") {
			t.Errorf("expected path to contain /dps/, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	exists, err := client.CheckDPSExists(context.Background(), "DPS12345", nil)
	if err != nil {
		t.Fatalf("CheckDPSExists failed: %v", err)
	}
	if !exists {
		t.Error("expected exists to be true")
	}
}

func TestCheckDPSExists_ReturnsFalse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	exists, err := client.CheckDPSExists(context.Background(), "DPS_NOTEXISTS", nil)
	if err != nil {
		t.Fatalf("CheckDPSExists failed: %v", err)
	}
	if exists {
		t.Error("expected exists to be false for 404 response")
	}
}

func TestCheckDPSExists_ServiceUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.CheckDPSExists(context.Background(), "DPS12345", nil)
	if !errors.Is(err, ErrServiceUnavailable) {
		t.Errorf("expected ErrServiceUnavailable, got %v", err)
	}
}

func TestCheckDPSExists_EmptyDPSID(t *testing.T) {
	client, err := NewProductionClient(ClientConfig{
		BaseURL:     "http://localhost",
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.CheckDPSExists(context.Background(), "", nil)
	if err == nil {
		t.Error("expected error for empty dpsID")
	}
}

func TestCheckDPSExists_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err = client.CheckDPSExists(ctx, "DPS12345", nil)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestCheckDPSExists_UnexpectedStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.CheckDPSExists(context.Background(), "DPS12345", nil)
	if err == nil {
		t.Error("expected error for 500 status code")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain 500, got %v", err)
	}
}

// ================================================================================
// ProductionClient QueryEvents Tests
// ================================================================================

func TestQueryEvents_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET method, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/eventos") {
			t.Errorf("expected path to contain /eventos, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"eventos": []map[string]interface{}{
				{
					"tipo":      "EMISSAO",
					"descricao": "NFS-e emitida",
					"sequencia": 1,
					"data":      "2024-01-15T10:30:00-03:00",
					"xml":       "<evento>...</evento>",
				},
				{
					"tipo":      "e101101",
					"descricao": "Cancelamento de NFS-e",
					"sequencia": 2,
					"data":      "2024-01-16T14:00:00-03:00",
					"xml":       "<evento>...</evento>",
				},
			},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	result, err := client.QueryEvents(context.Background(), "NFSe12345", nil)
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}

	if result.ChaveAcesso != "NFSe12345" {
		t.Errorf("expected ChaveAcesso NFSe12345, got %s", result.ChaveAcesso)
	}
	if len(result.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(result.Events))
	}
	if result.Events[0].Tipo != "EMISSAO" {
		t.Errorf("expected first event type EMISSAO, got %s", result.Events[0].Tipo)
	}
	if result.Events[1].Tipo != "e101101" {
		t.Errorf("expected second event type e101101, got %s", result.Events[1].Tipo)
	}
	if result.Events[0].Sequencia != 1 {
		t.Errorf("expected first event sequencia 1, got %d", result.Events[0].Sequencia)
	}
}

func TestQueryEvents_EmptyList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"eventos": []map[string]interface{}{},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	result, err := client.QueryEvents(context.Background(), "NFSe12345", nil)
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}

	if len(result.Events) != 0 {
		t.Errorf("expected 0 events, got %d", len(result.Events))
	}
}

func TestQueryEvents_NFSeNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.QueryEvents(context.Background(), "NFSe_NOTEXISTS", nil)
	if !errors.Is(err, ErrNFSeNotFound) {
		t.Errorf("expected ErrNFSeNotFound, got %v", err)
	}
}

func TestQueryEvents_ServiceUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.QueryEvents(context.Background(), "NFSe12345", nil)
	if !errors.Is(err, ErrServiceUnavailable) {
		t.Errorf("expected ErrServiceUnavailable, got %v", err)
	}
}

func TestQueryEvents_EmptyChaveAcesso(t *testing.T) {
	client, err := NewProductionClient(ClientConfig{
		BaseURL:     "http://localhost",
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.QueryEvents(context.Background(), "", nil)
	if err == nil {
		t.Error("expected error for empty chaveAcesso")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected 'required' in error message, got %v", err)
	}
}

func TestQueryEvents_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err = client.QueryEvents(ctx, "NFSe12345", nil)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

// ================================================================================
// MockClient Tests
// ================================================================================

func TestMockClient_QueryNFSe_Success(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 0 // Disable latency for faster tests

	result, err := client.QueryNFSe(context.Background(), "NFSe12345", nil)
	if err != nil {
		t.Fatalf("MockClient.QueryNFSe failed: %v", err)
	}

	if result.ChaveAcesso != "NFSe12345" {
		t.Errorf("expected ChaveAcesso NFSe12345, got %s", result.ChaveAcesso)
	}
	if result.Status != "active" {
		t.Errorf("expected Status active, got %s", result.Status)
	}
	if result.Prestador.Documento == "" {
		t.Error("expected Prestador.Documento to be set")
	}
	if result.Tomador == nil {
		t.Error("expected Tomador to be set")
	}
}

func TestMockClient_QueryNFSe_NotFound(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 0

	_, err := client.QueryNFSe(context.Background(), "NOTFOUND12345", nil)
	if !errors.Is(err, ErrNFSeNotFound) {
		t.Errorf("expected ErrNFSeNotFound, got %v", err)
	}
}

func TestMockClient_QueryNFSe_SimulateFailure(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 0
	client.SimulateFailure = true

	_, err := client.QueryNFSe(context.Background(), "NFSe12345", nil)
	if !errors.Is(err, ErrServiceUnavailable) {
		t.Errorf("expected ErrServiceUnavailable, got %v", err)
	}
}

func TestMockClient_QueryNFSe_ContextCancellation(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 2 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.QueryNFSe(ctx, "NFSe12345", nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestMockClient_LookupDPS_Success(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 0

	result, err := client.LookupDPS(context.Background(), "DPS12345", nil)
	if err != nil {
		t.Fatalf("MockClient.LookupDPS failed: %v", err)
	}

	if result.DPSID != "DPS12345" {
		t.Errorf("expected DPSID DPS12345, got %s", result.DPSID)
	}
	if result.ChaveAcesso == "" {
		t.Error("expected ChaveAcesso to be set")
	}
}

func TestMockClient_LookupDPS_NotFound(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 0

	_, err := client.LookupDPS(context.Background(), "NOTFOUND12345", nil)
	if !errors.Is(err, ErrDPSNotFound) {
		t.Errorf("expected ErrDPSNotFound, got %v", err)
	}
}

func TestMockClient_LookupDPS_Forbidden(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 0

	_, err := client.LookupDPS(context.Background(), "FORBIDDEN12345", nil)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("expected ErrForbidden, got %v", err)
	}
}

func TestMockClient_LookupDPS_SimulateFailure(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 0
	client.SimulateFailure = true

	_, err := client.LookupDPS(context.Background(), "DPS12345", nil)
	if !errors.Is(err, ErrServiceUnavailable) {
		t.Errorf("expected ErrServiceUnavailable, got %v", err)
	}
}

func TestMockClient_CheckDPSExists_True(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 0

	exists, err := client.CheckDPSExists(context.Background(), "DPS12345", nil)
	if err != nil {
		t.Fatalf("MockClient.CheckDPSExists failed: %v", err)
	}
	if !exists {
		t.Error("expected exists to be true")
	}
}

func TestMockClient_CheckDPSExists_False(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 0

	exists, err := client.CheckDPSExists(context.Background(), "NOTFOUND12345", nil)
	if err != nil {
		t.Fatalf("MockClient.CheckDPSExists failed: %v", err)
	}
	if exists {
		t.Error("expected exists to be false")
	}
}

func TestMockClient_CheckDPSExists_SimulateFailure(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 0
	client.SimulateFailure = true

	_, err := client.CheckDPSExists(context.Background(), "DPS12345", nil)
	if !errors.Is(err, ErrServiceUnavailable) {
		t.Errorf("expected ErrServiceUnavailable, got %v", err)
	}
}

func TestMockClient_QueryEvents_Success(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 0

	result, err := client.QueryEvents(context.Background(), "NFSe12345", nil)
	if err != nil {
		t.Fatalf("MockClient.QueryEvents failed: %v", err)
	}

	if result.ChaveAcesso != "NFSe12345" {
		t.Errorf("expected ChaveAcesso NFSe12345, got %s", result.ChaveAcesso)
	}
	if len(result.Events) == 0 {
		t.Error("expected at least one event")
	}
	if result.Events[0].Tipo != "EMISSAO" {
		t.Errorf("expected first event type EMISSAO, got %s", result.Events[0].Tipo)
	}
}

func TestMockClient_QueryEvents_EmptyList(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 0

	result, err := client.QueryEvents(context.Background(), "NOEVENTS12345", nil)
	if err != nil {
		t.Fatalf("MockClient.QueryEvents failed: %v", err)
	}

	if len(result.Events) != 0 {
		t.Errorf("expected 0 events for NOEVENTS prefix, got %d", len(result.Events))
	}
}

func TestMockClient_QueryEvents_NotFound(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 0

	_, err := client.QueryEvents(context.Background(), "NOTFOUND12345", nil)
	if !errors.Is(err, ErrNFSeNotFound) {
		t.Errorf("expected ErrNFSeNotFound, got %v", err)
	}
}

func TestMockClient_QueryEvents_CancelledNFSe(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 0

	result, err := client.QueryEvents(context.Background(), "CANCELLED12345", nil)
	if err != nil {
		t.Fatalf("MockClient.QueryEvents failed: %v", err)
	}

	if len(result.Events) < 2 {
		t.Errorf("expected at least 2 events for CANCELLED prefix, got %d", len(result.Events))
	}

	// Check for cancellation event
	hasCancellation := false
	for _, evt := range result.Events {
		if evt.Tipo == "e101101" {
			hasCancellation = true
			break
		}
	}
	if !hasCancellation {
		t.Error("expected cancellation event (e101101) for CANCELLED prefix")
	}
}

func TestMockClient_QueryEvents_SimulateFailure(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 0
	client.SimulateFailure = true

	_, err := client.QueryEvents(context.Background(), "NFSe12345", nil)
	if !errors.Is(err, ErrServiceUnavailable) {
		t.Errorf("expected ErrServiceUnavailable, got %v", err)
	}
}

// ================================================================================
// MockClient FailureRate Tests
// ================================================================================

func TestMockClient_FailureRate_ZeroNeverFails(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 0
	client.FailureRate = 0

	// Should never fail
	for i := 0; i < 10; i++ {
		_, err := client.QueryNFSe(context.Background(), "NFSe12345", nil)
		if err != nil {
			t.Errorf("iteration %d: expected no error with FailureRate=0, got %v", i, err)
		}
	}
}

func TestMockClient_FailureRate_HundredAlwaysFails(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 0
	client.FailureRate = 100

	// Should always fail
	for i := 0; i < 10; i++ {
		_, err := client.QueryNFSe(context.Background(), "NFSe12345", nil)
		if err == nil {
			t.Errorf("iteration %d: expected error with FailureRate=100", i)
		}
	}
}

// ================================================================================
// ProductionClient Configuration Tests
// ================================================================================

func TestNewProductionClient_DefaultBaseURL_Production(t *testing.T) {
	client, err := NewProductionClient(ClientConfig{
		Environment: EnvironmentProduction,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if client.baseURL != ProductionBaseURL {
		t.Errorf("expected baseURL %s, got %s", ProductionBaseURL, client.baseURL)
	}
	if client.environment != 1 {
		t.Errorf("expected environment 1 (production), got %d", client.environment)
	}
}

func TestNewProductionClient_DefaultBaseURL_Homologation(t *testing.T) {
	client, err := NewProductionClient(ClientConfig{
		Environment: EnvironmentHomologation,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if client.baseURL != HomologationBaseURL {
		t.Errorf("expected baseURL %s, got %s", HomologationBaseURL, client.baseURL)
	}
	if client.environment != 2 {
		t.Errorf("expected environment 2 (homologation), got %d", client.environment)
	}
}

func TestNewProductionClient_CustomBaseURL(t *testing.T) {
	customURL := "https://custom.api.example.com"
	client, err := NewProductionClient(ClientConfig{
		BaseURL:     customURL,
		Environment: EnvironmentHomologation,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if client.baseURL != customURL {
		t.Errorf("expected baseURL %s, got %s", customURL, client.baseURL)
	}
}

func TestNewProductionClient_DefaultTimeout(t *testing.T) {
	client, err := NewProductionClient(ClientConfig{
		Environment: EnvironmentHomologation,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	expectedTimeout := 30 * time.Second
	if client.timeout != expectedTimeout {
		t.Errorf("expected default timeout %v, got %v", expectedTimeout, client.timeout)
	}
}

func TestNewProductionClient_CustomTimeout(t *testing.T) {
	customTimeout := 60 * time.Second
	client, err := NewProductionClient(ClientConfig{
		Environment: EnvironmentHomologation,
		Timeout:     customTimeout,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if client.timeout != customTimeout {
		t.Errorf("expected timeout %v, got %v", customTimeout, client.timeout)
	}
}

func TestNewProductionClient_DefaultRetries(t *testing.T) {
	client, err := NewProductionClient(ClientConfig{
		Environment: EnvironmentHomologation,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if client.maxRetries != 3 {
		t.Errorf("expected default maxRetries 3, got %d", client.maxRetries)
	}
}

func TestProductionClient_GetBaseURL(t *testing.T) {
	client, err := NewProductionClient(ClientConfig{
		BaseURL:     "https://test.example.com",
		Environment: EnvironmentHomologation,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if client.GetBaseURL() != "https://test.example.com" {
		t.Errorf("expected GetBaseURL to return https://test.example.com, got %s", client.GetBaseURL())
	}
}

func TestProductionClient_GetEnvironment(t *testing.T) {
	clientProd, _ := NewProductionClient(ClientConfig{
		Environment: EnvironmentProduction,
	})
	if clientProd.GetEnvironment() != 1 {
		t.Errorf("expected GetEnvironment to return 1 for production, got %d", clientProd.GetEnvironment())
	}

	clientHomolog, _ := NewProductionClient(ClientConfig{
		Environment: EnvironmentHomologation,
	})
	if clientHomolog.GetEnvironment() != 2 {
		t.Errorf("expected GetEnvironment to return 2 for homologation, got %d", clientHomolog.GetEnvironment())
	}
}

// ================================================================================
// Interface Compliance Tests
// ================================================================================

func TestProductionClient_ImplementsSefinClient(t *testing.T) {
	var _ SefinClient = (*ProductionClient)(nil)
}

func TestMockClient_ImplementsSefinClient(t *testing.T) {
	var _ SefinClient = (*MockClient)(nil)
}

// ================================================================================
// Additional Coverage Tests - Date Parsing Variations
// ================================================================================

func TestQueryNFSe_AlternativeDateFormat(t *testing.T) {
	// Test with alternative date format that falls back to second parser
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"chaveAcesso": "NFSe12345",
			"numero":      "000000001",
			"dataEmissao": "2024-01-15T10:30:00-0300", // Alternative format without colon in timezone
			"status":      "active",
			"xml":         "<NFSe/>",
			"prestador":   map[string]string{"documento": "12345678000199", "nome": "Test", "municipio": "SP", "municipioCodigo": "3550308"},
			"servico":     map[string]string{"codigoNacional": "010201", "descricao": "Test", "localPrestacao": "SP", "municipioCodigo": "3550308"},
			"valores":     map[string]float64{"valorServico": 100, "baseCalculo": 100, "aliquota": 5, "valorISSQN": 5, "valorLiquido": 95},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	result, err := client.QueryNFSe(context.Background(), "NFSe12345", nil)
	if err != nil {
		t.Fatalf("QueryNFSe failed: %v", err)
	}

	// Date should still be parsed (or be zero time if both formats fail)
	if result.ChaveAcesso != "NFSe12345" {
		t.Errorf("expected ChaveAcesso NFSe12345, got %s", result.ChaveAcesso)
	}
}

func TestQueryNFSe_InvalidDateFormat(t *testing.T) {
	// Test with completely invalid date format
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"chaveAcesso": "NFSe12345",
			"numero":      "000000001",
			"dataEmissao": "invalid-date-format",
			"status":      "active",
			"xml":         "<NFSe/>",
			"prestador":   map[string]string{"documento": "12345678000199", "nome": "Test", "municipio": "SP", "municipioCodigo": "3550308"},
			"servico":     map[string]string{"codigoNacional": "010201", "descricao": "Test", "localPrestacao": "SP", "municipioCodigo": "3550308"},
			"valores":     map[string]float64{"valorServico": 100, "baseCalculo": 100, "aliquota": 5, "valorISSQN": 5, "valorLiquido": 95},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	result, err := client.QueryNFSe(context.Background(), "NFSe12345", nil)
	if err != nil {
		t.Fatalf("QueryNFSe should not fail on invalid date: %v", err)
	}

	// Date should be zero time when parsing fails
	if !result.DataEmissao.IsZero() {
		t.Errorf("expected zero time for invalid date, got %v", result.DataEmissao)
	}
}

func TestQueryEvents_AlternativeDateFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"eventos": []map[string]interface{}{
				{
					"tipo":      "EMISSAO",
					"descricao": "NFS-e emitida",
					"sequencia": 1,
					"data":      "2024-01-15T10:30:00-0300", // Alternative format
					"xml":       "<evento/>",
				},
			},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	result, err := client.QueryEvents(context.Background(), "NFSe12345", nil)
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}

	if len(result.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(result.Events))
	}
}

func TestQueryEvents_InvalidDateFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"eventos": []map[string]interface{}{
				{
					"tipo":      "EMISSAO",
					"descricao": "NFS-e emitida",
					"sequencia": 1,
					"data":      "not-a-date",
					"xml":       "<evento/>",
				},
			},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	result, err := client.QueryEvents(context.Background(), "NFSe12345", nil)
	if err != nil {
		t.Fatalf("QueryEvents should not fail on invalid date: %v", err)
	}

	if len(result.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(result.Events))
	}
	if !result.Events[0].Data.IsZero() {
		t.Errorf("expected zero time for invalid date, got %v", result.Events[0].Data)
	}
}

func TestLookupDPS_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json {"))
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.LookupDPS(context.Background(), "DPS12345", nil)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got %v", err)
	}
}

func TestQueryEvents_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json {"))
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.QueryEvents(context.Background(), "NFSe12345", nil)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got %v", err)
	}
}

func TestLookupDPS_UnexpectedStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.LookupDPS(context.Background(), "DPS12345", nil)
	if err == nil {
		t.Error("expected error for 500 status code")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain 500, got %v", err)
	}
}

func TestQueryEvents_UnexpectedStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.QueryEvents(context.Background(), "NFSe12345", nil)
	if err == nil {
		t.Error("expected error for 500 status code")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain 500, got %v", err)
	}
}

// ================================================================================
// MockClient Context Cancellation Tests
// ================================================================================

func TestMockClient_LookupDPS_ContextCancellation(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 2 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.LookupDPS(ctx, "DPS12345", nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestMockClient_CheckDPSExists_ContextCancellation(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 2 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.CheckDPSExists(ctx, "DPS12345", nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestMockClient_QueryEvents_ContextCancellation(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 2 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.QueryEvents(ctx, "NFSe12345", nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// ================================================================================
// ProductionClient with Logger Tests
// ================================================================================

func TestProductionClient_SetLogger(t *testing.T) {
	client, err := NewProductionClient(ClientConfig{
		Environment: EnvironmentHomologation,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Should not panic when setting nil logger
	client.SetLogger(nil)
}

// ================================================================================
// Additional MockClient Tests
// ================================================================================

func TestMockClient_QueryNFSe_WithLatency(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 100 * time.Millisecond

	start := time.Now()
	_, err := client.QueryNFSe(context.Background(), "NFSe12345", nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("MockClient.QueryNFSe failed: %v", err)
	}
	if elapsed < 100*time.Millisecond {
		t.Errorf("expected latency of at least 100ms, got %v", elapsed)
	}
}

func TestMockClient_CheckDPSExists_ShorterLatency(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 200 * time.Millisecond

	start := time.Now()
	_, err := client.CheckDPSExists(context.Background(), "DPS12345", nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("MockClient.CheckDPSExists failed: %v", err)
	}
	// CheckDPSExists uses half the latency
	if elapsed >= 200*time.Millisecond {
		t.Errorf("expected latency less than 200ms (half of configured), got %v", elapsed)
	}
}

// ================================================================================
// SubmitDPS Tests (ProductionClient)
// ================================================================================

func TestSubmitDPS_Success(t *testing.T) {
	// Create test server that returns a successful SOAP response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}

		// Verify Content-Type header
		if !strings.Contains(r.Header.Get("Content-Type"), "text/xml") {
			t.Errorf("expected Content-Type text/xml, got %s", r.Header.Get("Content-Type"))
		}

		// Verify SOAPAction header
		if r.Header.Get("SOAPAction") != SOAPActionDPS {
			t.Errorf("expected SOAPAction %s, got %s", SOAPActionDPS, r.Header.Get("SOAPAction"))
		}

		// Return mock SOAP success response
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)
		soapResponse := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope">
  <soap:Body>
    <RecepcionarDPSResponse>
      <resultado>
        <sucesso>true</sucesso>
        <protocolo>202401151030001234</protocolo>
        <NFSe>
          <chaveAcesso>NFSe35012312345678000199990000100000000001234567890</chaveAcesso>
          <nNFSe>000000001</nNFSe>
          <xmlNFSe><![CDATA[<NFSe>...</NFSe>]]></xmlNFSe>
        </NFSe>
      </resultado>
    </RecepcionarDPSResponse>
  </soap:Body>
</soap:Envelope>`
		w.Write([]byte(soapResponse))
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
		MaxRetries:  0, // Disable retries for this test
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	result, err := client.SubmitDPS(context.Background(), "<DPS>test</DPS>", EnvironmentHomologation)
	if err != nil {
		t.Fatalf("SubmitDPS failed: %v", err)
	}

	if !result.Success {
		t.Errorf("expected Success to be true, got false")
	}
	if result.ChaveAcesso != "NFSe35012312345678000199990000100000000001234567890" {
		t.Errorf("expected ChaveAcesso NFSe35012312345678000199990000100000000001234567890, got %s", result.ChaveAcesso)
	}
	if result.NFSeNumber != "000000001" {
		t.Errorf("expected NFSeNumber 000000001, got %s", result.NFSeNumber)
	}
	if result.ProtocolNumber != "202401151030001234" {
		t.Errorf("expected ProtocolNumber 202401151030001234, got %s", result.ProtocolNumber)
	}
	if result.ProcessingTime == 0 {
		t.Error("expected ProcessingTime to be set")
	}
}

func TestSubmitDPS_WithErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)
		soapResponse := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope">
  <soap:Body>
    <RecepcionarDPSResponse>
      <resultado>
        <sucesso>false</sucesso>
        <protocolo>202401151030001234</protocolo>
        <erros>
          <erro>
            <codigo>E001</codigo>
            <mensagem>CNPJ invalido</mensagem>
          </erro>
          <erro>
            <codigo>E002</codigo>
            <mensagem>Data de emissao invalida</mensagem>
          </erro>
        </erros>
      </resultado>
    </RecepcionarDPSResponse>
  </soap:Body>
</soap:Envelope>`
		w.Write([]byte(soapResponse))
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
		MaxRetries:  0,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	result, err := client.SubmitDPS(context.Background(), "<DPS>test</DPS>", EnvironmentHomologation)
	if err != nil {
		t.Fatalf("SubmitDPS failed: %v", err)
	}

	if result.Success {
		t.Error("expected Success to be false")
	}
	if result.ErrorCode != "E001" {
		t.Errorf("expected first ErrorCode E001, got %s", result.ErrorCode)
	}
	if len(result.ErrorCodes) != 2 {
		t.Errorf("expected 2 error codes, got %d", len(result.ErrorCodes))
	}
	if !strings.Contains(result.ErrorMessage, "CNPJ invalido") {
		t.Errorf("expected error message to contain 'CNPJ invalido', got %s", result.ErrorMessage)
	}
}

func TestSubmitDPS_SOAPFault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)
		soapResponse := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope">
  <soap:Body>
    <soap:Fault>
      <soap:Code>
        <soap:Value>soap:Server</soap:Value>
      </soap:Code>
      <soap:Reason>
        <soap:Text>Internal server error</soap:Text>
      </soap:Reason>
    </soap:Fault>
  </soap:Body>
</soap:Envelope>`
		w.Write([]byte(soapResponse))
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
		MaxRetries:  0,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	result, err := client.SubmitDPS(context.Background(), "<DPS>test</DPS>", EnvironmentHomologation)
	if err != nil {
		t.Fatalf("SubmitDPS failed: %v", err)
	}

	if result.Success {
		t.Error("expected Success to be false for SOAP fault")
	}
}

func TestSubmitDPS_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
		MaxRetries:  0,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.SubmitDPS(context.Background(), "<DPS>test</DPS>", EnvironmentHomologation)
	if err == nil {
		t.Error("expected error for HTTP 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to contain 500, got %v", err)
	}
}

func TestSubmitDPS_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
		MaxRetries:  0,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = client.SubmitDPS(ctx, "<DPS>test</DPS>", EnvironmentHomologation)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestSubmitDPS_RetryOnNetworkError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			// Close the connection without responding (simulates network error)
			hj, ok := w.(http.Hijacker)
			if ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
				return
			}
		}
		// Success on second attempt
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)
		soapResponse := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope">
  <soap:Body>
    <RecepcionarDPSResponse>
      <resultado>
        <sucesso>true</sucesso>
        <protocolo>12345</protocolo>
        <NFSe>
          <chaveAcesso>NFSe123</chaveAcesso>
          <nNFSe>1</nNFSe>
          <xmlNFSe>test</xmlNFSe>
        </NFSe>
      </resultado>
    </RecepcionarDPSResponse>
  </soap:Body>
</soap:Envelope>`
		w.Write([]byte(soapResponse))
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
		MaxRetries:  3,
		RetryDelay:  10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	result, err := client.SubmitDPS(context.Background(), "<DPS>test</DPS>", EnvironmentHomologation)
	if err != nil {
		t.Fatalf("SubmitDPS failed: %v", err)
	}
	if !result.Success {
		t.Error("expected success after retry")
	}
}

func TestSubmitDPS_MaxRetriesExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always close connection to simulate persistent network error
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
			return
		}
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     1 * time.Second,
		MaxRetries:  2,
		RetryDelay:  10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.SubmitDPS(context.Background(), "<DPS>test</DPS>", EnvironmentHomologation)
	if err == nil {
		t.Error("expected error after max retries exceeded")
	}
	if !strings.Contains(err.Error(), "after") {
		t.Errorf("expected error message to mention retry attempts, got %v", err)
	}
}

func TestSubmitDPS_InvalidXMLResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)
		// Return invalid XML that can't be parsed as SOAP
		w.Write([]byte("this is not xml at all"))
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
		MaxRetries:  0,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	result, err := client.SubmitDPS(context.Background(), "<DPS>test</DPS>", EnvironmentHomologation)
	if err != nil {
		t.Fatalf("SubmitDPS should not fail on unparseable response: %v", err)
	}
	// Should fall back to parseRawResponse
	if result.Success {
		t.Error("expected Success to be false for unparseable response")
	}
}

func TestSubmitDPS_RawResponseWithSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)
		// Return non-SOAP XML that contains success indicator
		w.Write([]byte(`<response><sucesso>true</sucesso><protocolo>ABC123</protocolo></response>`))
	}))
	defer server.Close()

	client, err := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
		MaxRetries:  0,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	result, err := client.SubmitDPS(context.Background(), "<DPS>test</DPS>", EnvironmentHomologation)
	if err != nil {
		t.Fatalf("SubmitDPS failed: %v", err)
	}
	// parseRawResponse should detect success
	if !result.Success {
		t.Error("expected Success to be true for raw response with <sucesso>true</sucesso>")
	}
	if result.ProtocolNumber != "ABC123" {
		t.Errorf("expected ProtocolNumber ABC123, got %s", result.ProtocolNumber)
	}
}

// ================================================================================
// isRetryableError Tests
// ================================================================================

func TestIsRetryableError(t *testing.T) {
	client, _ := NewProductionClient(ClientConfig{
		Environment: EnvironmentHomologation,
	})

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"connection refused", errors.New("connection refused"), true},
		{"connection reset", errors.New("connection reset by peer"), true},
		{"timeout", errors.New("request timeout"), true},
		{"i/o timeout", errors.New("i/o timeout"), true},
		{"EOF", errors.New("unexpected EOF"), true},
		{"no such host", errors.New("no such host"), true},
		{"server misbehaving", errors.New("server misbehaving"), true},
		{"temporary failure", errors.New("temporary failure in name resolution"), true},
		{"validation error", errors.New("validation failed: invalid CNPJ"), false},
		{"auth error", errors.New("authentication failed"), false},
		{"generic error", errors.New("some random error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.isRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("isRetryableError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

// ================================================================================
// buildSOAPEnvelope Tests
// ================================================================================

func TestBuildSOAPEnvelope(t *testing.T) {
	client, _ := NewProductionClient(ClientConfig{
		Environment: EnvironmentHomologation,
	})

	dpsXML := "<DPS><test>content</test></DPS>"
	envelope := client.buildSOAPEnvelope(dpsXML)

	// Verify envelope structure
	if !strings.Contains(envelope, "soap:Envelope") {
		t.Error("expected SOAP envelope to contain soap:Envelope")
	}
	if !strings.Contains(envelope, "soap:Body") {
		t.Error("expected SOAP envelope to contain soap:Body")
	}
	if !strings.Contains(envelope, "RecepcionarDPSRequest") {
		t.Error("expected SOAP envelope to contain RecepcionarDPSRequest")
	}
	if !strings.Contains(envelope, "versaoDados") {
		t.Error("expected SOAP envelope to contain versaoDados")
	}
	if !strings.Contains(envelope, "1.00") {
		t.Error("expected SOAP envelope to contain version 1.00")
	}
	if !strings.Contains(envelope, dpsXML) {
		t.Error("expected SOAP envelope to contain the DPS XML")
	}
}

// ================================================================================
// MockClient SubmitDPS Tests
// ================================================================================

func TestMockClient_SubmitDPS_Success(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 0

	result, err := client.SubmitDPS(context.Background(), "<DPS>test</DPS>", EnvironmentHomologation)
	if err != nil {
		t.Fatalf("MockClient.SubmitDPS failed: %v", err)
	}

	if !result.Success {
		t.Error("expected Success to be true")
	}
	if result.ChaveAcesso == "" {
		t.Error("expected ChaveAcesso to be set")
	}
	if result.NFSeNumber == "" {
		t.Error("expected NFSeNumber to be set")
	}
	if result.ProtocolNumber == "" {
		t.Error("expected ProtocolNumber to be set")
	}
	if result.NFSeXML == "" {
		t.Error("expected NFSeXML to be set")
	}
	// ProcessingTime may be 0 with no latency, that's acceptable
}

func TestMockClient_SubmitDPS_SimulateFailure(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 0
	client.SimulateFailure = true

	result, err := client.SubmitDPS(context.Background(), "<DPS>test</DPS>", EnvironmentHomologation)
	if err != nil {
		t.Fatalf("MockClient.SubmitDPS should return result, not error: %v", err)
	}

	if result.Success {
		t.Error("expected Success to be false with SimulateFailure=true")
	}
	if result.ErrorCode != "E001" {
		t.Errorf("expected ErrorCode E001, got %s", result.ErrorCode)
	}
	if !strings.Contains(result.ErrorMessage, "Mock rejection") {
		t.Errorf("expected ErrorMessage to contain 'Mock rejection', got %s", result.ErrorMessage)
	}
}

func TestMockClient_SubmitDPS_ContextCancellation(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 2 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.SubmitDPS(ctx, "<DPS>test</DPS>", EnvironmentHomologation)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestMockClient_SubmitDPS_WithLatency(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 100 * time.Millisecond

	start := time.Now()
	_, err := client.SubmitDPS(context.Background(), "<DPS>test</DPS>", EnvironmentHomologation)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("MockClient.SubmitDPS failed: %v", err)
	}
	if elapsed < 100*time.Millisecond {
		t.Errorf("expected latency of at least 100ms, got %v", elapsed)
	}
}

func TestMockClient_SubmitDPS_FailureRate(t *testing.T) {
	client := NewMockClient()
	client.SimulatedLatency = 0
	client.FailureRate = 100 // Always fail

	result, err := client.SubmitDPS(context.Background(), "<DPS>test</DPS>", EnvironmentHomologation)
	if err != nil {
		t.Fatalf("MockClient.SubmitDPS should return result: %v", err)
	}
	if result.Success {
		t.Error("expected failure with FailureRate=100")
	}
}

// ================================================================================
// logDebug Tests
// ================================================================================

func TestLogDebug_WithLogger(t *testing.T) {
	var buf strings.Builder
	logger := log.New(&buf, "TEST: ", 0)

	client, _ := NewProductionClient(ClientConfig{
		Environment: EnvironmentHomologation,
		Logger:      logger,
	})

	client.logDebug("test message %d", 42)

	output := buf.String()
	if !strings.Contains(output, "test message 42") {
		t.Errorf("expected log output to contain 'test message 42', got %s", output)
	}
}

func TestLogDebug_WithoutLogger(t *testing.T) {
	client, _ := NewProductionClient(ClientConfig{
		Environment: EnvironmentHomologation,
	})

	// Should not panic when logger is nil
	client.logDebug("test message %d", 42)
}

// ================================================================================
// createQueryHTTPClient Tests
// ================================================================================

func TestCreateQueryHTTPClient_WithCertificate(t *testing.T) {
	client, _ := NewProductionClient(ClientConfig{
		Environment: EnvironmentHomologation,
	})

	cert := &tls.Certificate{}
	httpClient := client.createQueryHTTPClient(cert)

	if httpClient == nil {
		t.Error("expected httpClient to be created")
	}
	if httpClient.Timeout != QueryTimeout {
		t.Errorf("expected timeout %v, got %v", QueryTimeout, httpClient.Timeout)
	}
}

func TestCreateQueryHTTPClient_WithoutCertificate(t *testing.T) {
	client, _ := NewProductionClient(ClientConfig{
		Environment: EnvironmentHomologation,
	})

	httpClient := client.createQueryHTTPClient(nil)

	if httpClient == nil {
		t.Error("expected httpClient to be created")
	}
	if httpClient.Timeout != QueryTimeout {
		t.Errorf("expected timeout %v, got %v", QueryTimeout, httpClient.Timeout)
	}
}

// ================================================================================
// NewProductionClient Additional Tests
// ================================================================================

func TestNewProductionClient_WithCertificate(t *testing.T) {
	cert := &tls.Certificate{}
	client, err := NewProductionClient(ClientConfig{
		Environment: EnvironmentHomologation,
		Certificate: cert,
	})
	if err != nil {
		t.Fatalf("failed to create client with certificate: %v", err)
	}
	if client == nil {
		t.Error("expected client to be created")
	}
}

func TestNewProductionClient_InsecureSkipVerify(t *testing.T) {
	client, err := NewProductionClient(ClientConfig{
		Environment:        EnvironmentHomologation,
		InsecureSkipVerify: true,
	})
	if err != nil {
		t.Fatalf("failed to create client with InsecureSkipVerify: %v", err)
	}
	if client == nil {
		t.Error("expected client to be created")
	}
}

func TestNewProductionClient_CustomRetrySettings(t *testing.T) {
	client, err := NewProductionClient(ClientConfig{
		Environment: EnvironmentHomologation,
		MaxRetries:  5,
		RetryDelay:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	if client.maxRetries != 5 {
		t.Errorf("expected maxRetries 5, got %d", client.maxRetries)
	}
	if client.retryDelay != 5*time.Second {
		t.Errorf("expected retryDelay 5s, got %v", client.retryDelay)
	}
}

// ================================================================================
// parseSOAPResponse Additional Tests
// ================================================================================

func TestParseSOAPResponse_EmptyResponse(t *testing.T) {
	client, _ := NewProductionClient(ClientConfig{
		Environment: EnvironmentHomologation,
	})

	result, err := client.parseSOAPResponse("")
	if err != nil {
		t.Fatalf("parseSOAPResponse failed: %v", err)
	}
	if result.Success {
		t.Error("expected Success to be false for empty response")
	}
}

func TestParseSOAPResponse_ValidSuccess(t *testing.T) {
	client, _ := NewProductionClient(ClientConfig{
		Environment: EnvironmentHomologation,
	})

	soapResponse := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope">
  <soap:Body>
    <RecepcionarDPSResponse>
      <resultado>
        <sucesso>true</sucesso>
        <protocolo>PROT123</protocolo>
        <NFSe>
          <chaveAcesso>CHAVE123</chaveAcesso>
          <nNFSe>000001</nNFSe>
          <xmlNFSe>xml content</xmlNFSe>
        </NFSe>
      </resultado>
    </RecepcionarDPSResponse>
  </soap:Body>
</soap:Envelope>`

	result, err := client.parseSOAPResponse(soapResponse)
	if err != nil {
		t.Fatalf("parseSOAPResponse failed: %v", err)
	}
	if !result.Success {
		t.Error("expected Success to be true")
	}
	if result.ChaveAcesso != "CHAVE123" {
		t.Errorf("expected ChaveAcesso CHAVE123, got %s", result.ChaveAcesso)
	}
	if result.NFSeNumber != "000001" {
		t.Errorf("expected NFSeNumber 000001, got %s", result.NFSeNumber)
	}
	if result.NFSeXML != "xml content" {
		t.Errorf("expected NFSeXML 'xml content', got %s", result.NFSeXML)
	}
}

// ================================================================================
// Mock Helper Functions Tests
// ================================================================================

func TestGenerateMockNFSeNumber(t *testing.T) {
	// Generate multiple numbers and verify format
	for i := 0; i < 10; i++ {
		num := generateMockNFSeNumber()
		if len(num) != 9 {
			t.Errorf("expected NFSe number length 9, got %d for %s", len(num), num)
		}
	}
}

func TestGenerateMockChaveAcesso(t *testing.T) {
	chave := generateMockChaveAcesso(EnvironmentHomologation)
	if !strings.HasPrefix(chave, "NFSe") {
		t.Errorf("expected chaveAcesso to start with NFSe, got %s", chave)
	}
}

func TestGenerateMockProtocolNumber(t *testing.T) {
	protocol := generateMockProtocolNumber()
	if len(protocol) != 20 { // timestamp (14) + random (6)
		t.Errorf("expected protocol number length 20, got %d for %s", len(protocol), protocol)
	}
}

func TestGenerateMockNFSeXML(t *testing.T) {
	xml := generateMockNFSeXML("<DPS/>", "000001", "CHAVE123")
	if !strings.Contains(xml, "NFSe") {
		t.Error("expected XML to contain NFSe element")
	}
	if !strings.Contains(xml, "000001") {
		t.Error("expected XML to contain NFSe number")
	}
	if !strings.Contains(xml, "CHAVE123") {
		t.Error("expected XML to contain access key")
	}
}

func TestGenerateMockEventXML(t *testing.T) {
	xml := generateMockEventXML("EMISSAO", "CHAVE123", 1)
	if !strings.Contains(xml, "evento") {
		t.Error("expected XML to contain evento element")
	}
	if !strings.Contains(xml, "EMISSAO") {
		t.Error("expected XML to contain event type")
	}
	if !strings.Contains(xml, "CHAVE123") {
		t.Error("expected XML to contain access key")
	}
}

// ================================================================================
// Additional Query Method Edge Cases
// ================================================================================

func TestQueryNFSe_ConnectionError(t *testing.T) {
	// Use an invalid URL to force connection error
	client, err := NewProductionClient(ClientConfig{
		BaseURL:     "http://localhost:1", // Port 1 is unlikely to be listening
		Environment: EnvironmentHomologation,
		Timeout:     1 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.QueryNFSe(context.Background(), "NFSe12345", nil)
	if err == nil {
		t.Error("expected connection error")
	}
}

func TestLookupDPS_ConnectionError(t *testing.T) {
	client, err := NewProductionClient(ClientConfig{
		BaseURL:     "http://localhost:1",
		Environment: EnvironmentHomologation,
		Timeout:     1 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.LookupDPS(context.Background(), "DPS12345", nil)
	if err == nil {
		t.Error("expected connection error")
	}
}

func TestCheckDPSExists_ConnectionError(t *testing.T) {
	client, err := NewProductionClient(ClientConfig{
		BaseURL:     "http://localhost:1",
		Environment: EnvironmentHomologation,
		Timeout:     1 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.CheckDPSExists(context.Background(), "DPS12345", nil)
	if err == nil {
		t.Error("expected connection error")
	}
}

func TestQueryEvents_ConnectionError(t *testing.T) {
	client, err := NewProductionClient(ClientConfig{
		BaseURL:     "http://localhost:1",
		Environment: EnvironmentHomologation,
		Timeout:     1 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	_, err = client.QueryEvents(context.Background(), "NFSe12345", nil)
	if err == nil {
		t.Error("expected connection error")
	}
}

// ================================================================================
// Retry Delay Cap Test
// ================================================================================

func TestSubmitDPS_RetryDelayCapped(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			hj, ok := w.(http.Hijacker)
			if ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
				return
			}
		}
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope">
  <soap:Body>
    <RecepcionarDPSResponse>
      <resultado><sucesso>true</sucesso></resultado>
    </RecepcionarDPSResponse>
  </soap:Body>
</soap:Envelope>`))
	}))
	defer server.Close()

	// Use a very large retry delay that would exceed the 30s cap
	client, _ := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
		MaxRetries:  3,
		RetryDelay:  100 * time.Second, // Will be capped to 30s
	})

	start := time.Now()
	_, err := client.SubmitDPS(context.Background(), "<DPS/>", EnvironmentHomologation)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("SubmitDPS failed: %v", err)
	}

	// With capped delay, retries should complete much faster than uncapped
	// 2 retries with max 30s each = 60s max, but we're using short delays in practice
	if elapsed > 120*time.Second {
		t.Errorf("retries took too long, delay cap may not be working: %v", elapsed)
	}
}

// ================================================================================
// SubmitDPS Context Cancellation During Retry
// ================================================================================

func TestSubmitDPS_ContextCancelledDuringRetry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
			return
		}
	}))
	defer server.Close()

	client, _ := NewProductionClient(ClientConfig{
		BaseURL:     server.URL,
		Environment: EnvironmentHomologation,
		Timeout:     10 * time.Second,
		MaxRetries:  10,
		RetryDelay:  500 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.SubmitDPS(ctx, "<DPS/>", EnvironmentHomologation)
	if err == nil {
		t.Error("expected error when context cancelled during retry")
	}
}
