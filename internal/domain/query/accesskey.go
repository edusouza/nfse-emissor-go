// Package query provides domain logic for NFS-e query operations.
package query

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// AccessKeyLength is the length of an NFS-e access key.
//
// The format comes from TSChaveNFSe in tiposSimples_v1.00.xsd, which restricts
// it to the pattern [0-9]{50}. The Sefin Nacional specification says the same
// in the 400 response of GET /nfse/{chaveAcesso}: "A chave de acesso consultada
// deve conter 50 números."
const AccessKeyLength = 50

// Error definitions for access key validation.
var (
	// ErrAccessKeyEmpty indicates an empty access key was provided.
	ErrAccessKeyEmpty = errors.New("a chave de acesso nao pode ser vazia")

	// ErrAccessKeyInvalidLength indicates the key is not exactly 50 digits long.
	ErrAccessKeyInvalidLength = errors.New("a chave de acesso deve ter exatamente 50 digitos")

	// ErrAccessKeyInvalidCharacters indicates the key contains something other
	// than digits.
	ErrAccessKeyInvalidCharacters = errors.New("a chave de acesso deve conter apenas digitos")
)

// digitsOnly matches a string made entirely of decimal digits.
var digitsOnly = regexp.MustCompile(`^[0-9]+$`)

// ValidateAccessKey validates an NFS-e access key (chaveAcesso).
//
// A key is exactly 50 decimal digits. An earlier version of this function also
// required an "NFSe" prefix, which appears nowhere in the schema or the API
// specification — it would have rejected every real access key.
func ValidateAccessKey(key string) error {
	key = strings.TrimSpace(key)

	if key == "" {
		return ErrAccessKeyEmpty
	}

	if len(key) != AccessKeyLength {
		return fmt.Errorf("%w: recebi %d", ErrAccessKeyInvalidLength, len(key))
	}

	if !digitsOnly.MatchString(key) {
		return ErrAccessKeyInvalidCharacters
	}

	return nil
}

// IsValidAccessKey reports whether the access key is valid.
func IsValidAccessKey(key string) bool {
	return ValidateAccessKey(key) == nil
}

// NormalizeAccessKey trims whitespace and validates an access key, returning
// the normalized key.
func NormalizeAccessKey(key string) (string, error) {
	normalized := strings.TrimSpace(key)
	if err := ValidateAccessKey(normalized); err != nil {
		return "", err
	}
	return normalized, nil
}
