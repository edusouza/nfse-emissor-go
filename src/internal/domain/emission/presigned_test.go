// Package emission provides DTOs and business logic for NFS-e emission operations.
package emission

import (
	"encoding/base64"
	"strings"
	"testing"
)

// Test XML documents for pre-signed parsing tests.
const (
	validSignedDPSXML = `<?xml version="1.0" encoding="UTF-8"?>
<DPS xmlns="http://www.sped.fazenda.gov.br/nfse">
  <infDPS Id="DPS12345678901234567890123456789012345678901234567890">
    <tpAmb>2</tpAmb>
    <dhEmi>2024-01-15T10:30:00-03:00</dhEmi>
    <verAplic>1.0.0</verAplic>
    <serie>00001</serie>
    <nDPS>123456</nDPS>
    <dCompet>2024-01-15</dCompet>
    <tpEmit>1</tpEmit>
    <cLocEmi>3550308</cLocEmi>
    <subst>2</subst>
    <prest>
      <CNPJ>12345678000190</CNPJ>
      <xNome>Provider Company Ltd</xNome>
    </prest>
    <serv>
      <cTribNac>010101</cTribNac>
      <xDescServ>Software development services</xDescServ>
      <cLocPrest>3550308</cLocPrest>
    </serv>
    <valores>
      <vServPrest>1000.00</vServPrest>
    </valores>
  </infDPS>
  <Signature xmlns="http://www.w3.org/2000/09/xmldsig#">
    <SignedInfo>
      <Reference URI="#DPS12345678901234567890123456789012345678901234567890">
        <DigestValue>test</DigestValue>
      </Reference>
    </SignedInfo>
    <SignatureValue>test</SignatureValue>
  </Signature>
</DPS>`

	validUnsignedDPSXML = `<?xml version="1.0" encoding="UTF-8"?>
<DPS xmlns="http://www.sped.fazenda.gov.br/nfse">
  <infDPS Id="DPS12345678901234567890123456789012345678901234567890">
    <tpAmb>1</tpAmb>
    <dhEmi>2024-01-15T10:30:00-03:00</dhEmi>
    <verAplic>1.0.0</verAplic>
    <serie>00002</serie>
    <nDPS>789012</nDPS>
    <dCompet>2024-01-15</dCompet>
    <tpEmit>1</tpEmit>
    <cLocEmi>3550308</cLocEmi>
    <subst>2</subst>
    <prest>
      <CNPJ>98765432000121</CNPJ>
      <xNome>Another Company</xNome>
    </prest>
    <serv>
      <cTribNac>020202</cTribNac>
      <xDescServ>Consulting services</xDescServ>
      <cLocPrest>3550308</cLocPrest>
    </serv>
    <valores>
      <vServPrest>2500.50</vServPrest>
    </valores>
  </infDPS>
</DPS>`

	dpsWithCPFXML = `<?xml version="1.0" encoding="UTF-8"?>
<DPS xmlns="http://www.sped.fazenda.gov.br/nfse">
  <infDPS Id="DPS12345">
    <tpAmb>2</tpAmb>
    <dhEmi>2024-01-15T10:30:00-03:00</dhEmi>
    <verAplic>1.0.0</verAplic>
    <serie>00001</serie>
    <nDPS>1</nDPS>
    <dCompet>2024-01-15</dCompet>
    <tpEmit>1</tpEmit>
    <cLocEmi>3550308</cLocEmi>
    <subst>2</subst>
    <prest>
      <CPF>12345678901</CPF>
      <xNome>Individual Provider</xNome>
    </prest>
    <serv>
      <cTribNac>010101</cTribNac>
      <xDescServ>Services</xDescServ>
      <cLocPrest>3550308</cLocPrest>
    </serv>
    <valores>
      <vServPrest>500.00</vServPrest>
    </valores>
  </infDPS>
  <Signature xmlns="http://www.w3.org/2000/09/xmldsig#">
    <SignedInfo><Reference URI="#DPS12345"><DigestValue>test</DigestValue></Reference></SignedInfo>
    <SignatureValue>test</SignatureValue>
  </Signature>
</DPS>`

	noDPSXML = `<?xml version="1.0"?>
<root>
  <element>value</element>
</root>`

	noInfDPSXML = `<?xml version="1.0"?>
<DPS xmlns="http://www.sped.fazenda.gov.br/nfse">
</DPS>`

	noIDAttributeXML = `<?xml version="1.0"?>
<DPS xmlns="http://www.sped.fazenda.gov.br/nfse">
  <infDPS>
    <tpAmb>2</tpAmb>
  </infDPS>
</DPS>`

	noProviderXML = `<?xml version="1.0"?>
<DPS xmlns="http://www.sped.fazenda.gov.br/nfse">
  <infDPS Id="DPS12345">
    <tpAmb>2</tpAmb>
  </infDPS>
</DPS>`

	noProviderIDXML = `<?xml version="1.0"?>
<DPS xmlns="http://www.sped.fazenda.gov.br/nfse">
  <infDPS Id="DPS12345">
    <tpAmb>2</tpAmb>
    <prest>
      <xNome>Name Only</xNome>
    </prest>
  </infDPS>
</DPS>`
)

func TestPreSignedXMLRequest_DecodeXML(t *testing.T) {
	tests := []struct {
		name        string
		xml         string
		expectError bool
	}{
		{
			name:        "valid base64",
			xml:         base64.StdEncoding.EncodeToString([]byte(validSignedDPSXML)),
			expectError: false,
		},
		{
			name:        "empty string",
			xml:         "",
			expectError: true,
		},
		{
			name:        "invalid base64",
			xml:         "not-valid-base64!@#$",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &PreSignedXMLRequest{XML: tt.xml}
			decoded, err := req.DecodeXML()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if decoded != validSignedDPSXML {
					t.Error("Decoded XML does not match original")
				}
			}
		})
	}
}

func TestParsePreSignedXML_ValidSignedDocument(t *testing.T) {
	info, err := ParsePreSignedXML(validSignedDPSXML)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if info.DPSID != "DPS12345678901234567890123456789012345678901234567890" {
		t.Errorf("Expected DPSID 'DPS12345678901234567890123456789012345678901234567890', got '%s'", info.DPSID)
	}

	if info.ProviderCNPJ != "12345678000190" {
		t.Errorf("Expected ProviderCNPJ '12345678000190', got '%s'", info.ProviderCNPJ)
	}

	if info.ProviderName != "Provider Company Ltd" {
		t.Errorf("Expected ProviderName 'Provider Company Ltd', got '%s'", info.ProviderName)
	}

	if info.MunicipalityCode != "3550308" {
		t.Errorf("Expected MunicipalityCode '3550308', got '%s'", info.MunicipalityCode)
	}

	if info.Series != "00001" {
		t.Errorf("Expected Series '00001', got '%s'", info.Series)
	}

	if info.Number != "123456" {
		t.Errorf("Expected Number '123456', got '%s'", info.Number)
	}

	if info.ServiceValue != 1000.00 {
		t.Errorf("Expected ServiceValue 1000.00, got %f", info.ServiceValue)
	}

	if info.Environment != 2 {
		t.Errorf("Expected Environment 2, got %d", info.Environment)
	}

	if !info.HasSignature {
		t.Error("Expected HasSignature to be true")
	}

	if info.NationalServiceCode != "010101" {
		t.Errorf("Expected NationalServiceCode '010101', got '%s'", info.NationalServiceCode)
	}

	if info.ServiceDescription != "Software development services" {
		t.Errorf("Expected ServiceDescription 'Software development services', got '%s'", info.ServiceDescription)
	}

	if info.ServiceMunicipalityCode != "3550308" {
		t.Errorf("Expected ServiceMunicipalityCode '3550308', got '%s'", info.ServiceMunicipalityCode)
	}
}

func TestParsePreSignedXML_UnsignedDocument(t *testing.T) {
	info, err := ParsePreSignedXML(validUnsignedDPSXML)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if info.HasSignature {
		t.Error("Expected HasSignature to be false for unsigned document")
	}

	if info.Environment != 1 {
		t.Errorf("Expected Environment 1 (production), got %d", info.Environment)
	}
}

func TestParsePreSignedXML_WithCPF(t *testing.T) {
	info, err := ParsePreSignedXML(dpsWithCPFXML)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if info.ProviderCPF != "12345678901" {
		t.Errorf("Expected ProviderCPF '12345678901', got '%s'", info.ProviderCPF)
	}

	if info.ProviderCNPJ != "" {
		t.Errorf("Expected empty ProviderCNPJ when CPF is present, got '%s'", info.ProviderCNPJ)
	}
}

func TestParsePreSignedXML_EmptyXML(t *testing.T) {
	_, err := ParsePreSignedXML("")
	if err != ErrPreSignedInvalidXML {
		t.Errorf("Expected ErrPreSignedInvalidXML, got %v", err)
	}
}

func TestParsePreSignedXML_InvalidXML(t *testing.T) {
	_, err := ParsePreSignedXML("<invalid xml")
	if err == nil {
		t.Error("Expected error for invalid XML")
	}
}

func TestParsePreSignedXML_NoDPS(t *testing.T) {
	_, err := ParsePreSignedXML(noDPSXML)
	if err != ErrPreSignedNotDPS {
		t.Errorf("Expected ErrPreSignedNotDPS, got %v", err)
	}
}

func TestParsePreSignedXML_NoInfDPS(t *testing.T) {
	_, err := ParsePreSignedXML(noInfDPSXML)
	if err != ErrPreSignedMissingInfDPS {
		t.Errorf("Expected ErrPreSignedMissingInfDPS, got %v", err)
	}
}

func TestParsePreSignedXML_NoIDAttribute(t *testing.T) {
	_, err := ParsePreSignedXML(noIDAttributeXML)
	if err != ErrPreSignedMissingID {
		t.Errorf("Expected ErrPreSignedMissingID, got %v", err)
	}
}

func TestParsePreSignedXML_NoProvider(t *testing.T) {
	_, err := ParsePreSignedXML(noProviderXML)
	if err != ErrPreSignedMissingProvider {
		t.Errorf("Expected ErrPreSignedMissingProvider, got %v", err)
	}
}

func TestParsePreSignedXML_NoProviderID(t *testing.T) {
	_, err := ParsePreSignedXML(noProviderIDXML)
	if err != ErrPreSignedMissingProviderID {
		t.Errorf("Expected ErrPreSignedMissingProviderID, got %v", err)
	}
}

func TestPreSignedInfo_GetEnvironmentString(t *testing.T) {
	tests := []struct {
		environment int
		expected    string
	}{
		{1, "producao"},
		{2, "homologacao"},
		{0, "homologacao"}, // default
		{3, "homologacao"}, // invalid
	}

	for _, tt := range tests {
		info := &PreSignedInfo{Environment: tt.environment}
		result := info.GetEnvironmentString()
		if result != tt.expected {
			t.Errorf("GetEnvironmentString() with environment %d = %q, want %q", tt.environment, result, tt.expected)
		}
	}
}

func TestPreSignedInfo_GetProviderID(t *testing.T) {
	tests := []struct {
		name         string
		providerCNPJ string
		providerCPF  string
		expected     string
	}{
		{"CNPJ only", "12345678000190", "", "12345678000190"},
		{"CPF only", "", "12345678901", "12345678901"},
		{"Both", "12345678000190", "12345678901", "12345678000190"}, // CNPJ takes precedence
		{"Neither", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &PreSignedInfo{
				ProviderCNPJ: tt.providerCNPJ,
				ProviderCPF:  tt.providerCPF,
			}
			result := info.GetProviderID()
			if result != tt.expected {
				t.Errorf("GetProviderID() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestPreSignedInfo_Validate(t *testing.T) {
	tests := []struct {
		name           string
		info           *PreSignedInfo
		expectedErrors int
		checkError     string
	}{
		{
			name: "valid signed document",
			info: &PreSignedInfo{
				DPSID:        "DPS12345",
				ProviderCNPJ: "12345678000190",
				Environment:  2,
				HasSignature: true,
			},
			expectedErrors: 0,
		},
		{
			name: "missing DPSID",
			info: &PreSignedInfo{
				ProviderCNPJ: "12345678000190",
				Environment:  2,
				HasSignature: true,
			},
			expectedErrors: 1,
			checkError:     "DPS ID",
		},
		{
			name: "missing provider ID",
			info: &PreSignedInfo{
				DPSID:        "DPS12345",
				Environment:  2,
				HasSignature: true,
			},
			expectedErrors: 1,
			checkError:     "Provider CNPJ or CPF",
		},
		{
			name: "invalid CNPJ length",
			info: &PreSignedInfo{
				DPSID:        "DPS12345",
				ProviderCNPJ: "1234567",
				Environment:  2,
				HasSignature: true,
			},
			expectedErrors: 1,
			checkError:     "14 digits",
		},
		{
			name: "invalid CPF length",
			info: &PreSignedInfo{
				DPSID:       "DPS12345",
				ProviderCPF: "12345",
				Environment: 2,
				HasSignature: true,
			},
			expectedErrors: 1,
			checkError:     "11 digits",
		},
		{
			name: "invalid environment",
			info: &PreSignedInfo{
				DPSID:        "DPS12345",
				ProviderCNPJ: "12345678000190",
				Environment:  0,
				HasSignature: true,
			},
			expectedErrors: 1,
			checkError:     "Environment",
		},
		{
			name: "not signed",
			info: &PreSignedInfo{
				DPSID:        "DPS12345",
				ProviderCNPJ: "12345678000190",
				Environment:  2,
				HasSignature: false,
			},
			expectedErrors: 1,
			checkError:     "not signed",
		},
		{
			name: "multiple errors",
			info: &PreSignedInfo{
				Environment:  0,
				HasSignature: false,
			},
			expectedErrors: 4, // DPSID, provider ID, environment, signature
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := tt.info.Validate()
			if len(errors) != tt.expectedErrors {
				t.Errorf("Expected %d errors, got %d: %v", tt.expectedErrors, len(errors), errors)
			}
			if tt.checkError != "" && len(errors) > 0 {
				found := false
				for _, e := range errors {
					if strings.Contains(e, tt.checkError) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing '%s', got: %v", tt.checkError, errors)
				}
			}
		})
	}
}

func TestCleanNumericString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"12.345.678/0001-90", "12345678000190"},
		{"123.456.789-01", "12345678901"},
		{"12345678000190", "12345678000190"},
		{"abc123def", "123"},
		{"", ""},
	}

	for _, tt := range tests {
		result := cleanNumericString(tt.input)
		if result != tt.expected {
			t.Errorf("cleanNumericString(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestParseDateTime(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"RFC3339", "2024-01-15T10:30:00-03:00", true},
		{"UTC", "2024-01-15T10:30:00Z", true},
		{"with milliseconds", "2024-01-15T10:30:00.000-03:00", true},
		{"invalid", "invalid", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDateTime(tt.input)
			if tt.valid && result.IsZero() {
				t.Errorf("Expected valid time for %q, got zero time", tt.input)
			}
			if !tt.valid && !result.IsZero() {
				t.Errorf("Expected zero time for %q, got %v", tt.input, result)
			}
		})
	}
}

func TestParseDecimal(t *testing.T) {
	tests := []struct {
		input       string
		expected    float64
		expectError bool
	}{
		{"1000.00", 1000.00, false},
		{"1000,00", 1000.00, false}, // Brazilian format
		{"-500.50", -500.50, false},
		{"  1000.00  ", 1000.00, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseDecimal(tt.input)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("parseDecimal(%q) = %f, want %f", tt.input, result, tt.expected)
				}
			}
		})
	}
}
