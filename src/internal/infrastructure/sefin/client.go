// Package sefin provides client implementations for communicating with
// the SEFIN (Secretaria da Fazenda) government API for the Sistema Nacional NFS-e.
package sefin

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"
)

// API endpoints for the Sistema Nacional NFS-e.
const (
	// ProductionBaseURL is the production API endpoint.
	ProductionBaseURL = "https://sefin.nfse.gov.br/nfse"

	// HomologationBaseURL is the homologation (testing) API endpoint.
	HomologationBaseURL = "https://homolog.sefin.nfse.gov.br/nfse"
)

// SOAP action headers for different operations.
const (
	// SOAPActionDPS is the SOAP action for DPS submission.
	SOAPActionDPS = "http://www.sped.fazenda.gov.br/nfse/wsdl/RecepcionarDPS"

	// SOAPActionQuery is the SOAP action for NFS-e queries.
	SOAPActionQuery = "http://www.sped.fazenda.gov.br/nfse/wsdl/ConsultarNFSe"
)

// Query operation timeouts and limits.
const (
	// QueryTimeout is the default timeout for query operations (10 seconds as per spec).
	QueryTimeout = 10 * time.Second
)

// SefinClient defines the interface for interacting with the SEFIN API.
type SefinClient interface {
	// SubmitDPS submits a DPS XML document for processing and returns the result.
	SubmitDPS(ctx context.Context, dpsXML string, environment string) (*SefinResponse, error)

	// QueryNFSe retrieves an NFS-e by its 50-character access key (chaveAcesso).
	// The certificate parameter is used for mTLS authentication with the government API.
	// Returns ErrNFSeNotFound if the NFS-e does not exist.
	QueryNFSe(ctx context.Context, chaveAcesso string, cert *tls.Certificate) (*NFSeQueryResult, error)

	// LookupDPS looks up an NFS-e by its 42-character DPS identifier.
	// The certificate parameter is used for mTLS authentication.
	// Note: Actor restriction applies - only the provider who submitted the DPS can look it up.
	// Returns ErrDPSNotFound if the DPS does not exist, ErrForbidden if access is denied.
	LookupDPS(ctx context.Context, dpsID string, cert *tls.Certificate) (*DPSLookupResult, error)

	// CheckDPSExists checks whether a DPS with the given ID exists.
	// This uses a HEAD request which has no actor restriction (any valid certificate works).
	// Returns true if the DPS exists, false if it does not.
	CheckDPSExists(ctx context.Context, dpsID string, cert *tls.Certificate) (bool, error)

	// QueryEvents retrieves all events (cancellations, substitutions, etc.) for an NFS-e.
	// The certificate parameter is used for mTLS authentication.
	// Returns an empty event list (not an error) if the NFS-e has no events.
	// Returns ErrNFSeNotFound if the NFS-e does not exist.
	QueryEvents(ctx context.Context, chaveAcesso string, cert *tls.Certificate) (*EventsQueryResult, error)
}

// SefinResponse represents the response from a SEFIN API call.
type SefinResponse struct {
	// Success indicates whether the submission was successful.
	Success bool

	// ChaveAcesso is the 50-character access key for the NFS-e (only on success).
	ChaveAcesso string

	// NFSeNumber is the official NFS-e number assigned by SEFIN (only on success).
	NFSeNumber string

	// NFSeXML is the signed NFS-e XML returned by SEFIN (only on success).
	NFSeXML string

	// ErrorCode is the error code returned by SEFIN (only on failure).
	ErrorCode string

	// ErrorMessage is the error message returned by SEFIN (only on failure).
	ErrorMessage string

	// ErrorCodes contains multiple error codes if the submission had multiple issues.
	ErrorCodes []string

	// ProtocolNumber is the SEFIN protocol number for the submission.
	ProtocolNumber string

	// ProcessingTime is how long SEFIN took to process the request.
	ProcessingTime time.Duration

	// RawResponse is the raw XML response from SEFIN (for debugging).
	RawResponse string
}

// ================================================================================
// Query Result Types
// ================================================================================

// NFSeQueryResult holds the result of querying an NFS-e by access key.
type NFSeQueryResult struct {
	// ChaveAcesso is the 50-character NFS-e access key.
	ChaveAcesso string

	// Numero is the official NFS-e number assigned by the government.
	Numero string

	// DataEmissao is the emission date/time.
	DataEmissao time.Time

	// Status indicates the NFS-e status (active, cancelled, substituted).
	Status string

	// XML contains the complete signed NFS-e XML document.
	XML string

	// Prestador contains the service provider data parsed from the XML.
	Prestador PrestadorData

	// Tomador contains the service taker data parsed from the XML.
	// May be nil for anonymous consumer services.
	Tomador *TomadorData

	// Servico contains the service details parsed from the XML.
	Servico ServicoData

	// Valores contains the monetary values parsed from the XML.
	Valores ValoresData
}

// PrestadorData contains service provider information from an NFS-e query.
type PrestadorData struct {
	// Documento is the provider's tax identification (CNPJ).
	Documento string

	// Nome is the provider's legal name (razao social).
	Nome string

	// Municipio is the provider's municipality name.
	Municipio string

	// MunicipioCodigo is the 7-digit IBGE municipality code.
	MunicipioCodigo string
}

// TomadorData contains service taker information from an NFS-e query.
type TomadorData struct {
	// Documento is the taker's identification (CNPJ, CPF, or NIF).
	Documento string

	// TipoDocumento indicates the document type: "cnpj", "cpf", or "nif".
	TipoDocumento string

	// Nome is the taker's name.
	Nome string
}

// ServicoData contains service information from an NFS-e query.
type ServicoData struct {
	// CodigoNacional is the 6-digit national service code (cTribNac).
	CodigoNacional string

	// Descricao is the service description.
	Descricao string

	// LocalPrestacao is the location where the service was provided.
	LocalPrestacao string

	// MunicipioCodigo is the 7-digit IBGE code of the service location.
	MunicipioCodigo string
}

// ValoresData contains monetary values from an NFS-e query.
type ValoresData struct {
	// ValorServico is the gross service value.
	ValorServico float64

	// BaseCalculo is the ISS tax calculation base.
	BaseCalculo float64

	// Aliquota is the ISS tax rate as a percentage (e.g., 5.00 for 5%).
	Aliquota float64

	// ValorISSQN is the calculated ISS tax amount.
	ValorISSQN float64

	// ValorLiquido is the net value after taxes and deductions.
	ValorLiquido float64
}

// DPSLookupResult holds the result of looking up an NFS-e by DPS identifier.
type DPSLookupResult struct {
	// DPSID is the 42-character DPS identifier that was queried.
	DPSID string

	// ChaveAcesso is the 50-character access key of the corresponding NFS-e.
	ChaveAcesso string
}

// EventsQueryResult holds the result of querying events for an NFS-e.
type EventsQueryResult struct {
	// ChaveAcesso is the 50-character access key of the NFS-e.
	ChaveAcesso string

	// Events contains the list of events in chronological order.
	Events []EventData
}

// EventData contains information about a single NFS-e event.
type EventData struct {
	// Tipo is the event type code (e.g., "e101101" for cancellation).
	Tipo string

	// Descricao is a human-readable description of the event.
	Descricao string

	// Sequencia is the sequential event number for the NFS-e.
	Sequencia int

	// Data is the event timestamp.
	Data time.Time

	// XML contains the complete signed event XML document.
	XML string
}

// ================================================================================
// Error Types
// ================================================================================

// ErrNFSeNotFound is returned when the requested NFS-e does not exist.
var ErrNFSeNotFound = fmt.Errorf("nfse not found")

// ErrDPSNotFound is returned when the requested DPS does not exist.
var ErrDPSNotFound = fmt.Errorf("dps not found")

// ErrForbidden is returned when the caller does not have permission to access the resource.
var ErrForbidden = fmt.Errorf("forbidden: actor restriction applies")

// ErrServiceUnavailable is returned when the government API is temporarily unavailable.
var ErrServiceUnavailable = fmt.Errorf("government api temporarily unavailable")

// ErrTimeout is returned when the request to the government API times out.
var ErrTimeout = fmt.Errorf("request timeout")

// ClientConfig configures the SEFIN client.
type ClientConfig struct {
	// BaseURL is the SEFIN API base URL.
	BaseURL string

	// Environment is "producao" or "homologacao".
	Environment string

	// Timeout is the request timeout.
	Timeout time.Duration

	// CertPath is the path to the client certificate (PFX).
	CertPath string

	// CertPassword is the certificate password.
	CertPassword string

	// Certificate is the loaded TLS certificate for mTLS.
	Certificate *tls.Certificate

	// InsecureSkipVerify disables TLS verification (only for testing).
	InsecureSkipVerify bool

	// Logger is an optional logger for debugging.
	Logger *log.Logger

	// MaxRetries is the maximum number of retry attempts for transient failures.
	MaxRetries int

	// RetryDelay is the initial delay between retries (doubles each retry).
	RetryDelay time.Duration
}

// ProductionClient implements SefinClient for actual SEFIN API calls.
type ProductionClient struct {
	httpClient  *http.Client
	baseURL     string
	environment int // 1=production, 2=homologation
	timeout     time.Duration
	logger      *log.Logger
	maxRetries  int
	retryDelay  time.Duration
}

// NewProductionClient creates a new SEFIN client for real API calls.
func NewProductionClient(config ClientConfig) (*ProductionClient, error) {
	// Validate configuration
	if config.BaseURL == "" {
		// Set default based on environment
		if config.Environment == EnvironmentProduction {
			config.BaseURL = ProductionBaseURL
		} else {
			config.BaseURL = HomologationBaseURL
		}
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	if config.RetryDelay == 0 {
		config.RetryDelay = 2 * time.Second
	}

	// Configure TLS
	tlsConfig := &tls.Config{
		InsecureSkipVerify: config.InsecureSkipVerify,
		MinVersion:         tls.VersionTLS12,
	}

	// Add client certificate for mTLS if provided
	if config.Certificate != nil {
		tlsConfig.Certificates = []tls.Certificate{*config.Certificate}
	}

	// Create HTTP transport with TLS config
	transport := &http.Transport{
		TLSClientConfig:     tlsConfig,
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
	}

	// Create HTTP client
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}

	// Determine environment code
	envCode := 2 // Default to homologation
	if config.Environment == EnvironmentProduction {
		envCode = 1
	}

	return &ProductionClient{
		httpClient:  httpClient,
		baseURL:     config.BaseURL,
		environment: envCode,
		timeout:     config.Timeout,
		logger:      config.Logger,
		maxRetries:  config.MaxRetries,
		retryDelay:  config.RetryDelay,
	}, nil
}

// SubmitDPS submits a DPS to the SEFIN API.
func (c *ProductionClient) SubmitDPS(ctx context.Context, dpsXML string, environment string) (*SefinResponse, error) {
	start := time.Now()

	// Build SOAP envelope
	soapEnvelope := c.buildSOAPEnvelope(dpsXML)

	// Execute with retries
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate backoff delay
			delay := c.retryDelay * time.Duration(1<<uint(attempt-1))
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}

			c.logDebug("Retrying SEFIN submission (attempt %d/%d) after %v", attempt+1, c.maxRetries+1, delay)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		response, err := c.doSubmit(ctx, soapEnvelope)
		if err != nil {
			lastErr = err

			// Check if error is retryable
			if !c.isRetryableError(err) {
				return nil, fmt.Errorf("sefin submission failed: %w", err)
			}

			c.logDebug("SEFIN submission attempt %d failed: %v", attempt+1, err)
			continue
		}

		response.ProcessingTime = time.Since(start)
		return response, nil
	}

	return nil, fmt.Errorf("sefin submission failed after %d attempts: %w", c.maxRetries+1, lastErr)
}

// doSubmit performs a single submission attempt.
func (c *ProductionClient) doSubmit(ctx context.Context, soapEnvelope string) (*SefinResponse, error) {
	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, strings.NewReader(soapEnvelope))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", SOAPActionDPS)
	req.Header.Set("Accept", "text/xml")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	rawResponse := string(bodyBytes)
	c.logDebug("SEFIN response (status %d): %s", resp.StatusCode, rawResponse)

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}

	// Parse SOAP response
	return c.parseSOAPResponse(rawResponse)
}

// buildSOAPEnvelope wraps the DPS XML in a SOAP envelope.
func (c *ProductionClient) buildSOAPEnvelope(dpsXML string) string {
	// The actual SOAP structure depends on government WSDL specification.
	// This is a reasonable implementation based on Brazilian fiscal API patterns.
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope"
               xmlns:nfse="http://www.sped.fazenda.gov.br/nfse">
  <soap:Header/>
  <soap:Body>
    <nfse:RecepcionarDPSRequest>
      <nfse:versaoDados>1.00</nfse:versaoDados>
      <nfse:nfseDADOS>
        <![CDATA[%s]]>
      </nfse:nfseDADOS>
    </nfse:RecepcionarDPSRequest>
  </soap:Body>
</soap:Envelope>`, dpsXML)
}

// SOAPEnvelope represents a SOAP response envelope.
type SOAPEnvelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    SOAPBody `xml:"Body"`
}

// SOAPBody represents the SOAP body element.
type SOAPBody struct {
	Response RecepcionarDPSResponse `xml:"RecepcionarDPSResponse"`
	Fault    *SOAPFault             `xml:"Fault,omitempty"`
}

// SOAPFault represents a SOAP fault.
type SOAPFault struct {
	Code   string `xml:"Code>Value"`
	Reason string `xml:"Reason>Text"`
	Detail string `xml:"Detail,omitempty"`
}

// RecepcionarDPSResponse represents the DPS reception response.
type RecepcionarDPSResponse struct {
	Resultado ResultadoDPS `xml:"resultado"`
}

// ResultadoDPS represents the DPS processing result.
type ResultadoDPS struct {
	// Sucesso indicates if the submission was successful.
	Sucesso bool `xml:"sucesso"`

	// Protocolo is the protocol number.
	Protocolo string `xml:"protocolo,omitempty"`

	// NFSe contains the generated NFS-e data.
	NFSe *NFSeResult `xml:"NFSe,omitempty"`

	// Erros contains validation/processing errors.
	Erros []ErroNFSe `xml:"erros>erro,omitempty"`
}

// NFSeResult represents the generated NFS-e.
type NFSeResult struct {
	// ChaveAcesso is the 50-character access key.
	ChaveAcesso string `xml:"chaveAcesso"`

	// Numero is the NFS-e number.
	Numero string `xml:"nNFSe"`

	// XML is the complete signed NFS-e XML.
	XML string `xml:"xmlNFSe"`
}

// ErroNFSe represents an error returned by SEFIN.
type ErroNFSe struct {
	// Codigo is the error code (e.g., "E001").
	Codigo string `xml:"codigo"`

	// Mensagem is the error message.
	Mensagem string `xml:"mensagem"`
}

// parseSOAPResponse parses the SOAP response XML.
func (c *ProductionClient) parseSOAPResponse(rawXML string) (*SefinResponse, error) {
	response := &SefinResponse{
		RawResponse: rawXML,
	}

	// Parse SOAP envelope
	var envelope SOAPEnvelope
	decoder := xml.NewDecoder(bytes.NewReader([]byte(rawXML)))

	if err := decoder.Decode(&envelope); err != nil {
		// If SOAP parsing fails, try to extract error information directly
		return c.parseRawResponse(rawXML)
	}

	// Check for SOAP fault
	if envelope.Body.Fault != nil {
		response.Success = false
		response.ErrorCode = envelope.Body.Fault.Code
		response.ErrorMessage = envelope.Body.Fault.Reason
		return response, nil
	}

	// Parse DPS response
	result := envelope.Body.Response.Resultado

	response.Success = result.Sucesso
	response.ProtocolNumber = result.Protocolo

	if result.Sucesso && result.NFSe != nil {
		response.ChaveAcesso = result.NFSe.ChaveAcesso
		response.NFSeNumber = result.NFSe.Numero
		response.NFSeXML = result.NFSe.XML
	}

	// Parse errors if any
	if len(result.Erros) > 0 {
		response.Success = false
		codes := make([]string, len(result.Erros))
		messages := make([]string, len(result.Erros))

		for i, erro := range result.Erros {
			codes[i] = erro.Codigo
			messages[i] = fmt.Sprintf("%s: %s", erro.Codigo, erro.Mensagem)
		}

		response.ErrorCodes = codes
		response.ErrorCode = codes[0]
		response.ErrorMessage = strings.Join(messages, "; ")
	}

	return response, nil
}

// parseRawResponse attempts to parse error information from raw response.
func (c *ProductionClient) parseRawResponse(rawXML string) (*SefinResponse, error) {
	response := &SefinResponse{
		Success:     false,
		RawResponse: rawXML,
	}

	// Look for common error patterns in the XML
	if strings.Contains(rawXML, "<sucesso>true</sucesso>") {
		response.Success = true
	}

	// Extract protocol number if present
	if idx := strings.Index(rawXML, "<protocolo>"); idx != -1 {
		end := strings.Index(rawXML[idx:], "</protocolo>")
		if end != -1 {
			response.ProtocolNumber = rawXML[idx+11 : idx+end]
		}
	}

	// If we couldn't parse successfully, return with error
	if !response.Success && response.ErrorMessage == "" {
		response.ErrorMessage = "Failed to parse government response"
	}

	return response, nil
}

// isRetryableError determines if an error warrants a retry.
func (c *ProductionClient) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Network errors are retryable
	retryablePatterns := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary failure",
		"i/o timeout",
		"EOF",
		"no such host",
		"server misbehaving",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errStr), strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// logDebug logs a debug message if logger is configured.
func (c *ProductionClient) logDebug(format string, args ...interface{}) {
	if c.logger != nil {
		c.logger.Printf(format, args...)
	}
}

// SetLogger sets the logger for the client.
func (c *ProductionClient) SetLogger(logger *log.Logger) {
	c.logger = logger
}

// GetBaseURL returns the configured base URL.
func (c *ProductionClient) GetBaseURL() string {
	return c.baseURL
}

// GetEnvironment returns the environment code (1=production, 2=homologation).
func (c *ProductionClient) GetEnvironment() int {
	return c.environment
}

// ================================================================================
// Query Methods for ProductionClient
// ================================================================================

// createQueryHTTPClient creates a new HTTP client configured for query operations.
// Each query creates its own client to support stateless design with per-request certificates.
func (c *ProductionClient) createQueryHTTPClient(cert *tls.Certificate) *http.Client {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if cert != nil {
		tlsConfig.Certificates = []tls.Certificate{*cert}
	}

	transport := &http.Transport{
		TLSClientConfig:     tlsConfig,
		MaxIdleConns:        1,
		MaxIdleConnsPerHost: 1,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  false,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   QueryTimeout,
	}
}

// QueryNFSe retrieves an NFS-e by its access key from the government API.
func (c *ProductionClient) QueryNFSe(ctx context.Context, chaveAcesso string, cert *tls.Certificate) (*NFSeQueryResult, error) {
	if chaveAcesso == "" {
		return nil, fmt.Errorf("chaveAcesso is required")
	}

	// Build request URL
	url := fmt.Sprintf("%s/nfse/%s", c.baseURL, chaveAcesso)

	c.logDebug("QueryNFSe: requesting %s", url)

	// Create HTTP client with provided certificate
	httpClient := c.createQueryHTTPClient(cert)

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := httpClient.Do(req)
	if err != nil {
		// Check for timeout
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "timeout") {
			return nil, ErrTimeout
		}
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	c.logDebug("QueryNFSe: response status=%d body=%s", resp.StatusCode, string(bodyBytes))

	// Handle response status codes
	switch resp.StatusCode {
	case http.StatusOK:
		// Parse successful response
		return c.parseNFSeQueryResponse(bodyBytes)
	case http.StatusNotFound:
		return nil, ErrNFSeNotFound
	case http.StatusServiceUnavailable:
		return nil, ErrServiceUnavailable
	default:
		return nil, fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}
}

// nfseJSONResponse represents the JSON response from the government API for NFS-e queries.
type nfseJSONResponse struct {
	ChaveAcesso string `json:"chaveAcesso"`
	Numero      string `json:"numero"`
	DataEmissao string `json:"dataEmissao"`
	Status      string `json:"status"`
	XML         string `json:"xml"`
	Prestador   struct {
		Documento       string `json:"documento"`
		Nome            string `json:"nome"`
		Municipio       string `json:"municipio"`
		MunicipioCodigo string `json:"municipioCodigo"`
	} `json:"prestador"`
	Tomador *struct {
		Documento     string `json:"documento"`
		TipoDocumento string `json:"tipoDocumento"`
		Nome          string `json:"nome"`
	} `json:"tomador,omitempty"`
	Servico struct {
		CodigoNacional  string `json:"codigoNacional"`
		Descricao       string `json:"descricao"`
		LocalPrestacao  string `json:"localPrestacao"`
		MunicipioCodigo string `json:"municipioCodigo"`
	} `json:"servico"`
	Valores struct {
		ValorServico float64 `json:"valorServico"`
		BaseCalculo  float64 `json:"baseCalculo"`
		Aliquota     float64 `json:"aliquota"`
		ValorISSQN   float64 `json:"valorISSQN"`
		ValorLiquido float64 `json:"valorLiquido"`
	} `json:"valores"`
}

// parseNFSeQueryResponse parses the JSON response from the NFS-e query API.
func (c *ProductionClient) parseNFSeQueryResponse(body []byte) (*NFSeQueryResult, error) {
	var jsonResp nfseJSONResponse
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Parse the emission date
	dataEmissao, err := time.Parse(time.RFC3339, jsonResp.DataEmissao)
	if err != nil {
		// Try alternative date format
		dataEmissao, err = time.Parse("2006-01-02T15:04:05-07:00", jsonResp.DataEmissao)
		if err != nil {
			c.logDebug("QueryNFSe: failed to parse date %s: %v", jsonResp.DataEmissao, err)
			dataEmissao = time.Time{}
		}
	}

	result := &NFSeQueryResult{
		ChaveAcesso: jsonResp.ChaveAcesso,
		Numero:      jsonResp.Numero,
		DataEmissao: dataEmissao,
		Status:      jsonResp.Status,
		XML:         jsonResp.XML,
		Prestador: PrestadorData{
			Documento:       jsonResp.Prestador.Documento,
			Nome:            jsonResp.Prestador.Nome,
			Municipio:       jsonResp.Prestador.Municipio,
			MunicipioCodigo: jsonResp.Prestador.MunicipioCodigo,
		},
		Servico: ServicoData{
			CodigoNacional:  jsonResp.Servico.CodigoNacional,
			Descricao:       jsonResp.Servico.Descricao,
			LocalPrestacao:  jsonResp.Servico.LocalPrestacao,
			MunicipioCodigo: jsonResp.Servico.MunicipioCodigo,
		},
		Valores: ValoresData{
			ValorServico: jsonResp.Valores.ValorServico,
			BaseCalculo:  jsonResp.Valores.BaseCalculo,
			Aliquota:     jsonResp.Valores.Aliquota,
			ValorISSQN:   jsonResp.Valores.ValorISSQN,
			ValorLiquido: jsonResp.Valores.ValorLiquido,
		},
	}

	// Add tomador if present
	if jsonResp.Tomador != nil {
		result.Tomador = &TomadorData{
			Documento:     jsonResp.Tomador.Documento,
			TipoDocumento: jsonResp.Tomador.TipoDocumento,
			Nome:          jsonResp.Tomador.Nome,
		}
	}

	return result, nil
}

// LookupDPS looks up an NFS-e by its DPS identifier.
// Note: Actor restriction applies - only the provider who submitted the DPS can look it up.
func (c *ProductionClient) LookupDPS(ctx context.Context, dpsID string, cert *tls.Certificate) (*DPSLookupResult, error) {
	if dpsID == "" {
		return nil, fmt.Errorf("dpsID is required")
	}

	// Build request URL
	url := fmt.Sprintf("%s/dps/%s", c.baseURL, dpsID)

	c.logDebug("LookupDPS: requesting %s", url)

	// Create HTTP client with provided certificate
	httpClient := c.createQueryHTTPClient(cert)

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "timeout") {
			return nil, ErrTimeout
		}
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	c.logDebug("LookupDPS: response status=%d body=%s", resp.StatusCode, string(bodyBytes))

	// Handle response status codes
	switch resp.StatusCode {
	case http.StatusOK:
		return c.parseDPSLookupResponse(dpsID, bodyBytes)
	case http.StatusNotFound:
		return nil, ErrDPSNotFound
	case http.StatusForbidden:
		return nil, ErrForbidden
	case http.StatusServiceUnavailable:
		return nil, ErrServiceUnavailable
	default:
		return nil, fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}
}

// dpsLookupJSONResponse represents the JSON response from the DPS lookup API.
type dpsLookupJSONResponse struct {
	ChaveAcesso string `json:"chaveAcesso"`
}

// parseDPSLookupResponse parses the JSON response from the DPS lookup API.
func (c *ProductionClient) parseDPSLookupResponse(dpsID string, body []byte) (*DPSLookupResult, error) {
	var jsonResp dpsLookupJSONResponse
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return &DPSLookupResult{
		DPSID:       dpsID,
		ChaveAcesso: jsonResp.ChaveAcesso,
	}, nil
}

// CheckDPSExists checks whether a DPS with the given ID exists.
// Uses a HEAD request which has no actor restriction (any valid certificate works).
func (c *ProductionClient) CheckDPSExists(ctx context.Context, dpsID string, cert *tls.Certificate) (bool, error) {
	if dpsID == "" {
		return false, fmt.Errorf("dpsID is required")
	}

	// Build request URL
	url := fmt.Sprintf("%s/dps/%s", c.baseURL, dpsID)

	c.logDebug("CheckDPSExists: requesting HEAD %s", url)

	// Create HTTP client with provided certificate
	httpClient := c.createQueryHTTPClient(cert)

	// Create HEAD request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request
	resp, err := httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "timeout") {
			return false, ErrTimeout
		}
		return false, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	c.logDebug("CheckDPSExists: response status=%d", resp.StatusCode)

	// Handle response status codes
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	case http.StatusServiceUnavailable:
		return false, ErrServiceUnavailable
	default:
		return false, fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}
}

// QueryEvents retrieves all events for an NFS-e by its access key.
// Returns an empty event list (not an error) if the NFS-e has no events.
func (c *ProductionClient) QueryEvents(ctx context.Context, chaveAcesso string, cert *tls.Certificate) (*EventsQueryResult, error) {
	if chaveAcesso == "" {
		return nil, fmt.Errorf("chaveAcesso is required")
	}

	// Build request URL
	url := fmt.Sprintf("%s/nfse/%s/eventos", c.baseURL, chaveAcesso)

	c.logDebug("QueryEvents: requesting %s", url)

	// Create HTTP client with provided certificate
	httpClient := c.createQueryHTTPClient(cert)

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "timeout") {
			return nil, ErrTimeout
		}
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	c.logDebug("QueryEvents: response status=%d body=%s", resp.StatusCode, string(bodyBytes))

	// Handle response status codes
	switch resp.StatusCode {
	case http.StatusOK:
		return c.parseEventsQueryResponse(chaveAcesso, bodyBytes)
	case http.StatusNotFound:
		return nil, ErrNFSeNotFound
	case http.StatusServiceUnavailable:
		return nil, ErrServiceUnavailable
	default:
		return nil, fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}
}

// eventsJSONResponse represents the JSON response from the events query API.
type eventsJSONResponse struct {
	Events []struct {
		Tipo      string `json:"tipo"`
		Descricao string `json:"descricao"`
		Sequencia int    `json:"sequencia"`
		Data      string `json:"data"`
		XML       string `json:"xml"`
	} `json:"eventos"`
}

// parseEventsQueryResponse parses the JSON response from the events query API.
func (c *ProductionClient) parseEventsQueryResponse(chaveAcesso string, body []byte) (*EventsQueryResult, error) {
	var jsonResp eventsJSONResponse
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	result := &EventsQueryResult{
		ChaveAcesso: chaveAcesso,
		Events:      make([]EventData, 0, len(jsonResp.Events)),
	}

	for _, evt := range jsonResp.Events {
		// Parse event date
		eventDate, err := time.Parse(time.RFC3339, evt.Data)
		if err != nil {
			// Try alternative format
			eventDate, err = time.Parse("2006-01-02T15:04:05-07:00", evt.Data)
			if err != nil {
				c.logDebug("QueryEvents: failed to parse date %s: %v", evt.Data, err)
				eventDate = time.Time{}
			}
		}

		result.Events = append(result.Events, EventData{
			Tipo:      evt.Tipo,
			Descricao: evt.Descricao,
			Sequencia: evt.Sequencia,
			Data:      eventDate,
			XML:       evt.XML,
		})
	}

	return result, nil
}

// MockClient implements SefinClient with mock responses for development and testing.
type MockClient struct {
	// SimulateFailure controls whether the mock should return failures.
	SimulateFailure bool

	// FailureRate is the percentage of requests that should fail (0-100).
	FailureRate int

	// SimulatedLatency adds artificial latency to responses.
	SimulatedLatency time.Duration
}

// NewMockClient creates a new mock SEFIN client for development.
func NewMockClient() *MockClient {
	return &MockClient{
		SimulateFailure:  false,
		FailureRate:      0,
		SimulatedLatency: 500 * time.Millisecond,
	}
}

// SubmitDPS simulates submitting a DPS to the SEFIN API.
func (c *MockClient) SubmitDPS(ctx context.Context, dpsXML string, environment string) (*SefinResponse, error) {
	start := time.Now()

	// Add simulated latency
	if c.SimulatedLatency > 0 {
		select {
		case <-time.After(c.SimulatedLatency):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Check for simulated failure
	if c.SimulateFailure || c.shouldFail() {
		return &SefinResponse{
			Success:        false,
			ErrorCode:      "E001",
			ErrorMessage:   "Mock rejection: Simulated government API failure",
			ProcessingTime: time.Since(start),
		}, nil
	}

	// Generate mock success response
	nfseNumber := generateMockNFSeNumber()
	chaveAcesso := generateMockChaveAcesso(environment)
	protocolNumber := generateMockProtocolNumber()

	return &SefinResponse{
		Success:        true,
		ChaveAcesso:    chaveAcesso,
		NFSeNumber:     nfseNumber,
		NFSeXML:        generateMockNFSeXML(dpsXML, nfseNumber, chaveAcesso),
		ProtocolNumber: protocolNumber,
		ProcessingTime: time.Since(start),
	}, nil
}

// shouldFail determines if this request should fail based on failure rate.
func (c *MockClient) shouldFail() bool {
	if c.FailureRate <= 0 {
		return false
	}
	if c.FailureRate >= 100 {
		return true
	}

	n, err := rand.Int(rand.Reader, big.NewInt(100))
	if err != nil {
		return false
	}

	return n.Int64() < int64(c.FailureRate)
}

// generateMockNFSeNumber generates a mock NFS-e number.
func generateMockNFSeNumber() string {
	n, err := rand.Int(rand.Reader, big.NewInt(999999999))
	if err != nil {
		return "000000001"
	}
	return fmt.Sprintf("%09d", n.Int64()+1)
}

// generateMockChaveAcesso generates a mock 50-character access key.
func generateMockChaveAcesso(environment string) string {
	// Format: NFSe + UF(2) + AAMM(4) + CNPJ(14) + Mod(2) + Serie(5) + Num(15) + CodVer(9) + CodNum(9) + DV(1)
	// This is a simplified mock format
	timestamp := time.Now().Format("0601")
	n, err := rand.Int(rand.Reader, big.NewInt(999999999999999))
	if err != nil {
		n = big.NewInt(123456789012345)
	}

	prefix := "NFSe"
	uf := "35" // Sao Paulo
	cnpj := "12345678000199"
	mod := "99"
	serie := "00001"
	num := fmt.Sprintf("%015d", n.Int64())
	codVer := "123456789"
	codNum := "987654321"
	dv := "0"

	return prefix + uf + timestamp + cnpj + mod + serie + num[:15] + codVer + codNum + dv
}

// generateMockProtocolNumber generates a mock protocol number.
func generateMockProtocolNumber() string {
	timestamp := time.Now().Format("20060102150405")
	n, err := rand.Int(rand.Reader, big.NewInt(999999))
	if err != nil {
		n = big.NewInt(123456)
	}
	return fmt.Sprintf("%s%06d", timestamp, n.Int64())
}

// generateMockNFSeXML generates a mock NFS-e XML response.
func generateMockNFSeXML(dpsXML, nfseNumber, chaveAcesso string) string {
	timestamp := time.Now().Format("2006-01-02T15:04:05-03:00")

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<NFSe xmlns="http://www.sped.fazenda.gov.br/nfse" versao="1.00">
  <infNFSe Id="%s">
    <nNFSe>%s</nNFSe>
    <dhEmi>%s</dhEmi>
    <chNFSe>%s</chNFSe>
    <sit>1</sit>
  </infNFSe>
</NFSe>`, chaveAcesso, nfseNumber, timestamp, chaveAcesso)
}

// Environment constants.
const (
	// EnvironmentProduction is the production environment.
	EnvironmentProduction = "producao"

	// EnvironmentHomologation is the homologation (testing) environment.
	EnvironmentHomologation = "homologacao"
)

// ================================================================================
// Mock Query Methods
// ================================================================================

// QueryNFSe returns a mock NFS-e query result.
// The certificate parameter is ignored in the mock implementation.
func (c *MockClient) QueryNFSe(ctx context.Context, chaveAcesso string, cert *tls.Certificate) (*NFSeQueryResult, error) {
	// Add simulated latency
	if c.SimulatedLatency > 0 {
		select {
		case <-time.After(c.SimulatedLatency):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Simulate failure if configured
	if c.SimulateFailure || c.shouldFail() {
		return nil, ErrServiceUnavailable
	}

	// Return not found for specific test keys
	if strings.HasPrefix(chaveAcesso, "NOTFOUND") {
		return nil, ErrNFSeNotFound
	}

	// Generate mock response
	return &NFSeQueryResult{
		ChaveAcesso: chaveAcesso,
		Numero:      generateMockNFSeNumber(),
		DataEmissao: time.Now().Add(-24 * time.Hour),
		Status:      "active",
		XML:         generateMockNFSeXML("", "000000001", chaveAcesso),
		Prestador: PrestadorData{
			Documento:       "12345678000199",
			Nome:            "Empresa Mock Ltda",
			Municipio:       "Sao Paulo",
			MunicipioCodigo: "3550308",
		},
		Tomador: &TomadorData{
			Documento:     "98765432000188",
			TipoDocumento: "cnpj",
			Nome:          "Cliente Mock S.A.",
		},
		Servico: ServicoData{
			CodigoNacional:  "010201",
			Descricao:       "Servico de consultoria em tecnologia da informacao",
			LocalPrestacao:  "Sao Paulo - SP",
			MunicipioCodigo: "3550308",
		},
		Valores: ValoresData{
			ValorServico: 1000.00,
			BaseCalculo:  1000.00,
			Aliquota:     5.00,
			ValorISSQN:   50.00,
			ValorLiquido: 950.00,
		},
	}, nil
}

// LookupDPS returns a mock DPS lookup result.
// The certificate parameter is ignored in the mock implementation.
func (c *MockClient) LookupDPS(ctx context.Context, dpsID string, cert *tls.Certificate) (*DPSLookupResult, error) {
	// Add simulated latency
	if c.SimulatedLatency > 0 {
		select {
		case <-time.After(c.SimulatedLatency):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Simulate failure if configured
	if c.SimulateFailure || c.shouldFail() {
		return nil, ErrServiceUnavailable
	}

	// Return not found for specific test IDs
	if strings.HasPrefix(dpsID, "NOTFOUND") {
		return nil, ErrDPSNotFound
	}

	// Return forbidden for specific test IDs (actor restriction)
	if strings.HasPrefix(dpsID, "FORBIDDEN") {
		return nil, ErrForbidden
	}

	// Generate mock access key from DPS ID
	mockChaveAcesso := generateMockChaveAcesso("homologacao")

	return &DPSLookupResult{
		DPSID:       dpsID,
		ChaveAcesso: mockChaveAcesso,
	}, nil
}

// CheckDPSExists returns whether a mock DPS exists.
// The certificate parameter is ignored in the mock implementation.
func (c *MockClient) CheckDPSExists(ctx context.Context, dpsID string, cert *tls.Certificate) (bool, error) {
	// Add simulated latency (shorter for HEAD requests)
	if c.SimulatedLatency > 0 {
		latency := c.SimulatedLatency / 2
		select {
		case <-time.After(latency):
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}

	// Simulate failure if configured
	if c.SimulateFailure || c.shouldFail() {
		return false, ErrServiceUnavailable
	}

	// Return false for specific test IDs
	if strings.HasPrefix(dpsID, "NOTFOUND") {
		return false, nil
	}

	// Default: DPS exists
	return true, nil
}

// QueryEvents returns mock events for an NFS-e.
// The certificate parameter is ignored in the mock implementation.
func (c *MockClient) QueryEvents(ctx context.Context, chaveAcesso string, cert *tls.Certificate) (*EventsQueryResult, error) {
	// Add simulated latency
	if c.SimulatedLatency > 0 {
		select {
		case <-time.After(c.SimulatedLatency):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Simulate failure if configured
	if c.SimulateFailure || c.shouldFail() {
		return nil, ErrServiceUnavailable
	}

	// Return not found for specific test keys
	if strings.HasPrefix(chaveAcesso, "NOTFOUND") {
		return nil, ErrNFSeNotFound
	}

	// Return empty events for keys starting with "NOEVENTS"
	if strings.HasPrefix(chaveAcesso, "NOEVENTS") {
		return &EventsQueryResult{
			ChaveAcesso: chaveAcesso,
			Events:      []EventData{},
		}, nil
	}

	// Return mock events including emission and potentially cancellation
	events := []EventData{
		{
			Tipo:      "EMISSAO",
			Descricao: "NFS-e emitida",
			Sequencia: 1,
			Data:      time.Now().Add(-24 * time.Hour),
			XML:       generateMockEventXML("EMISSAO", chaveAcesso, 1),
		},
	}

	// Add cancellation event for keys starting with "CANCELLED"
	if strings.HasPrefix(chaveAcesso, "CANCELLED") {
		events = append(events, EventData{
			Tipo:      "e101101",
			Descricao: "Cancelamento de NFS-e",
			Sequencia: 2,
			Data:      time.Now().Add(-12 * time.Hour),
			XML:       generateMockEventXML("e101101", chaveAcesso, 2),
		})
	}

	return &EventsQueryResult{
		ChaveAcesso: chaveAcesso,
		Events:      events,
	}, nil
}

// generateMockEventXML generates a mock event XML document.
func generateMockEventXML(eventType, chaveAcesso string, sequencia int) string {
	timestamp := time.Now().Format("2006-01-02T15:04:05-03:00")

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<evento xmlns="http://www.sped.fazenda.gov.br/nfse" versao="1.00">
  <infEvento Id="EVT%s%d">
    <chNFSe>%s</chNFSe>
    <tpEvento>%s</tpEvento>
    <nSeqEvento>%d</nSeqEvento>
    <dhEvento>%s</dhEvento>
  </infEvento>
</evento>`, chaveAcesso, sequencia, chaveAcesso, eventType, sequencia, timestamp)
}
