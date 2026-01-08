// Package sefin provides client implementations for communicating with
// the SEFIN (Secretaria da Fazenda) government API for the Sistema Nacional NFS-e.
package sefin

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/xml"
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

// SefinClient defines the interface for interacting with the SEFIN API.
type SefinClient interface {
	// SubmitDPS submits a DPS XML document for processing and returns the result.
	SubmitDPS(ctx context.Context, dpsXML string, environment string) (*SefinResponse, error)
}

// SefinResponse represents the response from a SEFIN API call.
type SefinResponse struct {
	// Success indicates whether the submission was successful.
	Success bool

	// ChaveAcesso is the 66-character access key for the NFS-e (only on success).
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
	// ChaveAcesso is the 66-character access key.
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

// generateMockChaveAcesso generates a mock 66-character access key.
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
