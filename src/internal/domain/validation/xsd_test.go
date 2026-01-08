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
    <subst>2</subst>
    <prest>
      <CNPJ>12345678000190</CNPJ>
    </prest>
    <serv>
      <cTribNac>010101</cTribNac>
      <xDescServ>Test</xDescServ>
      <cLocPrest>3550308</cLocPrest>
    </serv>
    <valores>
      <vServPrest>1000.00</vServPrest>
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
    <subst>2</subst>
    <prest>
      <CNPJ>12345678000190</CNPJ>
    </prest>
    <serv>
      <cTribNac>010101</cTribNac>
      <xDescServ>Test</xDescServ>
      <cLocPrest>3550308</cLocPrest>
    </serv>
    <valores>
      <vServPrest>1000.00</vServPrest>
    </valores>
  </infDPS>
</DPS>`
)

func TestNewXSDValidator(t *testing.T) {
	validator, err := NewXSDValidator("/path/to/schemas")
	if err != nil {
		t.Fatalf("NewXSDValidator should not return error: %v", err)
	}

	if validator == nil {
		t.Fatal("NewXSDValidator returned nil")
	}

	if validator.SchemaDir != "/path/to/schemas" {
		t.Errorf("Expected SchemaDir to be '/path/to/schemas', got '%s'", validator.SchemaDir)
	}
}

func TestXSDValidator_ValidateDPS_ValidDocument(t *testing.T) {
	validator, _ := NewXSDValidator("")

	errors := validator.ValidateDPS(validDPSXML)

	if len(errors) > 0 {
		t.Errorf("Expected no errors for valid DPS, got: %v", errors)
	}
}

func TestXSDValidator_ValidateDPS_InvalidXML(t *testing.T) {
	validator, _ := NewXSDValidator("")

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
	validator, _ := NewXSDValidator("")

	errors := validator.ValidateDPS("")

	if len(errors) == 0 {
		t.Error("Expected errors for empty document")
	}
}

func TestXSDValidator_ValidateDPS_MissingInfDPS(t *testing.T) {
	validator, _ := NewXSDValidator("")

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
	validator, _ := NewXSDValidator("")

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
	validator, _ := NewXSDValidator("")

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
    <subst>2</subst>
    <prest>
      <CNPJ>12345678000190</CNPJ>
    </prest>
    <serv>
      <cTribNac>010101</cTribNac>
      <xDescServ>Test</xDescServ>
      <cLocPrest>3550308</cLocPrest>
    </serv>
    <valores>
      <vServPrest>1000.00</vServPrest>
    </valores>
  </infDPS>
</DPS>`

	validator, _ := NewXSDValidator("")
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
		err      XSDValidationError
		expected string
	}{
		{
			name: "with value",
			err: XSDValidationError{
				Code:    XSDErrorInvalidValue,
				Element: "tpAmb",
				Message: "must be 1 or 2",
				Value:   "3",
			},
			expected: "INVALID_VALUE [tpAmb]: must be 1 or 2 (value: 3)",
		},
		{
			name: "without value",
			err: XSDValidationError{
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
