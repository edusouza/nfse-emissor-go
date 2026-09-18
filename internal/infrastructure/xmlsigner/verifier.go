// Package xmlsigner provides XMLDSig digital signature functionality for NFS-e documents.
package xmlsigner

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/beevik/etree"
)

// Signature verification error types for specific error handling.
var (
	// ErrVerificationNoSignature indicates that no Signature element was found.
	ErrVerificationNoSignature = errors.New("no Signature element found in XML")

	// ErrVerificationNoSignedInfo indicates that the SignedInfo element is missing.
	ErrVerificationNoSignedInfo = errors.New("SignedInfo element not found in Signature")

	// ErrVerificationNoSignatureValue indicates that the SignatureValue element is missing.
	ErrVerificationNoSignatureValue = errors.New("SignatureValue element not found in Signature")

	// ErrVerificationNoKeyInfo indicates that the KeyInfo element is missing.
	ErrVerificationNoKeyInfo = errors.New("KeyInfo element not found in Signature")

	// ErrVerificationNoCertificate indicates that no X509Certificate was found.
	ErrVerificationNoCertificate = errors.New("X509Certificate not found in KeyInfo")

	// ErrVerificationNoReference indicates that no Reference element was found.
	ErrVerificationNoReference = errors.New("Reference element not found in SignedInfo")

	// ErrVerificationNoDigestValue indicates that the DigestValue element is missing.
	ErrVerificationNoDigestValue = errors.New("DigestValue element not found in Reference")

	// ErrVerificationDigestMismatch indicates that the digest verification failed.
	ErrVerificationDigestMismatch = errors.New("digest verification failed: computed digest does not match DigestValue")

	// ErrVerificationSignatureMismatch indicates that the signature verification failed.
	ErrVerificationSignatureMismatch = errors.New("signature verification failed: signature does not match SignedInfo")

	// ErrVerificationInvalidCertificate indicates that the certificate could not be parsed.
	ErrVerificationInvalidCertificate = errors.New("failed to parse X509 certificate")

	// ErrVerificationReferencedElementNotFound indicates that the referenced element was not found.
	ErrVerificationReferencedElementNotFound = errors.New("referenced element not found")

	// ErrVerificationUnsupportedAlgorithm indicates an unsupported signature algorithm.
	ErrVerificationUnsupportedAlgorithm = errors.New("unsupported signature algorithm")

	// ErrVerificationUnsupportedKeyType indicates the certificate key is not RSA.
	ErrVerificationUnsupportedKeyType = errors.New("unsupported key type: only RSA keys are supported")
)

// VerificationResult contains the result of an XML signature verification.
type VerificationResult struct {
	// Valid indicates whether the signature is valid.
	Valid bool `json:"valid"`

	// SignerCN is the Common Name from the signer's certificate subject.
	SignerCN string `json:"signer_cn,omitempty"`

	// SignerSerial is the certificate serial number.
	SignerSerial string `json:"signer_serial,omitempty"`

	// SignedElementID is the ID of the signed element (e.g., infDPS Id).
	SignedElementID string `json:"signed_element_id,omitempty"`

	// Errors contains a list of verification errors encountered.
	Errors []string `json:"errors,omitempty"`

	// Certificate contains the parsed certificate (not serialized to JSON).
	Certificate *x509.Certificate `json:"-"`
}

// AddError adds an error message to the verification result.
func (r *VerificationResult) AddError(err string) {
	r.Errors = append(r.Errors, err)
	r.Valid = false
}

// XMLVerifier provides XML digital signature verification functionality.
type XMLVerifier struct {
	// ValidateCertificate controls whether to validate the certificate dates.
	// Set to false to skip expiration checks (useful for testing).
	ValidateCertificate bool

	// CertificateValidator is used to validate the signer's certificate.
	CertificateValidator *CertificateValidator
}

// NewXMLVerifier creates a new XMLVerifier with default settings.
func NewXMLVerifier() *XMLVerifier {
	return &XMLVerifier{
		ValidateCertificate:  true,
		CertificateValidator: NewCertificateValidator(),
	}
}

// VerifySignature verifies the XMLDSig signature in a signed XML document.
//
// The verification process:
//  1. Parse XML and find Signature element
//  2. Extract SignedInfo, SignatureValue, KeyInfo/X509Certificate
//  3. Parse and optionally validate the certificate
//  4. Extract Reference URI and DigestValue
//  5. Find referenced element and canonicalize it
//  6. Verify digest matches
//  7. Canonicalize SignedInfo
//  8. Verify SignatureValue against canonicalized SignedInfo
//
// Parameters:
//   - signedXML: The signed XML document as a string
//
// Returns:
//   - *VerificationResult: The verification result with details
//   - error: Only returns error for fatal parsing errors; verification failures are in the result
func (v *XMLVerifier) VerifySignature(signedXML string) (*VerificationResult, error) {
	result := &VerificationResult{
		Valid:  true,
		Errors: make([]string, 0),
	}

	// Check for empty XML
	if signedXML == "" {
		return nil, fmt.Errorf("XML document is empty")
	}

	// Parse the XML document
	doc := etree.NewDocument()
	if err := doc.ReadFromString(signedXML); err != nil {
		return nil, fmt.Errorf("failed to parse XML document: %w", err)
	}

	// Find the Signature element
	signature := v.findSignatureElement(doc)
	if signature == nil {
		result.AddError(ErrVerificationNoSignature.Error())
		return result, nil
	}

	// Extract SignedInfo
	signedInfo := signature.FindElement("SignedInfo")
	if signedInfo == nil {
		result.AddError(ErrVerificationNoSignedInfo.Error())
		return result, nil
	}

	// Extract SignatureValue
	signatureValueElem := signature.FindElement("SignatureValue")
	if signatureValueElem == nil {
		result.AddError(ErrVerificationNoSignatureValue.Error())
		return result, nil
	}
	signatureValue := cleanBase64(signatureValueElem.Text())

	// Extract certificate from KeyInfo
	cert, err := v.extractCertificate(signature)
	if err != nil {
		result.AddError(err.Error())
		return result, nil
	}
	result.Certificate = cert
	result.SignerCN = cert.Subject.CommonName
	result.SignerSerial = cert.SerialNumber.String()

	// Validate certificate if enabled
	if v.ValidateCertificate {
		certInfo := &CertificateInfo{Certificate: cert}
		if err := v.CertificateValidator.Validate(certInfo); err != nil {
			result.AddError(fmt.Sprintf("certificate validation failed: %v", err))
		}
	}

	// Extract Reference from SignedInfo
	reference := signedInfo.FindElement("Reference")
	if reference == nil {
		result.AddError(ErrVerificationNoReference.Error())
		return result, nil
	}

	// Get the URI attribute to find the referenced element
	uriAttr := reference.SelectAttr("URI")
	if uriAttr == nil || uriAttr.Value == "" {
		result.AddError("Reference URI attribute is missing or empty")
		return result, nil
	}
	referenceURI := uriAttr.Value
	result.SignedElementID = strings.TrimPrefix(referenceURI, "#")

	// Extract expected DigestValue
	digestValueElem := reference.FindElement("DigestValue")
	if digestValueElem == nil {
		result.AddError(ErrVerificationNoDigestValue.Error())
		return result, nil
	}
	expectedDigest := cleanBase64(digestValueElem.Text())

	// Find the referenced element
	referencedElement := v.findElementByID(doc, result.SignedElementID)
	if referencedElement == nil {
		result.AddError(fmt.Sprintf("%s: %s", ErrVerificationReferencedElementNotFound.Error(), result.SignedElementID))
		return result, nil
	}

	// Verify the digest
	if !v.verifyDigest(referencedElement, expectedDigest) {
		result.AddError(ErrVerificationDigestMismatch.Error())
	}

	// Verify the signature
	if err := v.verifySignatureValue(signedInfo, signatureValue, cert); err != nil {
		result.AddError(fmt.Sprintf("%s: %v", ErrVerificationSignatureMismatch.Error(), err))
	}

	return result, nil
}

// findSignatureElement finds the Signature element in the document.
// It handles both namespaced and non-namespaced Signature elements.
func (v *XMLVerifier) findSignatureElement(doc *etree.Document) *etree.Element {
	// Try without namespace first (most common in NFS-e documents)
	signature := doc.FindElement("//Signature")
	if signature != nil {
		return signature
	}

	// Try finding Signature under DPS
	signature = doc.FindElement("//DPS/Signature")
	if signature != nil {
		return signature
	}

	// Try recursive search for any element named Signature
	root := doc.Root()
	if root != nil {
		return findSignatureRecursive(root)
	}

	return nil
}

// findSignatureRecursive recursively searches for a Signature element.
func findSignatureRecursive(element *etree.Element) *etree.Element {
	if element.Tag == "Signature" {
		return element
	}
	for _, child := range element.ChildElements() {
		if found := findSignatureRecursive(child); found != nil {
			return found
		}
	}
	return nil
}

// extractCertificate extracts and parses the X509 certificate from KeyInfo.
func (v *XMLVerifier) extractCertificate(signature *etree.Element) (*x509.Certificate, error) {
	// Find KeyInfo
	keyInfo := signature.FindElement("KeyInfo")
	if keyInfo == nil {
		return nil, ErrVerificationNoKeyInfo
	}

	// Find X509Data
	x509Data := keyInfo.FindElement("X509Data")
	if x509Data == nil {
		return nil, ErrVerificationNoCertificate
	}

	// Find X509Certificate
	x509CertElem := x509Data.FindElement("X509Certificate")
	if x509CertElem == nil {
		return nil, ErrVerificationNoCertificate
	}

	// Decode the base64 certificate
	certBase64 := cleanBase64(x509CertElem.Text())
	certDER, err := base64.StdEncoding.DecodeString(certBase64)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid base64 encoding: %v", ErrVerificationInvalidCertificate, err)
	}

	// Parse the certificate
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVerificationInvalidCertificate, err)
	}

	return cert, nil
}

// findElementByID finds an element by its Id attribute.
func (v *XMLVerifier) findElementByID(doc *etree.Document, id string) *etree.Element {
	// Try various XPath expressions to find the element
	element := doc.FindElement(fmt.Sprintf("//*[@Id='%s']", id))
	if element != nil {
		return element
	}

	// Try with lowercase 'id'
	element = doc.FindElement(fmt.Sprintf("//*[@id='%s']", id))
	if element != nil {
		return element
	}

	// Try with xml:id
	element = doc.FindElement(fmt.Sprintf("//*[@xml:id='%s']", id))
	return element
}

// verifyDigest verifies the digest of the referenced element.
func (v *XMLVerifier) verifyDigest(element *etree.Element, expectedDigest string) bool {
	// Canonicalize the element (applying enveloped-signature transform which removes Signature)
	canonicalContent, err := CanonicalizeSigned(element)
	if err != nil {
		return false
	}

	// Compute SHA-256 digest
	digest := sha256.Sum256(canonicalContent)
	computedDigest := base64.StdEncoding.EncodeToString(digest[:])

	return computedDigest == expectedDigest
}

// verifySignatureValue verifies the signature value against the canonicalized SignedInfo.
func (v *XMLVerifier) verifySignatureValue(signedInfo *etree.Element, signatureValue string, cert *x509.Certificate) error {
	// Get the RSA public key from the certificate
	rsaPubKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return ErrVerificationUnsupportedKeyType
	}

	// Canonicalize SignedInfo
	canonicalSignedInfo, err := Canonicalize(signedInfo)
	if err != nil {
		return fmt.Errorf("failed to canonicalize SignedInfo: %w", err)
	}

	// Decode the signature value
	sigBytes, err := base64.StdEncoding.DecodeString(signatureValue)
	if err != nil {
		return fmt.Errorf("failed to decode SignatureValue: %w", err)
	}

	// Compute the hash of the canonicalized SignedInfo
	hash := sha256.Sum256(canonicalSignedInfo)

	// Verify the signature using RSA-SHA256
	err = rsa.VerifyPKCS1v15(rsaPubKey, crypto.SHA256, hash[:], sigBytes)
	if err != nil {
		return fmt.Errorf("RSA signature verification failed: %w", err)
	}

	return nil
}

// cleanBase64 removes whitespace from a base64-encoded string.
func cleanBase64(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\t", "")
	return strings.TrimSpace(s)
}

// VerifyDPSSignature verifies a signed DPS XML document specifically.
// This is a convenience method that also validates the DPS structure.
//
// Parameters:
//   - signedDPSXML: The signed DPS XML document
//
// Returns:
//   - *VerificationResult: The verification result
//   - error: Only returns error for fatal parsing errors
func (v *XMLVerifier) VerifyDPSSignature(signedDPSXML string) (*VerificationResult, error) {
	// First perform general signature verification
	result, err := v.VerifySignature(signedDPSXML)
	if err != nil {
		return nil, err
	}

	// Additional DPS-specific checks
	doc := etree.NewDocument()
	if err := doc.ReadFromString(signedDPSXML); err != nil {
		return nil, fmt.Errorf("failed to parse DPS XML: %w", err)
	}

	// Verify this is a DPS document
	dps := doc.FindElement("//DPS")
	if dps == nil {
		result.AddError("not a valid DPS document: DPS element not found")
		return result, nil
	}

	// Verify infDPS exists
	infDPS := dps.FindElement("infDPS")
	if infDPS == nil {
		result.AddError("not a valid DPS document: infDPS element not found")
		return result, nil
	}

	// Verify the signed element is infDPS
	idAttr := infDPS.SelectAttr("Id")
	if idAttr == nil {
		result.AddError("infDPS element is missing Id attribute")
	} else if result.SignedElementID != idAttr.Value {
		result.AddError(fmt.Sprintf("signature does not reference infDPS element: expected %s, got %s",
			idAttr.Value, result.SignedElementID))
	}

	return result, nil
}
