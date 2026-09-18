// Package cnpjcpf provides validation and formatting utilities for Brazilian
// tax identification numbers (CNPJ for companies and CPF for individuals).
package cnpjcpf

import (
	"regexp"
	"strings"
)

// CNPJ validation constants.
const (
	// CNPJLength is the length of a valid CNPJ without formatting.
	CNPJLength = 14
)

// Common patterns for cleaning tax IDs.
var nonDigitRegex = regexp.MustCompile(`\D`)

// ValidateCNPJ validates a 14-digit CNPJ number using the modulo 11 algorithm.
// It accepts both formatted (XX.XXX.XXX/XXXX-XX) and unformatted (XXXXXXXXXXXXXX) inputs.
// Returns true if the CNPJ is valid, false otherwise.
func ValidateCNPJ(cnpj string) bool {
	// Clean the input
	cnpj = CleanCNPJ(cnpj)

	// Check length
	if len(cnpj) != CNPJLength {
		return false
	}

	// Check if all digits are the same (invalid CNPJs like 00000000000000)
	if isAllSameDigit(cnpj) {
		return false
	}

	// Convert to digits
	digits := make([]int, CNPJLength)
	for i, c := range cnpj {
		if c < '0' || c > '9' {
			return false
		}
		digits[i] = int(c - '0')
	}

	// Calculate first verification digit
	// Weights: 5,4,3,2,9,8,7,6,5,4,3,2
	firstWeights := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	sum := 0
	for i := 0; i < 12; i++ {
		sum += digits[i] * firstWeights[i]
	}

	remainder := sum % 11
	expectedFirst := 0
	if remainder >= 2 {
		expectedFirst = 11 - remainder
	}

	if digits[12] != expectedFirst {
		return false
	}

	// Calculate second verification digit
	// Weights: 6,5,4,3,2,9,8,7,6,5,4,3,2
	secondWeights := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	sum = 0
	for i := 0; i < 13; i++ {
		sum += digits[i] * secondWeights[i]
	}

	remainder = sum % 11
	expectedSecond := 0
	if remainder >= 2 {
		expectedSecond = 11 - remainder
	}

	return digits[13] == expectedSecond
}

// CleanCNPJ removes all non-digit characters from a CNPJ string.
// It removes dots, slashes, hyphens, and any other non-numeric characters.
func CleanCNPJ(cnpj string) string {
	return nonDigitRegex.ReplaceAllString(cnpj, "")
}

// FormatCNPJ formats a clean CNPJ string into the standard format XX.XXX.XXX/XXXX-XX.
// If the input is not a valid 14-digit string, it returns the original input.
func FormatCNPJ(cnpj string) string {
	cnpj = CleanCNPJ(cnpj)
	if len(cnpj) != CNPJLength {
		return cnpj
	}

	return cnpj[0:2] + "." + cnpj[2:5] + "." + cnpj[5:8] + "/" + cnpj[8:12] + "-" + cnpj[12:14]
}

// IsCNPJFormatted checks if a CNPJ string is in the standard format XX.XXX.XXX/XXXX-XX.
func IsCNPJFormatted(cnpj string) bool {
	if len(cnpj) != 18 { // XX.XXX.XXX/XXXX-XX
		return false
	}

	// Check format characters at expected positions
	if cnpj[2] != '.' || cnpj[6] != '.' || cnpj[10] != '/' || cnpj[15] != '-' {
		return false
	}

	// Check that the rest are digits
	for i, c := range cnpj {
		if i == 2 || i == 6 || i == 10 || i == 15 {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

// GenerateCNPJCheckDigits calculates the check digits for the first 12 digits of a CNPJ.
// Returns the two check digits that should be appended to create a valid CNPJ.
func GenerateCNPJCheckDigits(baseDigits string) (int, int, bool) {
	baseDigits = CleanCNPJ(baseDigits)
	if len(baseDigits) != 12 {
		return 0, 0, false
	}

	// Convert to digits
	digits := make([]int, 12)
	for i, c := range baseDigits {
		if c < '0' || c > '9' {
			return 0, 0, false
		}
		digits[i] = int(c - '0')
	}

	// Calculate first check digit
	firstWeights := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	sum := 0
	for i := 0; i < 12; i++ {
		sum += digits[i] * firstWeights[i]
	}

	remainder := sum % 11
	firstCheck := 0
	if remainder >= 2 {
		firstCheck = 11 - remainder
	}

	// Calculate second check digit
	secondWeights := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	sum = 0
	for i := 0; i < 12; i++ {
		sum += digits[i] * secondWeights[i]
	}
	sum += firstCheck * secondWeights[12]

	remainder = sum % 11
	secondCheck := 0
	if remainder >= 2 {
		secondCheck = 11 - remainder
	}

	return firstCheck, secondCheck, true
}

// isAllSameDigit checks if all characters in a string are the same digit.
func isAllSameDigit(s string) bool {
	if len(s) == 0 {
		return false
	}
	first := s[0]
	for i := 1; i < len(s); i++ {
		if s[i] != first {
			return false
		}
	}
	return true
}

// CNPJMask returns a masked version of the CNPJ showing only the first and last segments.
// Example: "12.***.***/****-34" - useful for logging without exposing full CNPJ.
func CNPJMask(cnpj string) string {
	cnpj = CleanCNPJ(cnpj)
	if len(cnpj) != CNPJLength {
		return strings.Repeat("*", len(cnpj))
	}

	return cnpj[0:2] + "." + "***" + "." + "***" + "/" + "****" + "-" + cnpj[12:14]
}
