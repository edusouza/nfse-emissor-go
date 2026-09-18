// Package xmlsigner provides XMLDSig digital signature functionality for NFS-e documents.
package xmlsigner

import (
	"strings"
	"testing"
)

// Test XML documents for verification tests.
const (
	// Unsigned DPS XML for testing
	testUnsignedDPS = `<?xml version="1.0" encoding="UTF-8"?>
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
</DPS>`

	// DPS XML with invalid Signature element (for testing signature not found)
	testDPSNoSignature = `<?xml version="1.0" encoding="UTF-8"?>
<DPS xmlns="http://www.sped.fazenda.gov.br/nfse">
  <infDPS Id="DPS12345">
    <tpAmb>2</tpAmb>
  </infDPS>
</DPS>`
)

func TestXMLVerifier_VerifySignature_NoSignature(t *testing.T) {
	verifier := NewXMLVerifier()

	result, err := verifier.VerifySignature(testUnsignedDPS)
	if err != nil {
		t.Fatalf("VerifySignature should not return error for missing signature, got: %v", err)
	}

	if result.Valid {
		t.Error("Expected Valid to be false for unsigned document")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected at least one error for unsigned document")
	}

	// Check that the error mentions missing signature
	found := false
	for _, e := range result.Errors {
		if strings.Contains(e, "Signature") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error about missing Signature, got: %v", result.Errors)
	}
}

func TestXMLVerifier_VerifySignature_InvalidXML(t *testing.T) {
	verifier := NewXMLVerifier()

	_, err := verifier.VerifySignature("<invalid xml")
	if err == nil {
		t.Error("Expected error for invalid XML")
	}
}

func TestXMLVerifier_VerifySignature_EmptyXML(t *testing.T) {
	verifier := NewXMLVerifier()

	_, err := verifier.VerifySignature("")
	if err == nil {
		t.Error("Expected error for empty XML")
	}
}

func TestXMLVerifier_VerifyDPSSignature_NotDPS(t *testing.T) {
	verifier := NewXMLVerifier()

	nonDPSXML := `<?xml version="1.0"?><root><element>value</element></root>`

	result, err := verifier.VerifyDPSSignature(nonDPSXML)
	if err != nil {
		t.Fatalf("VerifyDPSSignature should not return error for non-DPS, got: %v", err)
	}

	if result.Valid {
		t.Error("Expected Valid to be false for non-DPS document")
	}

	// Should have errors about DPS structure
	found := false
	for _, e := range result.Errors {
		if strings.Contains(e, "DPS") || strings.Contains(e, "Signature") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error about DPS or Signature, got: %v", result.Errors)
	}
}

func TestVerificationResult_AddError(t *testing.T) {
	result := &VerificationResult{
		Valid:  true,
		Errors: make([]string, 0),
	}

	result.AddError("test error")

	if result.Valid {
		t.Error("Expected Valid to be false after AddError")
	}

	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(result.Errors))
	}

	if result.Errors[0] != "test error" {
		t.Errorf("Expected 'test error', got '%s'", result.Errors[0])
	}
}

func TestCleanBase64(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean string",
			input:    "SGVsbG9Xb3JsZA==",
			expected: "SGVsbG9Xb3JsZA==",
		},
		{
			name:     "with newlines",
			input:    "SGVsbG9\nXb3Js\nZA==",
			expected: "SGVsbG9Xb3JsZA==",
		},
		{
			name:     "with spaces and tabs",
			input:    "SGVsbG9 Xb3Js\tZA==",
			expected: "SGVsbG9Xb3JsZA==",
		},
		{
			name:     "with carriage returns",
			input:    "SGVsbG9\r\nXb3JsZA==",
			expected: "SGVsbG9Xb3JsZA==",
		},
		{
			name:     "with leading and trailing whitespace",
			input:    "  SGVsbG9Xb3JsZA==  ",
			expected: "SGVsbG9Xb3JsZA==",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanBase64(tt.input)
			if result != tt.expected {
				t.Errorf("cleanBase64(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNewXMLVerifier(t *testing.T) {
	verifier := NewXMLVerifier()

	if verifier == nil {
		t.Fatal("NewXMLVerifier returned nil")
	}

	if !verifier.ValidateCertificate {
		t.Error("Expected ValidateCertificate to be true by default")
	}

	if verifier.CertificateValidator == nil {
		t.Error("Expected CertificateValidator to be initialized")
	}
}

func TestXMLVerifier_VerifySignature_MissingSignedInfo(t *testing.T) {
	// XML with Signature element but missing SignedInfo
	xmlWithBadSignature := `<?xml version="1.0" encoding="UTF-8"?>
<DPS xmlns="http://www.sped.fazenda.gov.br/nfse">
  <infDPS Id="DPS12345">
    <tpAmb>2</tpAmb>
  </infDPS>
  <Signature xmlns="http://www.w3.org/2000/09/xmldsig#">
    <SignatureValue>test</SignatureValue>
  </Signature>
</DPS>`

	verifier := NewXMLVerifier()
	result, err := verifier.VerifySignature(xmlWithBadSignature)

	if err != nil {
		t.Fatalf("Should not return error, got: %v", err)
	}

	if result.Valid {
		t.Error("Expected Valid to be false for signature without SignedInfo")
	}

	found := false
	for _, e := range result.Errors {
		if strings.Contains(e, "SignedInfo") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error about SignedInfo, got: %v", result.Errors)
	}
}

func TestXMLVerifier_VerifySignature_MissingKeyInfo(t *testing.T) {
	// XML with Signature and SignedInfo but missing KeyInfo
	xmlWithBadSignature := `<?xml version="1.0" encoding="UTF-8"?>
<DPS xmlns="http://www.sped.fazenda.gov.br/nfse">
  <infDPS Id="DPS12345">
    <tpAmb>2</tpAmb>
  </infDPS>
  <Signature xmlns="http://www.w3.org/2000/09/xmldsig#">
    <SignedInfo>
      <Reference URI="#DPS12345">
        <DigestValue>test</DigestValue>
      </Reference>
    </SignedInfo>
    <SignatureValue>test</SignatureValue>
  </Signature>
</DPS>`

	verifier := NewXMLVerifier()
	result, err := verifier.VerifySignature(xmlWithBadSignature)

	if err != nil {
		t.Fatalf("Should not return error, got: %v", err)
	}

	if result.Valid {
		t.Error("Expected Valid to be false for signature without KeyInfo")
	}
}
