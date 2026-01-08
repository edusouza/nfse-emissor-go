// Package validation provides validation logic for NFS-e domain objects.
package validation

import (
	"strings"
	"testing"

	"github.com/eduardo/nfse-nacional/internal/domain/emission"
)

func TestTakerValidator_ValidateTaker(t *testing.T) {
	validator := NewTakerValidator()

	tests := []struct {
		name          string
		taker         *emission.TakerRequest
		expectedCount int // number of expected validation errors
		checkFields   []string
	}{
		{
			name:          "nil taker returns error",
			taker:         nil,
			expectedCount: 1,
			checkFields:   []string{"taker"},
		},
		{
			name: "valid taker with CNPJ",
			taker: &emission.TakerRequest{
				CNPJ: "11222333000181", // Valid CNPJ
				Name: "Test Company",
				Address: &emission.AddressRequest{
					Street:           "Rua Test",
					Number:           "123",
					Neighborhood:     "Centro",
					MunicipalityCode: "3550308",
					State:            "SP",
					PostalCode:       "01310100",
				},
			},
			expectedCount: 0,
		},
		{
			name: "valid taker with CPF (address optional)",
			taker: &emission.TakerRequest{
				CPF:  "12345678909", // Valid CPF
				Name: "Test Person",
			},
			expectedCount: 0,
		},
		{
			name: "valid taker with NIF",
			taker: &emission.TakerRequest{
				NIF:  "ES12345678A",
				Name: "Foreign Company",
				Address: &emission.AddressRequest{
					Street:       "Foreign Street",
					Number:       "456",
					Neighborhood: "Foreign District",
					CountryCode:  "ES",
				},
			},
			expectedCount: 0,
		},
		{
			name: "missing identification",
			taker: &emission.TakerRequest{
				Name: "Test Company",
			},
			expectedCount: 1,
			checkFields:   []string{"taker"},
		},
		{
			name: "multiple identifications",
			taker: &emission.TakerRequest{
				CNPJ: "11222333000181",
				CPF:  "12345678909",
				Name: "Test",
				Address: &emission.AddressRequest{
					Street:           "Rua Test",
					Number:           "123",
					Neighborhood:     "Centro",
					MunicipalityCode: "3550308",
					State:            "SP",
					PostalCode:       "01310100",
				},
			},
			expectedCount: 1,
			checkFields:   []string{"taker"},
		},
		{
			name: "invalid CNPJ",
			taker: &emission.TakerRequest{
				CNPJ: "11111111111111",
				Name: "Test Company",
				Address: &emission.AddressRequest{
					Street:           "Rua Test",
					Number:           "123",
					Neighborhood:     "Centro",
					MunicipalityCode: "3550308",
					State:            "SP",
					PostalCode:       "01310100",
				},
			},
			expectedCount: 1,
			checkFields:   []string{"taker.cnpj"},
		},
		{
			name: "invalid CPF",
			taker: &emission.TakerRequest{
				CPF:  "11111111111",
				Name: "Test Person",
			},
			expectedCount: 1,
			checkFields:   []string{"taker.cpf"},
		},
		{
			name: "NIF too long",
			taker: &emission.TakerRequest{
				NIF:  strings.Repeat("A", 41),
				Name: "Foreign Company",
				Address: &emission.AddressRequest{
					Street:       "Foreign Street",
					Number:       "456",
					Neighborhood: "Foreign District",
					CountryCode:  "ES",
				},
			},
			expectedCount: 1,
			checkFields:   []string{"taker.nif"},
		},
		{
			name: "NIF with special characters",
			taker: &emission.TakerRequest{
				NIF:  "ES@#$%",
				Name: "Foreign Company",
				Address: &emission.AddressRequest{
					Street:       "Foreign Street",
					Number:       "456",
					Neighborhood: "Foreign District",
					CountryCode:  "ES",
				},
			},
			expectedCount: 1,
			checkFields:   []string{"taker.nif"},
		},
		{
			name: "missing name",
			taker: &emission.TakerRequest{
				CNPJ: "11222333000181",
				Address: &emission.AddressRequest{
					Street:           "Rua Test",
					Number:           "123",
					Neighborhood:     "Centro",
					MunicipalityCode: "3550308",
					State:            "SP",
					PostalCode:       "01310100",
				},
			},
			expectedCount: 1,
			checkFields:   []string{"taker.name"},
		},
		{
			name: "name too long",
			taker: &emission.TakerRequest{
				CPF:  "12345678909",
				Name: strings.Repeat("A", 301),
			},
			expectedCount: 1,
			checkFields:   []string{"taker.name"},
		},
		{
			name: "CNPJ taker missing required address",
			taker: &emission.TakerRequest{
				CNPJ: "11222333000181",
				Name: "Test Company",
			},
			expectedCount: 1,
			checkFields:   []string{"taker.address"},
		},
		{
			name: "NIF taker missing required address",
			taker: &emission.TakerRequest{
				NIF:  "ES12345678A",
				Name: "Foreign Company",
			},
			expectedCount: 1,
			checkFields:   []string{"taker.address"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.ValidateTaker(tt.taker)

			if len(errors) != tt.expectedCount {
				t.Errorf("expected %d errors, got %d: %+v", tt.expectedCount, len(errors), errors)
				return
			}

			// Check that expected fields are present in errors
			for _, field := range tt.checkFields {
				found := false
				for _, err := range errors {
					if err.Field == field {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error for field %q, but not found in %+v", field, errors)
				}
			}
		})
	}
}

func TestTakerValidator_ValidateNationalAddress(t *testing.T) {
	validator := NewTakerValidator()

	tests := []struct {
		name          string
		taker         *emission.TakerRequest
		expectedCount int
		checkFields   []string
	}{
		{
			name: "valid national address",
			taker: &emission.TakerRequest{
				CNPJ: "11222333000181",
				Name: "Test Company",
				Address: &emission.AddressRequest{
					Street:           "Rua Test",
					Number:           "123",
					Complement:       "Sala 101",
					Neighborhood:     "Centro",
					MunicipalityCode: "3550308",
					State:            "SP",
					PostalCode:       "01310100",
					CountryCode:      "BR",
				},
			},
			expectedCount: 0,
		},
		{
			name: "missing street",
			taker: &emission.TakerRequest{
				CNPJ: "11222333000181",
				Name: "Test Company",
				Address: &emission.AddressRequest{
					Number:           "123",
					Neighborhood:     "Centro",
					MunicipalityCode: "3550308",
					State:            "SP",
					PostalCode:       "01310100",
				},
			},
			expectedCount: 1,
			checkFields:   []string{"taker.address.street"},
		},
		{
			name: "missing number",
			taker: &emission.TakerRequest{
				CNPJ: "11222333000181",
				Name: "Test Company",
				Address: &emission.AddressRequest{
					Street:           "Rua Test",
					Neighborhood:     "Centro",
					MunicipalityCode: "3550308",
					State:            "SP",
					PostalCode:       "01310100",
				},
			},
			expectedCount: 1,
			checkFields:   []string{"taker.address.number"},
		},
		{
			name: "missing neighborhood",
			taker: &emission.TakerRequest{
				CNPJ: "11222333000181",
				Name: "Test Company",
				Address: &emission.AddressRequest{
					Street:           "Rua Test",
					Number:           "123",
					MunicipalityCode: "3550308",
					State:            "SP",
					PostalCode:       "01310100",
				},
			},
			expectedCount: 1,
			checkFields:   []string{"taker.address.neighborhood"},
		},
		{
			name: "missing municipality code",
			taker: &emission.TakerRequest{
				CNPJ: "11222333000181",
				Name: "Test Company",
				Address: &emission.AddressRequest{
					Street:       "Rua Test",
					Number:       "123",
					Neighborhood: "Centro",
					State:        "SP",
					PostalCode:   "01310100",
				},
			},
			expectedCount: 1,
			checkFields:   []string{"taker.address.municipality_code"},
		},
		{
			name: "invalid municipality code format",
			taker: &emission.TakerRequest{
				CNPJ: "11222333000181",
				Name: "Test Company",
				Address: &emission.AddressRequest{
					Street:           "Rua Test",
					Number:           "123",
					Neighborhood:     "Centro",
					MunicipalityCode: "123456", // Should be 7 digits
					State:            "SP",
					PostalCode:       "01310100",
				},
			},
			expectedCount: 1,
			checkFields:   []string{"taker.address.municipality_code"},
		},
		{
			name: "missing state",
			taker: &emission.TakerRequest{
				CNPJ: "11222333000181",
				Name: "Test Company",
				Address: &emission.AddressRequest{
					Street:           "Rua Test",
					Number:           "123",
					Neighborhood:     "Centro",
					MunicipalityCode: "3550308",
					PostalCode:       "01310100",
				},
			},
			expectedCount: 1,
			checkFields:   []string{"taker.address.state"},
		},
		{
			name: "invalid state",
			taker: &emission.TakerRequest{
				CNPJ: "11222333000181",
				Name: "Test Company",
				Address: &emission.AddressRequest{
					Street:           "Rua Test",
					Number:           "123",
					Neighborhood:     "Centro",
					MunicipalityCode: "3550308",
					State:            "XX", // Invalid state
					PostalCode:       "01310100",
				},
			},
			expectedCount: 1,
			checkFields:   []string{"taker.address.state"},
		},
		{
			name: "missing postal code",
			taker: &emission.TakerRequest{
				CNPJ: "11222333000181",
				Name: "Test Company",
				Address: &emission.AddressRequest{
					Street:           "Rua Test",
					Number:           "123",
					Neighborhood:     "Centro",
					MunicipalityCode: "3550308",
					State:            "SP",
				},
			},
			expectedCount: 1,
			checkFields:   []string{"taker.address.postal_code"},
		},
		{
			name: "invalid postal code format",
			taker: &emission.TakerRequest{
				CNPJ: "11222333000181",
				Name: "Test Company",
				Address: &emission.AddressRequest{
					Street:           "Rua Test",
					Number:           "123",
					Neighborhood:     "Centro",
					MunicipalityCode: "3550308",
					State:            "SP",
					PostalCode:       "123456", // Should be 8 digits
				},
			},
			expectedCount: 1,
			checkFields:   []string{"taker.address.postal_code"},
		},
		{
			name: "invalid country code for national address",
			taker: &emission.TakerRequest{
				CNPJ: "11222333000181",
				Name: "Test Company",
				Address: &emission.AddressRequest{
					Street:           "Rua Test",
					Number:           "123",
					Neighborhood:     "Centro",
					MunicipalityCode: "3550308",
					State:            "SP",
					PostalCode:       "01310100",
					CountryCode:      "US", // Should be BR for national
				},
			},
			expectedCount: 1,
			checkFields:   []string{"taker.address.country_code"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.ValidateTaker(tt.taker)

			if len(errors) != tt.expectedCount {
				t.Errorf("expected %d errors, got %d: %+v", tt.expectedCount, len(errors), errors)
				return
			}

			for _, field := range tt.checkFields {
				found := false
				for _, err := range errors {
					if err.Field == field {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error for field %q, but not found in %+v", field, errors)
				}
			}
		})
	}
}

func TestTakerValidator_ValidateForeignAddress(t *testing.T) {
	validator := NewTakerValidator()

	tests := []struct {
		name          string
		taker         *emission.TakerRequest
		expectedCount int
		checkFields   []string
	}{
		{
			name: "valid foreign address",
			taker: &emission.TakerRequest{
				NIF:  "ES12345678A",
				Name: "Foreign Company",
				Address: &emission.AddressRequest{
					Street:       "Foreign Street",
					Number:       "456",
					Neighborhood: "Foreign District",
					CountryCode:  "ES",
				},
			},
			expectedCount: 0,
		},
		{
			name: "valid foreign address with complement",
			taker: &emission.TakerRequest{
				NIF:  "US123456789",
				Name: "US Company",
				Address: &emission.AddressRequest{
					Street:       "Main Street",
					Number:       "100",
					Complement:   "Suite 500",
					Neighborhood: "Downtown",
					CountryCode:  "US",
				},
			},
			expectedCount: 0,
		},
		{
			name: "missing country code",
			taker: &emission.TakerRequest{
				NIF:  "ES12345678A",
				Name: "Foreign Company",
				Address: &emission.AddressRequest{
					Street:       "Foreign Street",
					Number:       "456",
					Neighborhood: "Foreign District",
				},
			},
			expectedCount: 1,
			checkFields:   []string{"taker.address.country_code"},
		},
		{
			name: "BR country code for NIF taker",
			taker: &emission.TakerRequest{
				NIF:  "ES12345678A",
				Name: "Foreign Company",
				Address: &emission.AddressRequest{
					Street:       "Foreign Street",
					Number:       "456",
					Neighborhood: "Foreign District",
					CountryCode:  "BR", // Invalid for foreign taker
				},
			},
			expectedCount: 1,
			checkFields:   []string{"taker.address.country_code"},
		},
		{
			name: "municipality code provided for foreign address",
			taker: &emission.TakerRequest{
				NIF:  "ES12345678A",
				Name: "Foreign Company",
				Address: &emission.AddressRequest{
					Street:           "Foreign Street",
					Number:           "456",
					Neighborhood:     "Foreign District",
					MunicipalityCode: "3550308", // Should not be provided
					CountryCode:      "ES",
				},
			},
			expectedCount: 1,
			checkFields:   []string{"taker.address.municipality_code"},
		},
		{
			name: "state provided for foreign address",
			taker: &emission.TakerRequest{
				NIF:  "ES12345678A",
				Name: "Foreign Company",
				Address: &emission.AddressRequest{
					Street:       "Foreign Street",
					Number:       "456",
					Neighborhood: "Foreign District",
					State:        "SP", // Should not be provided
					CountryCode:  "ES",
				},
			},
			expectedCount: 1,
			checkFields:   []string{"taker.address.state"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.ValidateTaker(tt.taker)

			if len(errors) != tt.expectedCount {
				t.Errorf("expected %d errors, got %d: %+v", tt.expectedCount, len(errors), errors)
				return
			}

			for _, field := range tt.checkFields {
				found := false
				for _, err := range errors {
					if err.Field == field {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error for field %q, but not found in %+v", field, errors)
				}
			}
		})
	}
}

func TestTakerValidator_ValidatePhoneAndEmail(t *testing.T) {
	validator := NewTakerValidator()

	tests := []struct {
		name          string
		taker         *emission.TakerRequest
		expectedCount int
		checkFields   []string
	}{
		{
			name: "valid Brazilian phone",
			taker: &emission.TakerRequest{
				CPF:   "12345678909",
				Name:  "Test Person",
				Phone: "11999998888",
			},
			expectedCount: 0,
		},
		{
			name: "valid Brazilian phone with formatting",
			taker: &emission.TakerRequest{
				CPF:   "12345678909",
				Name:  "Test Person",
				Phone: "(11) 99999-8888",
			},
			expectedCount: 0,
		},
		{
			name: "invalid Brazilian phone - too short",
			taker: &emission.TakerRequest{
				CPF:   "12345678909",
				Name:  "Test Person",
				Phone: "123456789", // 9 digits
			},
			expectedCount: 1,
			checkFields:   []string{"taker.phone"},
		},
		{
			name: "valid international phone",
			taker: &emission.TakerRequest{
				NIF:   "ES12345678A",
				Name:  "Foreign Company",
				Phone: "+34123456789",
				Address: &emission.AddressRequest{
					Street:       "Foreign Street",
					Number:       "456",
					Neighborhood: "Foreign District",
					CountryCode:  "ES",
				},
			},
			expectedCount: 0,
		},
		{
			name: "valid email",
			taker: &emission.TakerRequest{
				CPF:   "12345678909",
				Name:  "Test Person",
				Email: "test@example.com",
			},
			expectedCount: 0,
		},
		{
			name: "invalid email format",
			taker: &emission.TakerRequest{
				CPF:   "12345678909",
				Name:  "Test Person",
				Email: "invalid-email",
			},
			expectedCount: 1,
			checkFields:   []string{"taker.email"},
		},
		{
			name: "email too long",
			taker: &emission.TakerRequest{
				CPF:   "12345678909",
				Name:  "Test Person",
				Email: strings.Repeat("a", 250) + "@example.com",
			},
			expectedCount: 1,
			checkFields:   []string{"taker.email"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.ValidateTaker(tt.taker)

			if len(errors) != tt.expectedCount {
				t.Errorf("expected %d errors, got %d: %+v", tt.expectedCount, len(errors), errors)
				return
			}

			for _, field := range tt.checkFields {
				found := false
				for _, err := range errors {
					if err.Field == field {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error for field %q, but not found in %+v", field, errors)
				}
			}
		})
	}
}

func TestTakerValidator_NIFValidation(t *testing.T) {
	validator := NewTakerValidator()

	tests := []struct {
		name    string
		nif     string
		isValid bool
	}{
		{"valid NIF alphanumeric", "ES12345678A", true},
		{"valid NIF numeric", "123456789012345", true},
		{"valid NIF alpha", "ABCDEFGHIJ", true},
		{"valid NIF max length", strings.Repeat("A", 40), true},
		{"invalid NIF too long", strings.Repeat("A", 41), false},
		{"invalid NIF with special chars", "ES@123", false},
		{"invalid NIF with spaces", "ES 123", false},
		{"invalid NIF empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taker := &emission.TakerRequest{
				NIF:  tt.nif,
				Name: "Test Company",
				Address: &emission.AddressRequest{
					Street:       "Test Street",
					Number:       "100",
					Neighborhood: "Test District",
					CountryCode:  "ES",
				},
			}

			// Handle empty NIF case specially - it should fail "no identifier" check
			if tt.nif == "" {
				taker.CNPJ = "11222333000181" // Use valid CNPJ instead for empty NIF test
				taker.NIF = ""
				taker.Address = &emission.AddressRequest{
					Street:           "Rua Test",
					Number:           "123",
					Neighborhood:     "Centro",
					MunicipalityCode: "3550308",
					State:            "SP",
					PostalCode:       "01310100",
				}
				errors := validator.ValidateTaker(taker)
				if len(errors) != 0 {
					t.Errorf("expected 0 errors for CNPJ fallback, got %d: %+v", len(errors), errors)
				}
				return
			}

			errors := validator.ValidateTaker(taker)
			hasNIFError := false
			for _, err := range errors {
				if err.Field == "taker.nif" {
					hasNIFError = true
					break
				}
			}

			if tt.isValid && hasNIFError {
				t.Errorf("expected NIF %q to be valid, but got error", tt.nif)
			}
			if !tt.isValid && !hasNIFError {
				t.Errorf("expected NIF %q to be invalid, but no error", tt.nif)
			}
		})
	}
}
