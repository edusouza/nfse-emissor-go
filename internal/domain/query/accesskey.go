// Package query provides domain logic for NFS-e query operations including
// access key validation, error handling, and response structures.
package query

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Access key constants.
const (
	// AccessKeyLength is the required length of an NFS-e access key (50 characters).
	AccessKeyLength = 50

	// AccessKeyPrefix is the required prefix for all NFS-e access keys.
	AccessKeyPrefix = "NFSe"

	// AccessKeyPrefixLength is the length of the "NFSe" prefix.
	AccessKeyPrefixLength = 4
)

// Error definitions for access key validation.
var (
	// ErrAccessKeyEmpty indicates an empty access key was provided.
	ErrAccessKeyEmpty = errors.New("access key cannot be empty")

	// ErrAccessKeyInvalidLength indicates the access key does not have exactly 50 characters.
	ErrAccessKeyInvalidLength = errors.New("access key must be exactly 50 characters")

	// ErrAccessKeyInvalidPrefix indicates the access key does not start with "NFSe".
	ErrAccessKeyInvalidPrefix = errors.New("access key must start with 'NFSe' prefix")

	// ErrAccessKeyInvalidCharacters indicates the access key contains invalid characters.
	ErrAccessKeyInvalidCharacters = errors.New("access key must contain only alphanumeric characters")
)

// alphanumericRegex validates that a string contains only alphanumeric characters.
var alphanumericRegex = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

// ValidateAccessKey validates an NFS-e access key (chaveAcesso).
//
// Validation rules:
//   - Must be exactly 50 characters
//   - Must contain only alphanumeric characters (a-z, A-Z, 0-9)
//   - Must start with "NFSe" prefix
//
// Example valid access key: "NFSe3550308202601081123456789012300000000000012310"
//
// Returns nil if the access key is valid, or an error describing the validation failure.
func ValidateAccessKey(key string) error {
	// Trim whitespace
	key = strings.TrimSpace(key)

	if key == "" {
		return ErrAccessKeyEmpty
	}

	if len(key) != AccessKeyLength {
		return fmt.Errorf("%w: got %d characters", ErrAccessKeyInvalidLength, len(key))
	}

	if !strings.HasPrefix(key, AccessKeyPrefix) {
		return fmt.Errorf("%w: got '%s'", ErrAccessKeyInvalidPrefix, key[:min(4, len(key))])
	}

	if !alphanumericRegex.MatchString(key) {
		return ErrAccessKeyInvalidCharacters
	}

	return nil
}

// IsValidAccessKey returns true if the access key is valid, false otherwise.
// This is a convenience wrapper around ValidateAccessKey for use in boolean contexts.
func IsValidAccessKey(key string) bool {
	return ValidateAccessKey(key) == nil
}

// NormalizeAccessKey trims whitespace and validates an access key, returning
// the normalized key and any validation error.
func NormalizeAccessKey(key string) (string, error) {
	normalized := strings.TrimSpace(key)
	if err := ValidateAccessKey(normalized); err != nil {
		return "", err
	}
	return normalized, nil
}

// AccessKeyInfo represents parsed information from an NFS-e access key.
// Note: This is a best-effort parse based on the typical access key structure.
// The exact format may vary and should be confirmed with official documentation.
type AccessKeyInfo struct {
	// Prefix is the "NFSe" prefix that identifies the key type.
	Prefix string

	// Body is the remaining 46 characters after the prefix.
	Body string
}

// ParseAccessKey parses an access key into its components.
// Returns an error if the access key is invalid.
func ParseAccessKey(key string) (*AccessKeyInfo, error) {
	if err := ValidateAccessKey(key); err != nil {
		return nil, err
	}

	return &AccessKeyInfo{
		Prefix: key[:AccessKeyPrefixLength],
		Body:   key[AccessKeyPrefixLength:],
	}, nil
}
