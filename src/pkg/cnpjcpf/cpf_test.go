package cnpjcpf

import "testing"

func TestValidateCPF(t *testing.T) {
	tests := []struct {
		name  string
		cpf   string
		valid bool
	}{
		// Valid CPFs
		{"valid CPF unformatted", "52998224725", true},
		{"valid CPF formatted", "529.982.247-25", true},
		{"valid CPF another", "11144477735", true},

		// Invalid CPFs
		{"invalid CPF wrong check digits", "52998224726", false},
		{"invalid CPF all zeros", "00000000000", false},
		{"invalid CPF all ones", "11111111111", false},
		{"invalid CPF all twos", "22222222222", false},
		{"invalid CPF all threes", "33333333333", false},
		{"invalid CPF all fours", "44444444444", false},
		{"invalid CPF all fives", "55555555555", false},
		{"invalid CPF all sixes", "66666666666", false},
		{"invalid CPF all sevens", "77777777777", false},
		{"invalid CPF all eights", "88888888888", false},
		{"invalid CPF all nines", "99999999999", false},
		{"invalid CPF too short", "5299822472", false},
		{"invalid CPF too long", "529982247251", false},
		{"invalid CPF with letters", "52998224A25", false},
		{"invalid CPF empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateCPF(tt.cpf); got != tt.valid {
				t.Errorf("ValidateCPF(%q) = %v, want %v", tt.cpf, got, tt.valid)
			}
		})
	}
}

func TestCleanCPF(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"formatted CPF", "529.982.247-25", "52998224725"},
		{"clean CPF", "52998224725", "52998224725"},
		{"CPF with spaces", "529 982 247 25", "52998224725"},
		{"CPF with mixed chars", "529.982.247-25 ", "52998224725"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CleanCPF(tt.input); got != tt.expected {
				t.Errorf("CleanCPF(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatCPF(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"clean CPF", "52998224725", "529.982.247-25"},
		{"already formatted", "529.982.247-25", "529.982.247-25"},
		{"too short", "5299822", "5299822"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatCPF(tt.input); got != tt.expected {
				t.Errorf("FormatCPF(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsCPFFormatted(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		formatted bool
	}{
		{"formatted", "529.982.247-25", true},
		{"unformatted", "52998224725", false},
		{"wrong format", "529-982-247.25", false},
		{"too short", "529.982.247-2", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCPFFormatted(tt.input); got != tt.formatted {
				t.Errorf("IsCPFFormatted(%q) = %v, want %v", tt.input, got, tt.formatted)
			}
		})
	}
}

func TestGenerateCPFCheckDigits(t *testing.T) {
	tests := []struct {
		name        string
		base        string
		firstDigit  int
		secondDigit int
		ok          bool
	}{
		{"valid base", "529982247", 2, 5, true},
		{"another valid base", "111444777", 3, 5, true},
		{"too short", "52998224", 0, 0, false},
		{"too long", "5299822471", 0, 0, false},
		{"with letters", "52998224A", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, second, ok := GenerateCPFCheckDigits(tt.base)
			if ok != tt.ok {
				t.Errorf("GenerateCPFCheckDigits(%q) ok = %v, want %v", tt.base, ok, tt.ok)
			}
			if ok && (first != tt.firstDigit || second != tt.secondDigit) {
				t.Errorf("GenerateCPFCheckDigits(%q) = (%d, %d), want (%d, %d)",
					tt.base, first, second, tt.firstDigit, tt.secondDigit)
			}
		})
	}
}

func TestCPFMask(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"valid CPF", "52998224725", "529.***.***-25"},
		{"formatted CPF", "529.982.247-25", "529.***.***-25"},
		{"too short", "5299", "****"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CPFMask(tt.input); got != tt.expected {
				t.Errorf("CPFMask(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestValidateTaxID(t *testing.T) {
	tests := []struct {
		name      string
		taxID     string
		wantType  string
		wantValid bool
	}{
		{"valid CPF", "52998224725", "cpf", true},
		{"valid CNPJ", "11222333000181", "cnpj", true},
		{"invalid CPF", "52998224726", "cpf", false},
		{"invalid CNPJ", "11222333000182", "cnpj", false},
		{"wrong length", "123456", "", false},
		{"empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotValid := ValidateTaxID(tt.taxID)
			if gotType != tt.wantType || gotValid != tt.wantValid {
				t.Errorf("ValidateTaxID(%q) = (%q, %v), want (%q, %v)",
					tt.taxID, gotType, gotValid, tt.wantType, tt.wantValid)
			}
		})
	}
}

func BenchmarkValidateCPF(b *testing.B) {
	cpf := "529.982.247-25"
	for i := 0; i < b.N; i++ {
		ValidateCPF(cpf)
	}
}

func BenchmarkCleanCPF(b *testing.B) {
	cpf := "529.982.247-25"
	for i := 0; i < b.N; i++ {
		CleanCPF(cpf)
	}
}
