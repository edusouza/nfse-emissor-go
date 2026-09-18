package dpsid

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *DPSIdentifier
		wantErr error
	}{
		{
			name:  "valid CNPJ identifier",
			input: "355030811234567800019900001000000000000001",
			want: &DPSIdentifier{
				MunicipalityCode:    "3550308",
				RegistrationType:    RegistrationTypeCNPJ,
				FederalRegistration: "12345678000199",
				Series:              "00001",
				Number:              "000000000000001",
			},
			wantErr: nil,
		},
		{
			name:  "valid CPF identifier",
			input: "355030820001234567890100001000000000000001",
			want: &DPSIdentifier{
				MunicipalityCode:    "3550308",
				RegistrationType:    RegistrationTypeCPF,
				FederalRegistration: "00012345678901",
				Series:              "00001",
				Number:              "000000000000001",
			},
			wantErr: nil,
		},
		{
			name:  "valid with all zeros in number",
			input: "355030811234567800019900001000000000000000",
			want: &DPSIdentifier{
				MunicipalityCode:    "3550308",
				RegistrationType:    RegistrationTypeCNPJ,
				FederalRegistration: "12345678000199",
				Series:              "00001",
				Number:              "000000000000000",
			},
			wantErr: nil,
		},
		{
			name:  "valid with high number",
			input: "355030811234567800019999999999999999999999",
			want: &DPSIdentifier{
				MunicipalityCode:    "3550308",
				RegistrationType:    RegistrationTypeCNPJ,
				FederalRegistration: "12345678000199",
				Series:              "99999",
				Number:              "999999999999999",
			},
			wantErr: nil,
		},
		{
			name:    "empty string",
			input:   "",
			want:    nil,
			wantErr: ErrEmptyDPSID,
		},
		{
			name:    "too short",
			input:   "35503081123456780001990000100000000000001",
			want:    nil,
			wantErr: ErrInvalidLength,
		},
		{
			name:    "too long",
			input:   "3550308112345678000199000010000000000000011",
			want:    nil,
			wantErr: ErrInvalidLength,
		},
		{
			name:    "contains letters",
			input:   "35503081123456780001990000100000000000000A",
			want:    nil,
			wantErr: ErrInvalidCharacters,
		},
		{
			name:    "contains special characters",
			input:   "3550308-12345678000199-00001-00000000000001",
			want:    nil,
			wantErr: ErrInvalidLength, // Special chars cause length change
		},
		{
			name:    "invalid registration type 0",
			input:   "355030801234567800019900001000000000000001",
			want:    nil,
			wantErr: ErrInvalidRegistrationType,
		},
		{
			name:    "invalid registration type 3",
			input:   "355030831234567800019900001000000000000001",
			want:    nil,
			wantErr: ErrInvalidRegistrationType,
		},
		{
			name:    "invalid registration type 9",
			input:   "355030891234567800019900001000000000000001",
			want:    nil,
			wantErr: ErrInvalidRegistrationType,
		},
		{
			name:    "CPF without 000 prefix",
			input:   "355030821234567800019900001000000000000001",
			want:    nil,
			wantErr: ErrInvalidCPFPadding,
		},
		{
			name:  "whitespace is trimmed",
			input: "  355030811234567800019900001000000000000001  ",
			want: &DPSIdentifier{
				MunicipalityCode:    "3550308",
				RegistrationType:    RegistrationTypeCNPJ,
				FederalRegistration: "12345678000199",
				Series:              "00001",
				Number:              "000000000000001",
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("Parse() expected error %v, got nil", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("Parse() unexpected error: %v", err)
				return
			}

			if got.MunicipalityCode != tt.want.MunicipalityCode {
				t.Errorf("MunicipalityCode = %v, want %v", got.MunicipalityCode, tt.want.MunicipalityCode)
			}
			if got.RegistrationType != tt.want.RegistrationType {
				t.Errorf("RegistrationType = %v, want %v", got.RegistrationType, tt.want.RegistrationType)
			}
			if got.FederalRegistration != tt.want.FederalRegistration {
				t.Errorf("FederalRegistration = %v, want %v", got.FederalRegistration, tt.want.FederalRegistration)
			}
			if got.Series != tt.want.Series {
				t.Errorf("Series = %v, want %v", got.Series, tt.want.Series)
			}
			if got.Number != tt.want.Number {
				t.Errorf("Number = %v, want %v", got.Number, tt.want.Number)
			}
		})
	}
}

func TestDPSIdentifier_String(t *testing.T) {
	tests := []struct {
		name string
		id   *DPSIdentifier
		want string
	}{
		{
			name: "CNPJ identifier",
			id: &DPSIdentifier{
				MunicipalityCode:    "3550308",
				RegistrationType:    RegistrationTypeCNPJ,
				FederalRegistration: "12345678000199",
				Series:              "00001",
				Number:              "000000000000001",
			},
			want: "355030811234567800019900001000000000000001",
		},
		{
			name: "CPF identifier",
			id: &DPSIdentifier{
				MunicipalityCode:    "3550308",
				RegistrationType:    RegistrationTypeCPF,
				FederalRegistration: "00012345678901",
				Series:              "00001",
				Number:              "000000000000001",
			},
			want: "355030820001234567890100001000000000000001",
		},
		{
			name: "nil identifier",
			id:   nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.id.String()
			if got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDPSIdentifier_Validate(t *testing.T) {
	tests := []struct {
		name    string
		id      *DPSIdentifier
		wantErr bool
		errType error
	}{
		{
			name: "valid CNPJ",
			id: &DPSIdentifier{
				MunicipalityCode:    "3550308",
				RegistrationType:    RegistrationTypeCNPJ,
				FederalRegistration: "12345678000199",
				Series:              "00001",
				Number:              "000000000000001",
			},
			wantErr: false,
		},
		{
			name: "valid CPF",
			id: &DPSIdentifier{
				MunicipalityCode:    "3550308",
				RegistrationType:    RegistrationTypeCPF,
				FederalRegistration: "00012345678901",
				Series:              "00001",
				Number:              "000000000000001",
			},
			wantErr: false,
		},
		{
			name:    "nil identifier",
			id:      nil,
			wantErr: true,
			errType: ErrEmptyDPSID,
		},
		{
			name: "municipality code too short",
			id: &DPSIdentifier{
				MunicipalityCode:    "355030",
				RegistrationType:    RegistrationTypeCNPJ,
				FederalRegistration: "12345678000199",
				Series:              "00001",
				Number:              "000000000000001",
			},
			wantErr: true,
			errType: ErrInvalidMunicipalityCode,
		},
		{
			name: "municipality code with letters",
			id: &DPSIdentifier{
				MunicipalityCode:    "355030A",
				RegistrationType:    RegistrationTypeCNPJ,
				FederalRegistration: "12345678000199",
				Series:              "00001",
				Number:              "000000000000001",
			},
			wantErr: true,
			errType: ErrInvalidMunicipalityCode,
		},
		{
			name: "invalid registration type",
			id: &DPSIdentifier{
				MunicipalityCode:    "3550308",
				RegistrationType:    5,
				FederalRegistration: "12345678000199",
				Series:              "00001",
				Number:              "000000000000001",
			},
			wantErr: true,
			errType: ErrInvalidRegistrationType,
		},
		{
			name: "federal registration too short",
			id: &DPSIdentifier{
				MunicipalityCode:    "3550308",
				RegistrationType:    RegistrationTypeCNPJ,
				FederalRegistration: "1234567800019",
				Series:              "00001",
				Number:              "000000000000001",
			},
			wantErr: true,
			errType: ErrInvalidFederalRegistration,
		},
		{
			name: "series too short",
			id: &DPSIdentifier{
				MunicipalityCode:    "3550308",
				RegistrationType:    RegistrationTypeCNPJ,
				FederalRegistration: "12345678000199",
				Series:              "0001",
				Number:              "000000000000001",
			},
			wantErr: true,
			errType: ErrInvalidSeries,
		},
		{
			name: "number too short",
			id: &DPSIdentifier{
				MunicipalityCode:    "3550308",
				RegistrationType:    RegistrationTypeCNPJ,
				FederalRegistration: "12345678000199",
				Series:              "00001",
				Number:              "00000000000001",
			},
			wantErr: true,
			errType: ErrInvalidNumber,
		},
		{
			name: "CPF registration without 000 prefix",
			id: &DPSIdentifier{
				MunicipalityCode:    "3550308",
				RegistrationType:    RegistrationTypeCPF,
				FederalRegistration: "12345678901234",
				Series:              "00001",
				Number:              "000000000000001",
			},
			wantErr: true,
			errType: ErrInvalidCPFPadding,
		},
		{
			name: "CNPJ registration does not require 000 prefix",
			id: &DPSIdentifier{
				MunicipalityCode:    "3550308",
				RegistrationType:    RegistrationTypeCNPJ,
				FederalRegistration: "12345678000199",
				Series:              "00001",
				Number:              "000000000000001",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.id.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errType != nil && !errors.Is(err, tt.errType) {
				t.Errorf("Validate() error type = %v, want %v", err, tt.errType)
			}
		})
	}
}

func TestDPSIdentifier_IsCNPJ(t *testing.T) {
	tests := []struct {
		name string
		id   *DPSIdentifier
		want bool
	}{
		{
			name: "CNPJ type",
			id:   &DPSIdentifier{RegistrationType: RegistrationTypeCNPJ},
			want: true,
		},
		{
			name: "CPF type",
			id:   &DPSIdentifier{RegistrationType: RegistrationTypeCPF},
			want: false,
		},
		{
			name: "nil identifier",
			id:   nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.id.IsCNPJ(); got != tt.want {
				t.Errorf("IsCNPJ() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDPSIdentifier_IsCPF(t *testing.T) {
	tests := []struct {
		name string
		id   *DPSIdentifier
		want bool
	}{
		{
			name: "CPF type",
			id:   &DPSIdentifier{RegistrationType: RegistrationTypeCPF},
			want: true,
		},
		{
			name: "CNPJ type",
			id:   &DPSIdentifier{RegistrationType: RegistrationTypeCNPJ},
			want: false,
		},
		{
			name: "nil identifier",
			id:   nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.id.IsCPF(); got != tt.want {
				t.Errorf("IsCPF() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDPSIdentifier_GetCNPJ(t *testing.T) {
	cnpjID := &DPSIdentifier{
		RegistrationType:    RegistrationTypeCNPJ,
		FederalRegistration: "12345678000199",
	}
	cpfID := &DPSIdentifier{
		RegistrationType:    RegistrationTypeCPF,
		FederalRegistration: "00012345678901",
	}

	if got := cnpjID.GetCNPJ(); got != "12345678000199" {
		t.Errorf("GetCNPJ() for CNPJ = %v, want %v", got, "12345678000199")
	}

	if got := cpfID.GetCNPJ(); got != "" {
		t.Errorf("GetCNPJ() for CPF = %v, want empty string", got)
	}
}

func TestDPSIdentifier_GetCPF(t *testing.T) {
	cpfID := &DPSIdentifier{
		RegistrationType:    RegistrationTypeCPF,
		FederalRegistration: "00012345678901",
	}
	cnpjID := &DPSIdentifier{
		RegistrationType:    RegistrationTypeCNPJ,
		FederalRegistration: "12345678000199",
	}

	if got := cpfID.GetCPF(); got != "12345678901" {
		t.Errorf("GetCPF() for CPF = %v, want %v", got, "12345678901")
	}

	if got := cnpjID.GetCPF(); got != "" {
		t.Errorf("GetCPF() for CNPJ = %v, want empty string", got)
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name                string
		municipalityCode    string
		registrationType    int
		federalRegistration string
		series              string
		number              string
		wantString          string
		wantErr             bool
	}{
		{
			name:                "valid CNPJ with padding",
			municipalityCode:    "3550308",
			registrationType:    RegistrationTypeCNPJ,
			federalRegistration: "12345678000199",
			series:              "1",
			number:              "1",
			wantString:          "355030811234567800019900001000000000000001",
			wantErr:             false,
		},
		{
			name:                "valid CPF with padding",
			municipalityCode:    "3550308",
			registrationType:    RegistrationTypeCPF,
			federalRegistration: "12345678901",
			series:              "1",
			number:              "1",
			wantString:          "355030820001234567890100001000000000000001",
			wantErr:             false,
		},
		{
			name:                "CNPJ with formatting characters",
			municipalityCode:    "3550308",
			registrationType:    RegistrationTypeCNPJ,
			federalRegistration: "12.345.678/0001-99",
			series:              "00001",
			number:              "000000000000001",
			wantString:          "355030811234567800019900001000000000000001",
			wantErr:             false,
		},
		{
			name:                "invalid municipality code",
			municipalityCode:    "355",
			registrationType:    RegistrationTypeCNPJ,
			federalRegistration: "12345678000199",
			series:              "1",
			number:              "1",
			wantErr:             true,
		},
		{
			name:                "invalid registration type",
			municipalityCode:    "3550308",
			registrationType:    5,
			federalRegistration: "12345678000199",
			series:              "1",
			number:              "1",
			wantErr:             true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(tt.municipalityCode, tt.registrationType, tt.federalRegistration, tt.series, tt.number)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.String() != tt.wantString {
				t.Errorf("New().String() = %v, want %v", got.String(), tt.wantString)
			}
		})
	}
}

func TestMustParse(t *testing.T) {
	// Test valid input
	id := MustParse("355030811234567800019900001000000000000001")
	if id.MunicipalityCode != "3550308" {
		t.Errorf("MustParse() MunicipalityCode = %v, want %v", id.MunicipalityCode, "3550308")
	}

	// Test panic on invalid input
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustParse() did not panic on invalid input")
		}
	}()
	MustParse("invalid")
}

func TestRoundTrip(t *testing.T) {
	// Test that parsing and stringifying produces the same result
	original := "355030811234567800019900001000000000000001"
	parsed, err := Parse(original)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	result := parsed.String()
	if result != original {
		t.Errorf("Round trip failed: got %v, want %v", result, original)
	}
}
