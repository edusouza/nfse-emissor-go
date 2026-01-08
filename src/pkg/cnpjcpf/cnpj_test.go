package cnpjcpf

import "testing"

func TestValidateCNPJ(t *testing.T) {
	tests := []struct {
		name  string
		cnpj  string
		valid bool
	}{
		// Valid CNPJs
		{"valid CNPJ unformatted", "11222333000181", true},
		{"valid CNPJ formatted", "11.222.333/0001-81", true},
		{"valid CNPJ Receita Federal", "00394460005887", true},
		{"valid CNPJ Banco do Brasil", "00000000000191", true},

		// Invalid CNPJs
		{"invalid CNPJ wrong check digits", "11222333000182", false},
		{"invalid CNPJ all zeros", "00000000000000", false},
		{"invalid CNPJ all ones", "11111111111111", false},
		{"invalid CNPJ all twos", "22222222222222", false},
		{"invalid CNPJ too short", "1122233300018", false},
		{"invalid CNPJ too long", "112223330001811", false},
		{"invalid CNPJ with letters", "11222333A00181", false},
		{"invalid CNPJ empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateCNPJ(tt.cnpj); got != tt.valid {
				t.Errorf("ValidateCNPJ(%q) = %v, want %v", tt.cnpj, got, tt.valid)
			}
		})
	}
}

func TestCleanCNPJ(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"formatted CNPJ", "11.222.333/0001-81", "11222333000181"},
		{"clean CNPJ", "11222333000181", "11222333000181"},
		{"CNPJ with spaces", "11 222 333 0001 81", "11222333000181"},
		{"CNPJ with mixed chars", "11.222.333/0001-81 ", "11222333000181"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CleanCNPJ(tt.input); got != tt.expected {
				t.Errorf("CleanCNPJ(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatCNPJ(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"clean CNPJ", "11222333000181", "11.222.333/0001-81"},
		{"already formatted", "11.222.333/0001-81", "11.222.333/0001-81"},
		{"too short", "1122233", "1122233"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatCNPJ(tt.input); got != tt.expected {
				t.Errorf("FormatCNPJ(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsCNPJFormatted(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		formatted bool
	}{
		{"formatted", "11.222.333/0001-81", true},
		{"unformatted", "11222333000181", false},
		{"wrong format", "11-222-333/0001.81", false},
		{"too short", "11.222.333/0001-8", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCNPJFormatted(tt.input); got != tt.formatted {
				t.Errorf("IsCNPJFormatted(%q) = %v, want %v", tt.input, got, tt.formatted)
			}
		})
	}
}

func TestGenerateCNPJCheckDigits(t *testing.T) {
	tests := []struct {
		name         string
		base         string
		firstDigit   int
		secondDigit  int
		ok           bool
	}{
		{"valid base", "112223330001", 8, 1, true},
		{"another valid base", "003944600058", 8, 7, true},
		{"too short", "11222333000", 0, 0, false},
		{"too long", "1122233300011", 0, 0, false},
		{"with letters", "11222333000A", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, second, ok := GenerateCNPJCheckDigits(tt.base)
			if ok != tt.ok {
				t.Errorf("GenerateCNPJCheckDigits(%q) ok = %v, want %v", tt.base, ok, tt.ok)
			}
			if ok && (first != tt.firstDigit || second != tt.secondDigit) {
				t.Errorf("GenerateCNPJCheckDigits(%q) = (%d, %d), want (%d, %d)",
					tt.base, first, second, tt.firstDigit, tt.secondDigit)
			}
		})
	}
}

func TestCNPJMask(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"valid CNPJ", "11222333000181", "11.***.***/****-81"},
		{"formatted CNPJ", "11.222.333/0001-81", "11.***.***/****-81"},
		{"too short", "1122", "****"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CNPJMask(tt.input); got != tt.expected {
				t.Errorf("CNPJMask(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func BenchmarkValidateCNPJ(b *testing.B) {
	cnpj := "11.222.333/0001-81"
	for i := 0; i < b.N; i++ {
		ValidateCNPJ(cnpj)
	}
}

func BenchmarkCleanCNPJ(b *testing.B) {
	cnpj := "11.222.333/0001-81"
	for i := 0; i < b.N; i++ {
		CleanCNPJ(cnpj)
	}
}
