package xmlsigner

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
)

// generateTestCertificate creates a self-signed certificate for testing.
func generateTestCertificate(t *testing.T) *CertificateInfo {
	t.Helper()

	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	// Create certificate template
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "Test Certificate",
			Organization: []string{"Test Org"},
			Country:      []string{"BR"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour * 24 * 365),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		BasicConstraintsValid: true,
	}

	// Self-sign the certificate
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	// Parse the certificate
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	return &CertificateInfo{
		PrivateKey:  privateKey,
		Certificate: cert,
	}
}

// generateExpiredCertificate creates an expired certificate for testing.
func generateExpiredCertificate(t *testing.T) *CertificateInfo {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: "Expired Test Certificate",
		},
		NotBefore: time.Now().Add(-time.Hour * 48),
		NotAfter:  time.Now().Add(-time.Hour * 24), // Expired 24 hours ago
		KeyUsage:  x509.KeyUsageDigitalSignature,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	return &CertificateInfo{
		PrivateKey:  privateKey,
		Certificate: cert,
	}
}

// sampleDPSXML is a sample DPS XML document for testing.
const sampleDPSXML = `<?xml version="1.0" encoding="UTF-8"?>
<DPS xmlns="http://www.sped.fazenda.gov.br/nfse" versao="1.00">
  <infDPS Id="DPS355030812345678000199000010000000000000001">
    <tpAmb>2</tpAmb>
    <dhEmi>2024-01-15T10:30:00-03:00</dhEmi>
    <verAplic>1.0.0</verAplic>
    <serie>00001</serie>
    <nDPS>1</nDPS>
    <dCompet>2024-01-15</dCompet>
    <tpEmit>1</tpEmit>
    <cLocEmi>3550308</cLocEmi>
    <subst>2</subst>
    <prest>
      <CNPJ>12345678000199</CNPJ>
      <xNome>Test Provider</xNome>
      <regTrib>
        <opSimpNac>2</opSimpNac>
      </regTrib>
    </prest>
    <serv>
      <locPrest>
        <cLocPrestacao>3550308</cLocPrestacao>
      </locPrest>
      <cServ>
        <cTribNac>010101</cTribNac>
      </cServ>
      <xDescServ>Test Service</xDescServ>
    </serv>
    <valores>
      <vServPrest>
        <vServ>100.00</vServ>
      </vServPrest>
    </valores>
  </infDPS>
</DPS>`

func TestXMLSigner_SignDPS(t *testing.T) {
	certInfo := generateTestCertificate(t)
	signer := NewXMLSigner(certInfo)

	signedXML, err := signer.SignDPS(sampleDPSXML)
	if err != nil {
		t.Fatalf("Failed to sign DPS: %v", err)
	}

	// Verify the signed XML contains the Signature element
	if !strings.Contains(signedXML, "<Signature") {
		t.Error("Signed XML does not contain Signature element")
	}

	// Verify the signed XML contains SignedInfo
	if !strings.Contains(signedXML, "<SignedInfo") {
		t.Error("Signed XML does not contain SignedInfo element")
	}

	// Verify the signed XML contains SignatureValue
	if !strings.Contains(signedXML, "<SignatureValue") {
		t.Error("Signed XML does not contain SignatureValue element")
	}

	// Verify the signed XML contains KeyInfo with X509Certificate
	if !strings.Contains(signedXML, "<X509Certificate") {
		t.Error("Signed XML does not contain X509Certificate element")
	}

	// Verify the signed XML contains the correct reference URI
	if !strings.Contains(signedXML, `URI="#DPS355030812345678000199000010000000000000001"`) {
		t.Error("Signed XML does not contain correct reference URI")
	}

	// Verify the signed XML contains correct algorithms
	if !strings.Contains(signedXML, AlgorithmRSASHA256) {
		t.Error("Signed XML does not contain RSA-SHA256 algorithm")
	}

	if !strings.Contains(signedXML, AlgorithmExcC14N) {
		t.Error("Signed XML does not contain Exclusive C14N algorithm")
	}

	// Verify the signed XML is valid XML
	doc := etree.NewDocument()
	if err := doc.ReadFromString(signedXML); err != nil {
		t.Errorf("Signed XML is not valid: %v", err)
	}
}

func TestXMLSigner_SignDPSWithResult(t *testing.T) {
	certInfo := generateTestCertificate(t)
	signer := NewXMLSigner(certInfo)

	result, err := signer.SignDPSWithResult(sampleDPSXML)
	if err != nil {
		t.Fatalf("Failed to sign DPS: %v", err)
	}

	// Verify result fields
	if result.SignedXML == "" {
		t.Error("SignedXML is empty")
	}

	if result.DigestValue == "" {
		t.Error("DigestValue is empty")
	}

	if result.SignatureValue == "" {
		t.Error("SignatureValue is empty")
	}

	if result.ReferenceURI == "" {
		t.Error("ReferenceURI is empty")
	}

	// Verify DigestValue is valid base64
	if _, err := base64.StdEncoding.DecodeString(result.DigestValue); err != nil {
		t.Errorf("DigestValue is not valid base64: %v", err)
	}

	// Verify SignatureValue is valid base64
	sigValue := strings.ReplaceAll(result.SignatureValue, "\n", "")
	if _, err := base64.StdEncoding.DecodeString(sigValue); err != nil {
		t.Errorf("SignatureValue is not valid base64: %v", err)
	}
}

func TestXMLSigner_NilCertificate(t *testing.T) {
	signer := NewXMLSigner(nil)

	_, err := signer.SignDPS(sampleDPSXML)
	if err == nil {
		t.Error("Expected error for nil certificate")
	}

	if err != ErrSigningNilCertificate {
		t.Errorf("Expected ErrSigningNilCertificate, got: %v", err)
	}
}

func TestXMLSigner_NilPrivateKey(t *testing.T) {
	certInfo := generateTestCertificate(t)
	certInfo.PrivateKey = nil

	signer := NewXMLSigner(certInfo)

	_, err := signer.SignDPS(sampleDPSXML)
	if err == nil {
		t.Error("Expected error for nil private key")
	}
}

func TestXMLSigner_ExpiredCertificate(t *testing.T) {
	certInfo := generateExpiredCertificate(t)
	signer := NewXMLSigner(certInfo)

	_, err := signer.SignDPS(sampleDPSXML)
	if err == nil {
		t.Error("Expected error for expired certificate")
	}

	// Should contain expired error
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("Expected error to mention expired, got: %v", err)
	}
}

func TestXMLSigner_InvalidXML(t *testing.T) {
	certInfo := generateTestCertificate(t)
	signer := NewXMLSigner(certInfo)

	_, err := signer.SignDPS("not valid xml")
	if err == nil {
		t.Error("Expected error for invalid XML")
	}
}

func TestXMLSigner_MissingDPSElement(t *testing.T) {
	certInfo := generateTestCertificate(t)
	signer := NewXMLSigner(certInfo)

	xmlWithoutDPS := `<?xml version="1.0"?><root><data>test</data></root>`
	_, err := signer.SignDPS(xmlWithoutDPS)
	if err == nil {
		t.Error("Expected error for missing DPS element")
	}
}

func TestXMLSigner_MissingInfDPSElement(t *testing.T) {
	certInfo := generateTestCertificate(t)
	signer := NewXMLSigner(certInfo)

	xmlWithoutInfDPS := `<?xml version="1.0"?><DPS xmlns="http://www.sped.fazenda.gov.br/nfse"><data>test</data></DPS>`
	_, err := signer.SignDPS(xmlWithoutInfDPS)
	if err == nil {
		t.Error("Expected error for missing infDPS element")
	}
}

func TestXMLSigner_MissingIdAttribute(t *testing.T) {
	certInfo := generateTestCertificate(t)
	signer := NewXMLSigner(certInfo)

	xmlWithoutId := `<?xml version="1.0"?>
<DPS xmlns="http://www.sped.fazenda.gov.br/nfse">
  <infDPS>
    <tpAmb>2</tpAmb>
  </infDPS>
</DPS>`
	_, err := signer.SignDPS(xmlWithoutId)
	if err == nil {
		t.Error("Expected error for missing Id attribute")
	}

	if err != ErrSigningMissingID {
		t.Errorf("Expected ErrSigningMissingID, got: %v", err)
	}
}

func TestXMLSigner_SignatureConsistency(t *testing.T) {
	certInfo := generateTestCertificate(t)
	signer := NewXMLSigner(certInfo)

	// Sign the same document twice
	signedXML1, err := signer.SignDPSWithResult(sampleDPSXML)
	if err != nil {
		t.Fatalf("Failed to sign DPS (first time): %v", err)
	}

	signedXML2, err := signer.SignDPSWithResult(sampleDPSXML)
	if err != nil {
		t.Fatalf("Failed to sign DPS (second time): %v", err)
	}

	// Digest values should be the same (same content)
	if signedXML1.DigestValue != signedXML2.DigestValue {
		t.Error("Digest values should be consistent for the same content")
	}

	// Signature values may differ due to RSA padding randomness,
	// but both should be valid
	if signedXML1.SignatureValue == "" || signedXML2.SignatureValue == "" {
		t.Error("Signature values should not be empty")
	}
}
