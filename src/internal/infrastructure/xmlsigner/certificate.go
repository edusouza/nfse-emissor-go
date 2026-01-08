// Package xmlsigner provides XMLDSig digital signature functionality for NFS-e documents.
// It implements certificate parsing, validation, and XML signing according to Brazilian
// NFS-e (National Electronic Service Invoice) specifications.
package xmlsigner

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/pkcs12"
)

// CertificateInfo contains the parsed certificate information from a PFX/P12 file.
// This structure holds all the cryptographic material needed for XML signing operations.
type CertificateInfo struct {
	// PrivateKey is the RSA private key extracted from the PFX file.
	// This is used to create digital signatures.
	PrivateKey *rsa.PrivateKey

	// Certificate is the X.509 certificate containing the public key.
	// This is included in the KeyInfo section of the XML signature.
	Certificate *x509.Certificate

	// Chain contains any intermediate certificates in the chain.
	// These may be needed for complete certificate validation.
	Chain []*x509.Certificate
}

// Certificate parsing error types for specific error handling.
var (
	// ErrNilPFXData indicates that the provided PFX data is nil or empty.
	ErrNilPFXData = errors.New("PFX data is nil or empty")

	// ErrInvalidPFXFormat indicates that the PFX data could not be parsed.
	ErrInvalidPFXFormat = errors.New("invalid PFX format or incorrect password")

	// ErrNoPrivateKey indicates that no private key was found in the PFX file.
	ErrNoPrivateKey = errors.New("no private key found in PFX file")

	// ErrNoCertificate indicates that no certificate was found in the PFX file.
	ErrNoCertificate = errors.New("no certificate found in PFX file")

	// ErrUnsupportedKeyType indicates that the private key type is not RSA.
	ErrUnsupportedKeyType = errors.New("unsupported private key type: only RSA keys are supported")

	// ErrInvalidBase64 indicates that the base64 encoding is invalid.
	ErrInvalidBase64 = errors.New("invalid base64 encoding")
)

// ParsePFX parses a PFX/P12 file and extracts the private key and certificates.
// The PFX format (also known as PKCS#12) is a common format for storing private keys
// and certificates together, typically password-protected.
//
// Parameters:
//   - pfxData: The raw bytes of the PFX file
//   - password: The password protecting the PFX file
//
// Returns:
//   - *CertificateInfo: The parsed certificate information, or nil on error
//   - error: Any error encountered during parsing
//
// Example:
//
//	pfxBytes, err := os.ReadFile("certificate.pfx")
//	if err != nil {
//	    return err
//	}
//	certInfo, err := ParsePFX(pfxBytes, "mypassword")
//	if err != nil {
//	    return fmt.Errorf("failed to parse certificate: %w", err)
//	}
func ParsePFX(pfxData []byte, password string) (*CertificateInfo, error) {
	if len(pfxData) == 0 {
		return nil, ErrNilPFXData
	}

	// Decode the PFX data
	// The pkcs12.Decode function extracts the private key and certificate from
	// a PKCS#12 (PFX) encoded data blob.
	privateKey, certificate, err := pkcs12.Decode(pfxData, password)
	if err != nil {
		// The error from pkcs12.Decode can be cryptic, so we wrap it
		// with a more user-friendly message
		return nil, fmt.Errorf("%w: %v", ErrInvalidPFXFormat, err)
	}

	// Verify we got a private key
	if privateKey == nil {
		return nil, ErrNoPrivateKey
	}

	// Verify we got a certificate
	if certificate == nil {
		return nil, ErrNoCertificate
	}

	// Verify the private key is RSA (required for XMLDSig with RSA-SHA256)
	rsaKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%w: got %T", ErrUnsupportedKeyType, privateKey)
	}

	// Note: The pkcs12 package's Decode function returns only the leaf certificate.
	// For chain certificates, more complex parsing would be needed, but for
	// NFS-e signing purposes, the leaf certificate is typically sufficient.
	return &CertificateInfo{
		PrivateKey:  rsaKey,
		Certificate: certificate,
		Chain:       nil, // Chain certificates not extracted with basic Decode
	}, nil
}

// ParsePFXBase64 parses base64-encoded PFX data and extracts the certificate information.
// This is a convenience function for handling PFX data transmitted in base64 format,
// which is common in API requests.
//
// Parameters:
//   - pfxBase64: The base64-encoded PFX data
//   - password: The password protecting the PFX file
//
// Returns:
//   - *CertificateInfo: The parsed certificate information, or nil on error
//   - error: Any error encountered during parsing
//
// Example:
//
//	certInfo, err := ParsePFXBase64(request.Certificate.PFXBase64, request.Certificate.Password)
//	if err != nil {
//	    return fmt.Errorf("failed to parse certificate: %w", err)
//	}
func ParsePFXBase64(pfxBase64, password string) (*CertificateInfo, error) {
	if pfxBase64 == "" {
		return nil, ErrNilPFXData
	}

	// Decode the base64 string
	pfxData, err := base64.StdEncoding.DecodeString(pfxBase64)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidBase64, err)
	}

	return ParsePFX(pfxData, password)
}

// GetCertificateBase64 returns the certificate encoded as base64 for inclusion
// in the X509Certificate element of XMLDSig signatures.
// The certificate is encoded in DER format and then base64 encoded.
//
// Returns:
//   - string: The base64-encoded certificate
//   - error: Any error encountered during encoding
func (c *CertificateInfo) GetCertificateBase64() (string, error) {
	if c.Certificate == nil {
		return "", ErrNoCertificate
	}

	// The Certificate.Raw field contains the DER-encoded certificate
	return base64.StdEncoding.EncodeToString(c.Certificate.Raw), nil
}

// GetSubjectCN returns the Common Name (CN) from the certificate subject.
// This is typically the name of the entity the certificate was issued to.
func (c *CertificateInfo) GetSubjectCN() string {
	if c.Certificate == nil {
		return ""
	}
	return c.Certificate.Subject.CommonName
}

// GetIssuerCN returns the Common Name (CN) from the certificate issuer.
// This identifies the Certificate Authority that issued the certificate.
func (c *CertificateInfo) GetIssuerCN() string {
	if c.Certificate == nil {
		return ""
	}
	return c.Certificate.Issuer.CommonName
}

// GetSerialNumber returns the certificate serial number as a string.
func (c *CertificateInfo) GetSerialNumber() string {
	if c.Certificate == nil {
		return ""
	}
	return c.Certificate.SerialNumber.String()
}
