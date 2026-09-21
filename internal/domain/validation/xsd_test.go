// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

// Package validation provides validation logic for NFS-e domain objects.
package validation

import (
	"strings"
	"testing"
)

// Test XML documents for XSD validation tests.
const (
	validDPSXML = `<?xml version="1.0" encoding="UTF-8"?>
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
    <prest>
      <CNPJ>12345678000195</CNPJ>
      <xNome>Provider Company Ltd</xNome>
    </prest>
    <serv>
      <locPrest>
        <cLocPrestacao>3550308</cLocPrestacao>
      </locPrest>
      <cServ>
        <cTribNac>010101</cTribNac>
        <xDescServ>Software development services</xDescServ>
      </cServ>
    </serv>
    <valores>
      <vServPrest>
        <vServ>1000.00</vServ>
      </vServPrest>
      <trib>
        <tribMun>
          <tribISSQN>1</tribISSQN>
          <tpRetISSQN>1</tpRetISSQN>
        </tribMun>
        <totTrib>
          <indTotTrib>0</indTotTrib>
        </totTrib>
      </trib>
    </valores>
  </infDPS>
</DPS>`

	invalidEnvironmentXML = `<?xml version="1.0" encoding="UTF-8"?>
<DPS xmlns="http://www.sped.fazenda.gov.br/nfse">
  <infDPS Id="DPS12345">
    <tpAmb>3</tpAmb>
    <dhEmi>2024-01-15T10:30:00-03:00</dhEmi>
    <verAplic>1.0.0</verAplic>
    <serie>00001</serie>
    <nDPS>123456</nDPS>
    <dCompet>2024-01-15</dCompet>
    <tpEmit>1</tpEmit>
    <cLocEmi>3550308</cLocEmi>
    <prest>
      <CNPJ>12345678000195</CNPJ>
    </prest>
    <serv>
      <locPrest>
        <cLocPrestacao>3550308</cLocPrestacao>
      </locPrest>
      <cServ>
        <cTribNac>010101</cTribNac>
        <xDescServ>Test</xDescServ>
      </cServ>
    </serv>
    <valores>
      <vServPrest>
        <vServ>1000.00</vServ>
      </vServPrest>
      <trib>
        <tribMun>
          <tribISSQN>1</tribISSQN>
          <tpRetISSQN>1</tpRetISSQN>
        </tribMun>
        <totTrib>
          <indTotTrib>0</indTotTrib>
        </totTrib>
      </trib>
    </valores>
  </infDPS>
</DPS>`

	missingInfDPSXML = `<?xml version="1.0" encoding="UTF-8"?>
<DPS xmlns="http://www.sped.fazenda.gov.br/nfse">
</DPS>`

	invalidSeriesXML = `<?xml version="1.0" encoding="UTF-8"?>
<DPS xmlns="http://www.sped.fazenda.gov.br/nfse">
  <infDPS Id="DPS12345">
    <tpAmb>2</tpAmb>
    <dhEmi>2024-01-15T10:30:00-03:00</dhEmi>
    <verAplic>1.0.0</verAplic>
    <serie>123</serie>
    <nDPS>123456</nDPS>
    <dCompet>2024-01-15</dCompet>
    <tpEmit>1</tpEmit>
    <cLocEmi>3550308</cLocEmi>
    <prest>
      <CNPJ>12345678000195</CNPJ>
    </prest>
    <serv>
      <locPrest>
        <cLocPrestacao>3550308</cLocPrestacao>
      </locPrest>
      <cServ>
        <cTribNac>010101</cTribNac>
        <xDescServ>Test</xDescServ>
      </cServ>
    </serv>
    <valores>
      <vServPrest>
        <vServ>1000.00</vServ>
      </vServPrest>
      <trib>
        <tribMun>
          <tribISSQN>1</tribISSQN>
          <tpRetISSQN>1</tpRetISSQN>
        </tribMun>
        <totTrib>
          <indTotTrib>0</indTotTrib>
        </totTrib>
      </trib>
    </valores>
  </infDPS>
</DPS>`
)

func TestNewStructuralValidator(t *testing.T) {
	if NewStructuralValidator() == nil {
		t.Fatal("NewStructuralValidator returned nil")
	}
}

func TestXSDValidator_ValidateDPS_ValidDocument(t *testing.T) {
	validator := NewStructuralValidator()

	errors := validator.ValidateDPS(validDPSXML)

	if len(errors) > 0 {
		t.Errorf("Expected no errors for valid DPS, got: %v", errors)
	}
}

func TestXSDValidator_ValidateDPS_InvalidXML(t *testing.T) {
	validator := NewStructuralValidator()

	errors := validator.ValidateDPS("<invalid xml")

	if len(errors) == 0 {
		t.Error("Expected errors for invalid XML")
	}

	found := false
	for _, e := range errors {
		if e.Code == XSDErrorInvalidFormat {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected INVALID_FORMAT error code")
	}
}

func TestXSDValidator_ValidateDPS_EmptyDocument(t *testing.T) {
	validator := NewStructuralValidator()

	errors := validator.ValidateDPS("")

	if len(errors) == 0 {
		t.Error("Expected errors for empty document")
	}
}

func TestXSDValidator_ValidateDPS_MissingInfDPS(t *testing.T) {
	validator := NewStructuralValidator()

	errors := validator.ValidateDPS(missingInfDPSXML)

	if len(errors) == 0 {
		t.Error("Expected errors for missing infDPS")
	}

	found := false
	for _, e := range errors {
		if strings.Contains(e.Element, "infDPS") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected error about infDPS, got: %v", errors)
	}
}

func TestXSDValidator_ValidateDPS_InvalidEnvironment(t *testing.T) {
	validator := NewStructuralValidator()

	errors := validator.ValidateDPS(invalidEnvironmentXML)

	found := false
	for _, e := range errors {
		if strings.Contains(e.Element, "tpAmb") {
			found = true
			if e.Code != XSDErrorInvalidValue {
				t.Errorf("Expected INVALID_VALUE code for tpAmb, got %s", e.Code)
			}
			break
		}
	}
	if !found {
		t.Errorf("Expected error about tpAmb, got: %v", errors)
	}
}

func TestXSDValidator_ValidateDPS_InvalidSeries(t *testing.T) {
	validator := NewStructuralValidator()

	errors := validator.ValidateDPS(invalidSeriesXML)

	found := false
	for _, e := range errors {
		if strings.Contains(e.Element, "serie") {
			found = true
			if e.Code != XSDErrorInvalidFormat {
				t.Errorf("Expected INVALID_FORMAT code for serie, got %s", e.Code)
			}
			break
		}
	}
	if !found {
		t.Errorf("Expected error about serie format, got: %v", errors)
	}
}

func TestXSDValidator_ValidateDPS_MissingNamespace(t *testing.T) {
	noNamespaceXML := `<?xml version="1.0" encoding="UTF-8"?>
<DPS>
  <infDPS Id="DPS12345">
    <tpAmb>2</tpAmb>
    <dhEmi>2024-01-15T10:30:00-03:00</dhEmi>
    <verAplic>1.0.0</verAplic>
    <serie>00001</serie>
    <nDPS>123456</nDPS>
    <dCompet>2024-01-15</dCompet>
    <tpEmit>1</tpEmit>
    <cLocEmi>3550308</cLocEmi>
    <prest>
      <CNPJ>12345678000195</CNPJ>
    </prest>
    <serv>
      <locPrest>
        <cLocPrestacao>3550308</cLocPrestacao>
      </locPrest>
      <cServ>
        <cTribNac>010101</cTribNac>
        <xDescServ>Test</xDescServ>
      </cServ>
    </serv>
    <valores>
      <vServPrest>
        <vServ>1000.00</vServ>
      </vServPrest>
      <trib>
        <tribMun>
          <tribISSQN>1</tribISSQN>
          <tpRetISSQN>1</tpRetISSQN>
        </tribMun>
        <totTrib>
          <indTotTrib>0</indTotTrib>
        </totTrib>
      </trib>
    </valores>
  </infDPS>
</DPS>`

	validator := NewStructuralValidator()
	errors := validator.ValidateDPS(noNamespaceXML)

	found := false
	for _, e := range errors {
		if e.Code == XSDErrorInvalidNamespace {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected INVALID_NAMESPACE error code for missing namespace")
	}
}

func TestXSDValidationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      StructuralError
		expected string
	}{
		{
			name: "with value",
			err: StructuralError{
				Code:    XSDErrorInvalidValue,
				Element: "tpAmb",
				Message: "must be 1 or 2",
				Value:   "3",
			},
			expected: "INVALID_VALUE [tpAmb]: must be 1 or 2 (value: 3)",
		},
		{
			name: "without value",
			err: StructuralError{
				Code:    XSDErrorMissingElement,
				Element: "infDPS",
				Message: "required element not found",
			},
			expected: "MISSING_ELEMENT [infDPS]: required element not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestIsValidXSDDateTime(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"2024-01-15T10:30:00-03:00", true},
		{"2024-01-15T10:30:00Z", true},
		{"2024-01-15T10:30:00.000-03:00", true},
		{"2024-01-15T10:30:00.000Z", true},
		{"2024-01-15", false}, // Date only, not datetime
		{"10:30:00", false},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isValidXSDDateTime(tt.input)
			if result != tt.expected {
				t.Errorf("isValidXSDDateTime(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsValidXSDDate(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"2024-01-15", true},
		{"2024-12-31", true},
		{"2024-01-15T10:30:00", false},
		{"01-15-2024", false},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isValidXSDDate(tt.input)
			if result != tt.expected {
				t.Errorf("isValidXSDDate(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsValidDecimal(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"1000", true},
		{"1000.00", true},
		{"-1000.00", true},
		{"0.50", true},
		{"1000,00", false}, // Brazilian format not supported
		{"abc", false},
		{"", false},
		{"1.000.00", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isValidDecimal(tt.input)
			if result != tt.expected {
				t.Errorf("isValidDecimal(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetEnvironmentFromDPS(t *testing.T) {
	tests := []struct {
		name     string
		xml      string
		expected int
	}{
		{
			name:     "production",
			xml:      `<DPS><infDPS><tpAmb>1</tpAmb></infDPS></DPS>`,
			expected: 1,
		},
		{
			name:     "homologation",
			xml:      `<DPS><infDPS><tpAmb>2</tpAmb></infDPS></DPS>`,
			expected: 2,
		},
		{
			name:     "invalid value",
			xml:      `<DPS><infDPS><tpAmb>3</tpAmb></infDPS></DPS>`,
			expected: 0,
		},
		{
			name:     "missing tpAmb",
			xml:      `<DPS><infDPS></infDPS></DPS>`,
			expected: 0,
		},
		{
			name:     "invalid XML",
			xml:      "<invalid",
			expected: 0,
		},
		{
			name:     "full valid DPS",
			xml:      validDPSXML,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetEnvironmentFromDPS(tt.xml)
			if result != tt.expected {
				t.Errorf("GetEnvironmentFromDPS() = %d, want %d", result, tt.expected)
			}
		})
	}
}

// TestValidateVerAplic guards a field that is easy to overflow without
// noticing. TSVerAplic caps verAplic at 20 characters, and the emitter was
// filling it with "nfse-cli " plus a Go pseudo-version — 49 characters — which
// would have had every declaration rejected on that field alone.
func TestValidateVerAplic(t *testing.T) {
	build := func(verAplic string) string {
		return strings.Replace(validDPSXML, "<verAplic>1.0.0</verAplic>",
			"<verAplic>"+verAplic+"</verAplic>", 1)
	}

	cases := []struct {
		name     string
		verAplic string
		wantErr  bool
	}{
		{name: "curto", verAplic: "nfse-cli v0.4.0"},
		{name: "exatamente 20", verAplic: strings.Repeat("a", 20)},
		{name: "21 caracteres", verAplic: strings.Repeat("a", 21), wantErr: true},
		{name: "pseudo-versao completa", verAplic: "nfse-cli v0.0.0-20260918131458-fed93cba325f", wantErr: true},
		{name: "vazio", verAplic: "", wantErr: true},
	}

	validator := NewStructuralValidator()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			xmlStr := build(tc.verAplic)
			if xmlStr == validDPSXML && tc.verAplic != "1.0.0" {
				t.Fatal("a fixture mudou: <verAplic>1.0.0</verAplic> nao foi encontrado")
			}

			var encontrou bool
			for _, e := range validator.ValidateDPS(xmlStr) {
				if strings.Contains(e.Element, "verAplic") {
					encontrou = true
				}
			}
			if encontrou != tc.wantErr {
				t.Errorf("erro em verAplic = %v, esperava %v", encontrou, tc.wantErr)
			}
		})
	}
}
