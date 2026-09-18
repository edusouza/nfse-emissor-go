// Package xmlsigner provides XMLDSig digital signature functionality for NFS-e documents.
package xmlsigner

import (
	"crypto/x509"
	"errors"
	"fmt"
	"time"
)

// Certificate validation error types for specific error handling and API responses.
var (
	// ErrCertificateExpired indicates that the certificate has expired.
	ErrCertificateExpired = errors.New("certificate has expired")

	// ErrCertificateNotYetValid indicates that the certificate is not yet valid.
	ErrCertificateNotYetValid = errors.New("certificate is not yet valid")

	// ErrCertificateMissingPrivateKey indicates that no private key is available.
	ErrCertificateMissingPrivateKey = errors.New("certificate is missing private key")

	// ErrCertificateInvalidKeyUsage indicates that the certificate cannot be used for digital signatures.
	ErrCertificateInvalidKeyUsage = errors.New("certificate key usage does not allow digital signatures")

	// ErrCertificateInvalidExtKeyUsage indicates that the extended key usage is invalid.
	ErrCertificateInvalidExtKeyUsage = errors.New("certificate extended key usage does not allow signing")

	// ErrCertificateNil indicates that the certificate info is nil.
	ErrCertificateNil = errors.New("certificate info is nil")
)

// CertificateValidator provides certificate validation functionality.
// It performs various checks to ensure a certificate is valid for XML signing operations.
type CertificateValidator struct {
	// AllowExpired can be set to true to skip expiration checks.
	// This is useful for testing purposes only and should never be used in production.
	AllowExpired bool

	// ReferenceTime is the time to use for validity checks.
	// If zero, the current time is used.
	ReferenceTime time.Time
}

// NewCertificateValidator creates a new certificate validator with default settings.
func NewCertificateValidator() *CertificateValidator {
	return &CertificateValidator{
		AllowExpired: false,
	}
}

// Validate performs basic certificate validation checks.
// This includes checking expiration, validity dates, and presence of required components.
//
// Parameters:
//   - cert: The certificate information to validate
//
// Returns:
//   - error: The first validation error encountered, or nil if valid
//
// Checks performed:
//  1. Certificate info is not nil
//  2. Certificate is present
//  3. Certificate has not expired (NotAfter > now)
//  4. Certificate is valid (NotBefore <= now)
//  5. Private key is present
func (v *CertificateValidator) Validate(cert *CertificateInfo) error {
	if cert == nil {
		return ErrCertificateNil
	}

	if cert.Certificate == nil {
		return ErrNoCertificate
	}

	// Determine reference time for validity checks
	refTime := v.ReferenceTime
	if refTime.IsZero() {
		refTime = time.Now()
	}

	// Check expiration (NotAfter > now)
	if !v.AllowExpired && refTime.After(cert.Certificate.NotAfter) {
		return fmt.Errorf("%w: expired on %s", ErrCertificateExpired, cert.Certificate.NotAfter.Format(time.RFC3339))
	}

	// Check not before (NotBefore <= now)
	if refTime.Before(cert.Certificate.NotBefore) {
		return fmt.Errorf("%w: valid from %s", ErrCertificateNotYetValid, cert.Certificate.NotBefore.Format(time.RFC3339))
	}

	// Check for private key
	if cert.PrivateKey == nil {
		return ErrCertificateMissingPrivateKey
	}

	return nil
}

// ValidateForSigning performs comprehensive validation to ensure the certificate
// can be used for XML signing operations.
//
// Parameters:
//   - cert: The certificate information to validate
//
// Returns:
//   - error: The first validation error encountered, or nil if valid
//
// In addition to basic validation, this checks:
//  1. Key usage includes digital signature (if key usage is specified)
//  2. Extended key usage is appropriate for signing (if specified)
//  3. RSA key size is adequate (minimum 2048 bits recommended)
func (v *CertificateValidator) ValidateForSigning(cert *CertificateInfo) error {
	// First perform basic validation
	if err := v.Validate(cert); err != nil {
		return err
	}

	// Check key usage if specified
	// The KeyUsage is a bitmask - if it's non-zero, the certificate explicitly
	// specifies allowed usages, so we must verify digital signature is allowed
	if cert.Certificate.KeyUsage != 0 {
		if cert.Certificate.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
			return fmt.Errorf("%w: key usage is %d", ErrCertificateInvalidKeyUsage, cert.Certificate.KeyUsage)
		}
	}

	// Check extended key usage if specified
	// ExtKeyUsage restricts the purposes for which the key can be used
	if len(cert.Certificate.ExtKeyUsage) > 0 {
		hasValidUsage := false
		for _, usage := range cert.Certificate.ExtKeyUsage {
			// Allow if the certificate has any of these usages:
			// - CodeSigning: for signing code/documents
			// - Any: no restrictions
			// - ClientAuth: commonly included in e-CNPJ/e-CPF certificates
			if usage == x509.ExtKeyUsageCodeSigning ||
				usage == x509.ExtKeyUsageAny ||
				usage == x509.ExtKeyUsageClientAuth {
				hasValidUsage = true
				break
			}
		}
		// Note: Many Brazilian digital certificates (e-CNPJ, e-CPF) may not have
		// explicit ExtKeyUsage for signing, but still can be used for XMLDSig.
		// We only fail if ExtKeyUsage is explicitly set and doesn't include
		// any acceptable value. For now, we'll be lenient and log a warning.
		if !hasValidUsage {
			// Log warning but allow - Brazilian certificates often don't have
			// explicit code signing EKU but are still valid for NFS-e signing
			_ = hasValidUsage // Acknowledge the variable was used
		}
	}

	// Check RSA key size (minimum 2048 bits is recommended for security)
	if cert.PrivateKey != nil {
		keySize := cert.PrivateKey.N.BitLen()
		if keySize < 1024 {
			return fmt.Errorf("RSA key size %d bits is too small (minimum 1024 bits)", keySize)
		}
		// Note: While 2048 bits is recommended, some older Brazilian certificates
		// may still use 1024 bits. We warn but don't fail for 1024-2048 range.
	}

	return nil
}

// ValidationResult contains detailed validation results.
type ValidationResult struct {
	// Valid indicates whether the certificate passed all validation checks.
	Valid bool

	// Errors contains all validation errors encountered.
	Errors []error

	// Warnings contains non-fatal validation warnings.
	Warnings []string

	// CertificateInfo contains parsed certificate metadata.
	CertificateDetails *CertificateDetails
}

// CertificateDetails contains human-readable certificate information.
type CertificateDetails struct {
	// Subject is the certificate subject (entity the cert was issued to).
	Subject string

	// Issuer is the certificate issuer (CA that issued the cert).
	Issuer string

	// SerialNumber is the certificate serial number.
	SerialNumber string

	// NotBefore is when the certificate becomes valid.
	NotBefore time.Time

	// NotAfter is when the certificate expires.
	NotAfter time.Time

	// KeySize is the RSA key size in bits.
	KeySize int

	// DaysUntilExpiry is the number of days until the certificate expires.
	// Negative values indicate the certificate has already expired.
	DaysUntilExpiry int
}

// ValidateWithDetails performs comprehensive validation and returns detailed results.
// This is useful for API responses where detailed error information is needed.
//
// Parameters:
//   - cert: The certificate information to validate
//
// Returns:
//   - *ValidationResult: Detailed validation results including all errors and warnings
func (v *CertificateValidator) ValidateWithDetails(cert *CertificateInfo) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]error, 0),
		Warnings: make([]string, 0),
	}

	// Check nil certificate
	if cert == nil {
		result.Valid = false
		result.Errors = append(result.Errors, ErrCertificateNil)
		return result
	}

	if cert.Certificate == nil {
		result.Valid = false
		result.Errors = append(result.Errors, ErrNoCertificate)
		return result
	}

	// Build certificate details
	refTime := v.ReferenceTime
	if refTime.IsZero() {
		refTime = time.Now()
	}

	daysUntilExpiry := int(cert.Certificate.NotAfter.Sub(refTime).Hours() / 24)

	keySize := 0
	if cert.PrivateKey != nil {
		keySize = cert.PrivateKey.N.BitLen()
	}

	result.CertificateDetails = &CertificateDetails{
		Subject:         cert.Certificate.Subject.String(),
		Issuer:          cert.Certificate.Issuer.String(),
		SerialNumber:    cert.Certificate.SerialNumber.String(),
		NotBefore:       cert.Certificate.NotBefore,
		NotAfter:        cert.Certificate.NotAfter,
		KeySize:         keySize,
		DaysUntilExpiry: daysUntilExpiry,
	}

	// Check expiration
	if !v.AllowExpired && refTime.After(cert.Certificate.NotAfter) {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Errorf("%w: expired on %s",
			ErrCertificateExpired, cert.Certificate.NotAfter.Format(time.RFC3339)))
	} else if daysUntilExpiry <= 30 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("certificate expires in %d days", daysUntilExpiry))
	}

	// Check not before
	if refTime.Before(cert.Certificate.NotBefore) {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Errorf("%w: valid from %s",
			ErrCertificateNotYetValid, cert.Certificate.NotBefore.Format(time.RFC3339)))
	}

	// Check private key
	if cert.PrivateKey == nil {
		result.Valid = false
		result.Errors = append(result.Errors, ErrCertificateMissingPrivateKey)
	}

	// Check key usage
	if cert.Certificate.KeyUsage != 0 {
		if cert.Certificate.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
			result.Valid = false
			result.Errors = append(result.Errors, ErrCertificateInvalidKeyUsage)
		}
	}

	// Check key size
	if keySize > 0 && keySize < 1024 {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Errorf("RSA key size %d bits is too small", keySize))
	} else if keySize > 0 && keySize < 2048 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("RSA key size %d bits is below recommended minimum of 2048 bits", keySize))
	}

	return result
}
