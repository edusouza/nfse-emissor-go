// Package cnpjcpf provides validation and formatting utilities for Brazilian
// tax identification numbers (CNPJ for companies and CPF for individuals).
package cnpjcpf

import (
	"strings"
)

// CPF validation constants.
const (
	// CPFLength is the length of a valid CPF without formatting.
	CPFLength = 11
)

// ValidateCPF validates an 11-digit CPF number using the modulo 11 algorithm.
// It accepts both formatted (XXX.XXX.XXX-XX) and unformatted (XXXXXXXXXXX) inputs.
// Returns true if the CPF is valid, false otherwise.
func ValidateCPF(cpf string) bool {
	// Clean the input
	cpf = CleanCPF(cpf)

	// Check length
	if len(cpf) != CPFLength {
		return false
	}

	// Check if all digits are the same (invalid CPFs like 00000000000)
	if isAllSameDigit(cpf) {
		return false
	}

	// Convert to digits
	digits := make([]int, CPFLength)
	for i, c := range cpf {
		if c < '0' || c > '9' {
			return false
		}
		digits[i] = int(c - '0')
	}

	// Calculate first verification digit
	// Sum of (digit[i] * (10 - i)) for i = 0..8
	sum := 0
	for i := 0; i < 9; i++ {
		sum += digits[i] * (10 - i)
	}

	remainder := sum % 11
	expectedFirst := 0
	if remainder >= 2 {
		expectedFirst = 11 - remainder
	}

	if digits[9] != expectedFirst {
		return false
	}

	// Calculate second verification digit
	// Sum of (digit[i] * (11 - i)) for i = 0..9
	sum = 0
	for i := 0; i < 10; i++ {
		sum += digits[i] * (11 - i)
	}

	remainder = sum % 11
	expectedSecond := 0
	if remainder >= 2 {
		expectedSecond = 11 - remainder
	}

	return digits[10] == expectedSecond
}

// CleanCPF removes all non-digit characters from a CPF string.
// It removes dots, hyphens, and any other non-numeric characters.
func CleanCPF(cpf string) string {
	return nonDigitRegex.ReplaceAllString(cpf, "")
}

// FormatCPF formats a clean CPF string into the standard format XXX.XXX.XXX-XX.
// If the input is not a valid 11-digit string, it returns the original input.
func FormatCPF(cpf string) string {
	cpf = CleanCPF(cpf)
	if len(cpf) != CPFLength {
		return cpf
	}

	return cpf[0:3] + "." + cpf[3:6] + "." + cpf[6:9] + "-" + cpf[9:11]
}

// IsCPFFormatted checks if a CPF string is in the standard format XXX.XXX.XXX-XX.
func IsCPFFormatted(cpf string) bool {
	if len(cpf) != 14 { // XXX.XXX.XXX-XX
		return false
	}

	// Check format characters at expected positions
	if cpf[3] != '.' || cpf[7] != '.' || cpf[11] != '-' {
		return false
	}

	// Check that the rest are digits
	for i, c := range cpf {
		if i == 3 || i == 7 || i == 11 {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

// GenerateCPFCheckDigits calculates the check digits for the first 9 digits of a CPF.
// Returns the two check digits that should be appended to create a valid CPF.
func GenerateCPFCheckDigits(baseDigits string) (int, int, bool) {
	baseDigits = CleanCPF(baseDigits)
	if len(baseDigits) != 9 {
		return 0, 0, false
	}

	// Convert to digits
	digits := make([]int, 9)
	for i, c := range baseDigits {
		if c < '0' || c > '9' {
			return 0, 0, false
		}
		digits[i] = int(c - '0')
	}

	// Calculate first check digit
	sum := 0
	for i := 0; i < 9; i++ {
		sum += digits[i] * (10 - i)
	}

	remainder := sum % 11
	firstCheck := 0
	if remainder >= 2 {
		firstCheck = 11 - remainder
	}

	// Calculate second check digit
	sum = 0
	for i := 0; i < 9; i++ {
		sum += digits[i] * (11 - i)
	}
	sum += firstCheck * 2

	remainder = sum % 11
	secondCheck := 0
	if remainder >= 2 {
		secondCheck = 11 - remainder
	}

	return firstCheck, secondCheck, true
}

// CPFMask returns a masked version of the CPF showing only the first and last segments.
// Example: "123.***.***-45" - useful for logging without exposing full CPF.
func CPFMask(cpf string) string {
	cpf = CleanCPF(cpf)
	if len(cpf) != CPFLength {
		return strings.Repeat("*", len(cpf))
	}

	return cpf[0:3] + "." + "***" + "." + "***" + "-" + cpf[9:11]
}

// ValidateTaxID validates either a CPF or CNPJ based on the input length.
// Returns the type ("cpf" or "cnpj") and validity.
func ValidateTaxID(taxID string) (taxType string, valid bool) {
	cleaned := CleanCPF(taxID) // Use same cleaning function

	switch len(cleaned) {
	case CPFLength:
		return "cpf", ValidateCPF(cleaned)
	case CNPJLength:
		return "cnpj", ValidateCNPJ(cleaned)
	default:
		return "", false
	}
}
