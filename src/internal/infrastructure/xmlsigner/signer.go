// Package xmlsigner provides XMLDSig digital signature functionality for NFS-e documents.
package xmlsigner

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/beevik/etree"
)

// XMLDSig algorithm identifiers as defined by W3C standards.
const (
	// AlgorithmRSASHA256 is the RSA-SHA256 signature algorithm.
	AlgorithmRSASHA256 = "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"

	// AlgorithmSHA256 is the SHA-256 digest algorithm.
	AlgorithmSHA256 = "http://www.w3.org/2001/04/xmlenc#sha256"

	// AlgorithmEnvelopedSignature is the enveloped signature transform.
	AlgorithmEnvelopedSignature = "http://www.w3.org/2000/09/xmldsig#enveloped-signature"

	// NamespaceXMLDSig is the XML Digital Signature namespace.
	NamespaceXMLDSig = "http://www.w3.org/2000/09/xmldsig#"

	// NamespaceNFSe is the Brazilian NFS-e namespace.
	NamespaceNFSe = "http://www.sped.fazenda.gov.br/nfse"
)

// Signing error types for specific error handling.
var (
	// ErrSigningNilCertificate indicates that no certificate was provided for signing.
	ErrSigningNilCertificate = errors.New("certificate is nil")

	// ErrSigningNilPrivateKey indicates that no private key is available.
	ErrSigningNilPrivateKey = errors.New("private key is nil")

	// ErrSigningInvalidXML indicates that the XML document is invalid.
	ErrSigningInvalidXML = errors.New("invalid XML document")

	// ErrSigningMissingElement indicates that a required element is missing.
	ErrSigningMissingElement = errors.New("required element not found")

	// ErrSigningMissingID indicates that the Id attribute is missing.
	ErrSigningMissingID = errors.New("Id attribute not found on element")

	// ErrSigningFailed indicates that the signing operation failed.
	ErrSigningFailed = errors.New("signing operation failed")
)

// XMLSigner provides XML digital signature functionality.
// It implements XMLDSig with RSA-SHA256 for Brazilian NFS-e documents.
type XMLSigner struct {
	certInfo  *CertificateInfo
	validator *CertificateValidator
}

// NewXMLSigner creates a new XMLSigner with the given certificate.
//
// Parameters:
//   - certInfo: The certificate information containing the private key and certificate
//
// Returns:
//   - *XMLSigner: A new signer instance
func NewXMLSigner(certInfo *CertificateInfo) *XMLSigner {
	return &XMLSigner{
		certInfo:  certInfo,
		validator: NewCertificateValidator(),
	}
}

// SignDPS signs a DPS (Documento de Prestacao de Servicos) XML document.
// This is the main entry point for signing NFS-e DPS documents.
//
// The signing process:
//  1. Parse the XML document
//  2. Find the infDPS element and get its Id attribute
//  3. Canonicalize the infDPS element using exclusive C14N
//  4. Compute SHA-256 digest of the canonicalized content
//  5. Build the SignedInfo element with the digest
//  6. Canonicalize SignedInfo
//  7. Sign the canonicalized SignedInfo with RSA-SHA256
//  8. Build the complete Signature element
//  9. Append the Signature to the DPS element
//  10. Serialize and return the signed XML
//
// Parameters:
//   - dpsXML: The unsigned DPS XML document as a string
//
// Returns:
//   - string: The signed DPS XML document
//   - error: Any error encountered during signing
//
// Example:
//
//	signer := NewXMLSigner(certInfo)
//	signedXML, err := signer.SignDPS(unsignedXML)
//	if err != nil {
//	    return fmt.Errorf("failed to sign DPS: %w", err)
//	}
func (s *XMLSigner) SignDPS(dpsXML string) (string, error) {
	// Validate certificate before signing
	if err := s.validateCertificate(); err != nil {
		return "", err
	}

	// Parse the XML document
	doc := etree.NewDocument()
	if err := doc.ReadFromString(dpsXML); err != nil {
		return "", fmt.Errorf("%w: %v", ErrSigningInvalidXML, err)
	}

	// Find the DPS element
	dps := doc.FindElement("//DPS")
	if dps == nil {
		return "", fmt.Errorf("%w: DPS element", ErrSigningMissingElement)
	}

	// Find the infDPS element
	infDPS := dps.FindElement("infDPS")
	if infDPS == nil {
		return "", fmt.Errorf("%w: infDPS element", ErrSigningMissingElement)
	}

	// Get the Id attribute from infDPS
	idAttr := infDPS.SelectAttr("Id")
	if idAttr == nil {
		return "", ErrSigningMissingID
	}
	referenceURI := "#" + idAttr.Value

	// Create and append the signature
	signature, err := s.createSignature(infDPS, referenceURI)
	if err != nil {
		return "", err
	}

	// Append the signature element after infDPS
	dps.AddChild(signature)

	// Serialize the signed document
	doc.Indent(2)
	signedXML, err := doc.WriteToString()
	if err != nil {
		return "", fmt.Errorf("failed to serialize signed XML: %w", err)
	}

	return signedXML, nil
}

// SignDPSCompact signs a DPS XML document and returns a compact (non-indented) result.
// This is useful when whitespace matters or for transmission efficiency.
func (s *XMLSigner) SignDPSCompact(dpsXML string) (string, error) {
	// Validate certificate before signing
	if err := s.validateCertificate(); err != nil {
		return "", err
	}

	// Parse the XML document
	doc := etree.NewDocument()
	if err := doc.ReadFromString(dpsXML); err != nil {
		return "", fmt.Errorf("%w: %v", ErrSigningInvalidXML, err)
	}

	// Find the DPS element
	dps := doc.FindElement("//DPS")
	if dps == nil {
		return "", fmt.Errorf("%w: DPS element", ErrSigningMissingElement)
	}

	// Find the infDPS element
	infDPS := dps.FindElement("infDPS")
	if infDPS == nil {
		return "", fmt.Errorf("%w: infDPS element", ErrSigningMissingElement)
	}

	// Get the Id attribute from infDPS
	idAttr := infDPS.SelectAttr("Id")
	if idAttr == nil {
		return "", ErrSigningMissingID
	}
	referenceURI := "#" + idAttr.Value

	// Create and append the signature
	signature, err := s.createSignature(infDPS, referenceURI)
	if err != nil {
		return "", err
	}

	// Append the signature element after infDPS
	dps.AddChild(signature)

	// Serialize without indentation
	signedXML, err := doc.WriteToString()
	if err != nil {
		return "", fmt.Errorf("failed to serialize signed XML: %w", err)
	}

	return signedXML, nil
}

// validateCertificate checks that the signer has a valid certificate.
func (s *XMLSigner) validateCertificate() error {
	if s.certInfo == nil {
		return ErrSigningNilCertificate
	}

	if s.certInfo.PrivateKey == nil {
		return ErrSigningNilPrivateKey
	}

	// Validate the certificate for signing
	return s.validator.ValidateForSigning(s.certInfo)
}

// createSignature creates the XMLDSig Signature element.
func (s *XMLSigner) createSignature(elementToSign *etree.Element, referenceURI string) (*etree.Element, error) {
	// Step 1: Canonicalize the element to be signed (without any existing signature)
	canonicalContent, err := CanonicalizeSigned(elementToSign)
	if err != nil {
		return nil, fmt.Errorf("failed to canonicalize element: %w", err)
	}

	// Step 2: Compute the digest of the canonicalized content
	digest := sha256.Sum256(canonicalContent)
	digestBase64 := base64.StdEncoding.EncodeToString(digest[:])

	// Step 3: Build the SignedInfo element
	signedInfo := s.buildSignedInfo(referenceURI, digestBase64)

	// Step 4: Canonicalize SignedInfo for signing
	canonicalSignedInfo, err := Canonicalize(signedInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to canonicalize SignedInfo: %w", err)
	}

	// Step 5: Sign the canonicalized SignedInfo
	signatureValue, err := s.signData(canonicalSignedInfo)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSigningFailed, err)
	}
	signatureBase64 := base64.StdEncoding.EncodeToString(signatureValue)

	// Step 6: Get the certificate for KeyInfo
	certBase64, err := s.certInfo.GetCertificateBase64()
	if err != nil {
		return nil, fmt.Errorf("failed to encode certificate: %w", err)
	}

	// Step 7: Build the complete Signature element
	signature := s.buildSignatureElement(signedInfo, signatureBase64, certBase64)

	return signature, nil
}

// buildSignedInfo creates the SignedInfo element with the digest.
func (s *XMLSigner) buildSignedInfo(referenceURI, digestBase64 string) *etree.Element {
	signedInfo := etree.NewElement("SignedInfo")
	signedInfo.CreateAttr("xmlns", NamespaceXMLDSig)

	// CanonicalizationMethod
	canonMethod := signedInfo.CreateElement("CanonicalizationMethod")
	canonMethod.CreateAttr("Algorithm", AlgorithmExcC14N)

	// SignatureMethod
	sigMethod := signedInfo.CreateElement("SignatureMethod")
	sigMethod.CreateAttr("Algorithm", AlgorithmRSASHA256)

	// Reference
	reference := signedInfo.CreateElement("Reference")
	reference.CreateAttr("URI", referenceURI)

	// Transforms
	transforms := reference.CreateElement("Transforms")

	// Transform 1: Enveloped signature
	transform1 := transforms.CreateElement("Transform")
	transform1.CreateAttr("Algorithm", AlgorithmEnvelopedSignature)

	// Transform 2: Exclusive canonicalization
	transform2 := transforms.CreateElement("Transform")
	transform2.CreateAttr("Algorithm", AlgorithmExcC14N)

	// DigestMethod
	digestMethod := reference.CreateElement("DigestMethod")
	digestMethod.CreateAttr("Algorithm", AlgorithmSHA256)

	// DigestValue
	digestValue := reference.CreateElement("DigestValue")
	digestValue.SetText(digestBase64)

	return signedInfo
}

// buildSignatureElement creates the complete Signature element.
func (s *XMLSigner) buildSignatureElement(signedInfo *etree.Element, signatureBase64, certBase64 string) *etree.Element {
	signature := etree.NewElement("Signature")
	signature.CreateAttr("xmlns", NamespaceXMLDSig)

	// Copy SignedInfo (without the xmlns since it will be inherited)
	signedInfoCopy := signedInfo.Copy()
	signedInfoCopy.RemoveAttr("xmlns")
	signature.AddChild(signedInfoCopy)

	// SignatureValue
	signatureValue := signature.CreateElement("SignatureValue")
	signatureValue.SetText(formatBase64(signatureBase64, 76))

	// KeyInfo
	keyInfo := signature.CreateElement("KeyInfo")

	// X509Data
	x509Data := keyInfo.CreateElement("X509Data")

	// X509Certificate
	x509Cert := x509Data.CreateElement("X509Certificate")
	x509Cert.SetText(formatBase64(certBase64, 76))

	return signature
}

// signData signs data using RSA-SHA256.
func (s *XMLSigner) signData(data []byte) ([]byte, error) {
	// Compute SHA-256 hash of the data
	hash := sha256.Sum256(data)

	// Sign using PKCS#1 v1.5
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.certInfo.PrivateKey, crypto.SHA256, hash[:])
	if err != nil {
		return nil, fmt.Errorf("RSA signing failed: %w", err)
	}

	return signature, nil
}

// formatBase64 formats a base64 string with line breaks at the specified width.
// This improves readability of the signed XML.
func formatBase64(s string, width int) string {
	if width <= 0 || len(s) <= width {
		return s
	}

	var result strings.Builder
	for i := 0; i < len(s); i += width {
		end := i + width
		if end > len(s) {
			end = len(s)
		}
		if i > 0 {
			result.WriteString("\n")
		}
		result.WriteString(s[i:end])
	}
	return result.String()
}

// SigningResult contains the result of a signing operation.
type SigningResult struct {
	// SignedXML is the complete signed XML document.
	SignedXML string

	// DigestValue is the SHA-256 digest of the signed content (base64).
	DigestValue string

	// SignatureValue is the RSA signature value (base64).
	SignatureValue string

	// ReferenceURI is the URI reference to the signed element.
	ReferenceURI string
}

// SignDPSWithResult signs a DPS document and returns detailed results.
// This is useful for debugging and audit logging.
func (s *XMLSigner) SignDPSWithResult(dpsXML string) (*SigningResult, error) {
	// Validate certificate before signing
	if err := s.validateCertificate(); err != nil {
		return nil, err
	}

	// Parse the XML document
	doc := etree.NewDocument()
	if err := doc.ReadFromString(dpsXML); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSigningInvalidXML, err)
	}

	// Find the DPS element
	dps := doc.FindElement("//DPS")
	if dps == nil {
		return nil, fmt.Errorf("%w: DPS element", ErrSigningMissingElement)
	}

	// Find the infDPS element
	infDPS := dps.FindElement("infDPS")
	if infDPS == nil {
		return nil, fmt.Errorf("%w: infDPS element", ErrSigningMissingElement)
	}

	// Get the Id attribute from infDPS
	idAttr := infDPS.SelectAttr("Id")
	if idAttr == nil {
		return nil, ErrSigningMissingID
	}
	referenceURI := "#" + idAttr.Value

	// Canonicalize and compute digest
	canonicalContent, err := CanonicalizeSigned(infDPS)
	if err != nil {
		return nil, fmt.Errorf("failed to canonicalize element: %w", err)
	}

	digest := sha256.Sum256(canonicalContent)
	digestBase64 := base64.StdEncoding.EncodeToString(digest[:])

	// Build SignedInfo
	signedInfo := s.buildSignedInfo(referenceURI, digestBase64)

	// Canonicalize and sign
	canonicalSignedInfo, err := Canonicalize(signedInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to canonicalize SignedInfo: %w", err)
	}

	signatureValue, err := s.signData(canonicalSignedInfo)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSigningFailed, err)
	}
	signatureBase64 := base64.StdEncoding.EncodeToString(signatureValue)

	// Get certificate
	certBase64, err := s.certInfo.GetCertificateBase64()
	if err != nil {
		return nil, fmt.Errorf("failed to encode certificate: %w", err)
	}

	// Build signature element
	signature := s.buildSignatureElement(signedInfo, signatureBase64, certBase64)
	dps.AddChild(signature)

	// Serialize
	doc.Indent(2)
	signedXML, err := doc.WriteToString()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize signed XML: %w", err)
	}

	return &SigningResult{
		SignedXML:      signedXML,
		DigestValue:    digestBase64,
		SignatureValue: signatureBase64,
		ReferenceURI:   referenceURI,
	}, nil
}
