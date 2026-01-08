// Package validation provides validation logic for NFS-e domain objects.
package validation

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/eduardo/nfse-nacional/internal/domain/emission"
	"github.com/eduardo/nfse-nacional/internal/infrastructure/xmlsigner"
)

// Certificate validation error codes.
const (
	// CertificateCodeInvalidFormat indicates the PFX format is invalid.
	CertificateCodeInvalidFormat = "INVALID_CERTIFICATE_FORMAT"

	// CertificateCodeInvalidPassword indicates the password is incorrect.
	CertificateCodeInvalidPassword = "INVALID_CERTIFICATE_PASSWORD"

	// CertificateCodeExpired indicates the certificate has expired.
	CertificateCodeExpired = "CERTIFICATE_EXPIRED"

	// CertificateCodeNotYetValid indicates the certificate is not yet valid.
	CertificateCodeNotYetValid = "CERTIFICATE_NOT_YET_VALID"

	// CertificateCodeMissingKey indicates the certificate is missing a private key.
	CertificateCodeMissingKey = "CERTIFICATE_MISSING_KEY"

	// CertificateCodeInvalidBase64 indicates the base64 encoding is invalid.
	CertificateCodeInvalidBase64 = "INVALID_CERTIFICATE_BASE64"

	// CertificateCodeInvalidKeyUsage indicates the certificate cannot be used for signing.
	CertificateCodeInvalidKeyUsage = "CERTIFICATE_INVALID_KEY_USAGE"
)

// CertificateValidationResult contains the result of certificate validation.
type CertificateValidationResult struct {
	// Valid indicates whether the certificate passed all validation checks.
	Valid bool

	// Errors contains validation errors if any.
	Errors []ValidationError

	// CertificateInfo contains the parsed certificate information (if parsing succeeded).
	CertificateInfo *xmlsigner.CertificateInfo
}

// ValidateCertificate validates the certificate fields in an emission request.
// It performs the following checks:
//  1. pfx_base64 is valid base64
//  2. password is not empty
//  3. Can parse PFX with password
//  4. Certificate is valid (not expired, correct dates)
//  5. Certificate has a private key
//  6. Certificate can be used for signing
//
// Parameters:
//   - cert: The certificate request to validate
//
// Returns:
//   - []ValidationError: A slice of validation errors (empty if valid)
func ValidateCertificate(cert *emission.CertificateRequest) []ValidationError {
	var validationErrs []ValidationError

	if cert == nil {
		return validationErrs
	}

	// Check that PFX base64 is provided
	if cert.PFXBase64 == "" {
		validationErrs = append(validationErrs, NewValidationError(
			"certificate.pfx_base64",
			ValidationCodeRequired,
			"Certificate PFX (base64 encoded) is required",
		))
		return validationErrs
	}

	// Check that password is provided
	if cert.Password == "" {
		validationErrs = append(validationErrs, NewValidationError(
			"certificate.password",
			ValidationCodeRequired,
			"Certificate password is required",
		))
		return validationErrs
	}

	// Validate base64 encoding
	if !isValidBase64(cert.PFXBase64) {
		validationErrs = append(validationErrs, NewValidationError(
			"certificate.pfx_base64",
			CertificateCodeInvalidBase64,
			"Certificate PFX data is not valid base64 encoding",
		))
		return validationErrs
	}

	// Try to parse the PFX certificate
	certInfo, parseErr := xmlsigner.ParsePFXBase64(cert.PFXBase64, cert.Password)
	if parseErr != nil {
		// Determine the specific error type
		errCode := CertificateCodeInvalidFormat
		errMsg := "Failed to parse certificate: " + parseErr.Error()

		if errors.Is(parseErr, xmlsigner.ErrInvalidPFXFormat) {
			// Check if it's likely a password error
			if strings.Contains(parseErr.Error(), "password") ||
				strings.Contains(parseErr.Error(), "mac") ||
				strings.Contains(parseErr.Error(), "decrypt") {
				errCode = CertificateCodeInvalidPassword
				errMsg = "Invalid certificate password or corrupted PFX file"
			}
		} else if errors.Is(parseErr, xmlsigner.ErrInvalidBase64) {
			errCode = CertificateCodeInvalidBase64
			errMsg = "Certificate PFX data is not valid base64 encoding"
		} else if errors.Is(parseErr, xmlsigner.ErrNoPrivateKey) {
			errCode = CertificateCodeMissingKey
			errMsg = "Certificate PFX file does not contain a private key"
		}

		validationErrs = append(validationErrs, NewValidationError(
			"certificate.pfx_base64",
			errCode,
			errMsg,
		))
		return validationErrs
	}

	// Validate the certificate itself
	validator := xmlsigner.NewCertificateValidator()
	validationResult := validator.ValidateWithDetails(certInfo)

	if !validationResult.Valid {
		for _, certErr := range validationResult.Errors {
			errCode := CertificateCodeInvalidFormat
			errField := "certificate"

			if errors.Is(certErr, xmlsigner.ErrCertificateExpired) {
				errCode = CertificateCodeExpired
				errField = "certificate.pfx_base64"
			} else if errors.Is(certErr, xmlsigner.ErrCertificateNotYetValid) {
				errCode = CertificateCodeNotYetValid
				errField = "certificate.pfx_base64"
			} else if errors.Is(certErr, xmlsigner.ErrCertificateMissingPrivateKey) {
				errCode = CertificateCodeMissingKey
				errField = "certificate.pfx_base64"
			} else if errors.Is(certErr, xmlsigner.ErrCertificateInvalidKeyUsage) {
				errCode = CertificateCodeInvalidKeyUsage
				errField = "certificate.pfx_base64"
			}

			validationErrs = append(validationErrs, NewValidationError(
				errField,
				errCode,
				certErr.Error(),
			))
		}
	}

	return validationErrs
}

// ValidateCertificateWithResult performs certificate validation and returns
// the parsed certificate info if successful.
//
// This is useful when you need both validation and the parsed certificate
// for subsequent signing operations.
//
// Parameters:
//   - cert: The certificate request to validate
//
// Returns:
//   - *CertificateValidationResult: The validation result with errors and parsed certificate
func ValidateCertificateWithResult(cert *emission.CertificateRequest) *CertificateValidationResult {
	result := &CertificateValidationResult{
		Valid:  true,
		Errors: make([]ValidationError, 0),
	}

	if cert == nil {
		return result
	}

	// Check that PFX base64 is provided
	if cert.PFXBase64 == "" {
		result.Valid = false
		result.Errors = append(result.Errors, NewValidationError(
			"certificate.pfx_base64",
			ValidationCodeRequired,
			"Certificate PFX (base64 encoded) is required",
		))
		return result
	}

	// Check that password is provided
	if cert.Password == "" {
		result.Valid = false
		result.Errors = append(result.Errors, NewValidationError(
			"certificate.password",
			ValidationCodeRequired,
			"Certificate password is required",
		))
		return result
	}

	// Validate base64 encoding
	if !isValidBase64(cert.PFXBase64) {
		result.Valid = false
		result.Errors = append(result.Errors, NewValidationError(
			"certificate.pfx_base64",
			CertificateCodeInvalidBase64,
			"Certificate PFX data is not valid base64 encoding",
		))
		return result
	}

	// Try to parse the PFX certificate
	certInfo, parseErr := xmlsigner.ParsePFXBase64(cert.PFXBase64, cert.Password)
	if parseErr != nil {
		result.Valid = false

		// Determine the specific error type
		errCode := CertificateCodeInvalidFormat
		errMsg := "Failed to parse certificate: " + parseErr.Error()

		if errors.Is(parseErr, xmlsigner.ErrInvalidPFXFormat) {
			if strings.Contains(parseErr.Error(), "password") ||
				strings.Contains(parseErr.Error(), "mac") ||
				strings.Contains(parseErr.Error(), "decrypt") {
				errCode = CertificateCodeInvalidPassword
				errMsg = "Invalid certificate password or corrupted PFX file"
			}
		} else if errors.Is(parseErr, xmlsigner.ErrInvalidBase64) {
			errCode = CertificateCodeInvalidBase64
			errMsg = "Certificate PFX data is not valid base64 encoding"
		} else if errors.Is(parseErr, xmlsigner.ErrNoPrivateKey) {
			errCode = CertificateCodeMissingKey
			errMsg = "Certificate PFX file does not contain a private key"
		}

		result.Errors = append(result.Errors, NewValidationError(
			"certificate.pfx_base64",
			errCode,
			errMsg,
		))
		return result
	}

	// Store the parsed certificate
	result.CertificateInfo = certInfo

	// Validate the certificate for signing
	validator := xmlsigner.NewCertificateValidator()
	if err := validator.ValidateForSigning(certInfo); err != nil {
		result.Valid = false

		errCode := CertificateCodeInvalidFormat
		errField := "certificate"

		if errors.Is(err, xmlsigner.ErrCertificateExpired) {
			errCode = CertificateCodeExpired
			errField = "certificate.pfx_base64"
		} else if errors.Is(err, xmlsigner.ErrCertificateNotYetValid) {
			errCode = CertificateCodeNotYetValid
			errField = "certificate.pfx_base64"
		} else if errors.Is(err, xmlsigner.ErrCertificateMissingPrivateKey) {
			errCode = CertificateCodeMissingKey
			errField = "certificate.pfx_base64"
		} else if errors.Is(err, xmlsigner.ErrCertificateInvalidKeyUsage) {
			errCode = CertificateCodeInvalidKeyUsage
			errField = "certificate.pfx_base64"
		}

		result.Errors = append(result.Errors, NewValidationError(
			errField,
			errCode,
			err.Error(),
		))
	}

	return result
}

// isValidBase64 checks if a string is valid base64 encoding.
func isValidBase64(s string) bool {
	if s == "" {
		return false
	}

	// Check for valid base64 characters
	// Base64 uses A-Z, a-z, 0-9, +, /, and = for padding
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}
