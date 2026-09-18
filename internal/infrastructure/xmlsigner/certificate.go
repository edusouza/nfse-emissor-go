// Package xmlsigner provides XMLDSig digital signature functionality for NFS-e documents.
// It implements certificate parsing, validation, and XML signing according to Brazilian
// NFS-e (National Electronic Service Invoice) specifications.
package xmlsigner

import (
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	pkcs12 "software.sslmate.com/src/go-pkcs12"

	"github.com/edusouza/nfse-emissor-go/pkg/cnpjcpf"
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

	// DecodeChain also returns the intermediate certificates, which ICP-Brasil
	// A1 files carry and which some validators expect alongside the leaf.
	//
	// TrustStore-free decoding is intentional: the caller decides what to trust.
	privateKey, certificate, chain, err := pkcs12.DecodeChain(pfxData, password)
	if err != nil {
		// The error from pkcs12 can be cryptic, so we wrap it with a more
		// user-friendly message.
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

	return &CertificateInfo{
		PrivateKey:  rsaKey,
		Certificate: certificate,
		Chain:       chain,
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

// SubjectCNPJ returns the CNPJ the certificate was issued to, or an empty
// string when it cannot be read.
//
// ICP-Brasil writes the holder of an e-CNPJ as "RAZAO SOCIAL:CNPJ" in the
// subject's common name, so the taxpayer number travels with the file. Reading
// it saves the emitter from asking for something it already has — and a number
// the user cannot mistype.
//
// The check digits are verified: a common name that merely ends in fourteen
// digits is not evidence enough to fill a fiscal document with.
func (c *CertificateInfo) SubjectCNPJ() string {
	cn := c.GetSubjectCN()

	i := strings.LastIndex(cn, ":")
	if i < 0 {
		return ""
	}

	candidate := cnpjcpf.CleanCNPJ(cn[i+1:])
	if !cnpjcpf.ValidateCNPJ(candidate) {
		return ""
	}
	return candidate
}

// SubjectHolderName returns the holder's name without the CNPJ suffix that
// ICP-Brasil appends to the common name.
//
// For an e-CNPJ that name is the razão social as the Receita Federal has it,
// which is exactly what the DPS carries in xNome.
func (c *CertificateInfo) SubjectHolderName() string {
	cn := c.GetSubjectCN()

	if c.SubjectCNPJ() == "" {
		return cn
	}
	return strings.TrimSpace(cn[:strings.LastIndex(cn, ":")])
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

// TLSCertificate adapts the parsed A1 certificate for use as a mutual-TLS
// client certificate.
//
// The Sistema Nacional NFS-e identifies the caller by this certificate; there
// are no API keys. The intermediate chain is included so that servers which
// require the full path can build it.
func (c *CertificateInfo) TLSCertificate() (*tls.Certificate, error) {
	if c == nil || c.Certificate == nil {
		return nil, ErrNoCertificate
	}
	if c.PrivateKey == nil {
		return nil, ErrNoPrivateKey
	}

	chain := [][]byte{c.Certificate.Raw}
	for _, intermediate := range c.Chain {
		chain = append(chain, intermediate.Raw)
	}

	return &tls.Certificate{
		Certificate: chain,
		PrivateKey:  c.PrivateKey,
		Leaf:        c.Certificate,
	}, nil
}
