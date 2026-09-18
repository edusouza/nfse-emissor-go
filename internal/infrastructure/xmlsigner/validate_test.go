package xmlsigner

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"
)

func TestCertificateValidator_Validate(t *testing.T) {
	validator := NewCertificateValidator()

	t.Run("valid certificate", func(t *testing.T) {
		certInfo := generateValidCertificate(t)
		err := validator.Validate(certInfo)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("nil certificate info", func(t *testing.T) {
		err := validator.Validate(nil)
		if err != ErrCertificateNil {
			t.Errorf("Expected ErrCertificateNil, got: %v", err)
		}
	})

	t.Run("nil certificate", func(t *testing.T) {
		certInfo := &CertificateInfo{
			PrivateKey:  generateRSAKey(t),
			Certificate: nil,
		}
		err := validator.Validate(certInfo)
		if err != ErrNoCertificate {
			t.Errorf("Expected ErrNoCertificate, got: %v", err)
		}
	})

	t.Run("nil private key", func(t *testing.T) {
		certInfo := generateValidCertificate(t)
		certInfo.PrivateKey = nil
		err := validator.Validate(certInfo)
		if err != ErrCertificateMissingPrivateKey {
			t.Errorf("Expected ErrCertificateMissingPrivateKey, got: %v", err)
		}
	})

	t.Run("expired certificate", func(t *testing.T) {
		certInfo := generateCertificateWithDates(t, time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour))
		err := validator.Validate(certInfo)
		if err == nil {
			t.Error("Expected error for expired certificate")
		}
	})

	t.Run("not yet valid certificate", func(t *testing.T) {
		certInfo := generateCertificateWithDates(t, time.Now().Add(24*time.Hour), time.Now().Add(48*time.Hour))
		err := validator.Validate(certInfo)
		if err == nil {
			t.Error("Expected error for not-yet-valid certificate")
		}
	})
}

func TestCertificateValidator_ValidateForSigning(t *testing.T) {
	validator := NewCertificateValidator()

	t.Run("valid signing certificate", func(t *testing.T) {
		certInfo := generateValidCertificate(t)
		err := validator.ValidateForSigning(certInfo)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("certificate without digital signature usage", func(t *testing.T) {
		certInfo := generateCertificateWithKeyUsage(t, x509.KeyUsageKeyEncipherment)
		err := validator.ValidateForSigning(certInfo)
		if err == nil {
			t.Error("Expected error for certificate without digital signature usage")
		}
	})

	t.Run("certificate with digital signature usage", func(t *testing.T) {
		certInfo := generateCertificateWithKeyUsage(t, x509.KeyUsageDigitalSignature)
		err := validator.ValidateForSigning(certInfo)
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	// Note: Test for small RSA key is skipped because Go's crypto/rsa package
	// no longer allows generating keys smaller than 1024 bits for security reasons.
	// The validator does check key size, but we cannot test with very small keys.
}

func TestCertificateValidator_ValidateWithDetails(t *testing.T) {
	validator := NewCertificateValidator()

	t.Run("valid certificate", func(t *testing.T) {
		certInfo := generateValidCertificate(t)
		result := validator.ValidateWithDetails(certInfo)

		if !result.Valid {
			t.Error("Expected valid result")
		}

		if len(result.Errors) != 0 {
			t.Errorf("Expected no errors, got: %v", result.Errors)
		}

		if result.CertificateDetails == nil {
			t.Error("Expected certificate details")
		}

		if result.CertificateDetails.Subject == "" {
			t.Error("Expected non-empty subject")
		}

		if result.CertificateDetails.KeySize == 0 {
			t.Error("Expected non-zero key size")
		}
	})

	t.Run("expiring soon certificate", func(t *testing.T) {
		// Certificate expiring in 15 days
		certInfo := generateCertificateWithDates(t, time.Now().Add(-time.Hour), time.Now().Add(15*24*time.Hour))
		result := validator.ValidateWithDetails(certInfo)

		if !result.Valid {
			t.Error("Certificate should still be valid")
		}

		// Should have a warning about expiring soon
		if len(result.Warnings) == 0 {
			t.Error("Expected warning about expiring certificate")
		}
	})

	t.Run("nil certificate info", func(t *testing.T) {
		result := validator.ValidateWithDetails(nil)

		if result.Valid {
			t.Error("Expected invalid result for nil certificate")
		}

		if len(result.Errors) == 0 {
			t.Error("Expected errors for nil certificate")
		}
	})
}

func TestCertificateValidator_AllowExpired(t *testing.T) {
	validator := &CertificateValidator{
		AllowExpired: true,
	}

	certInfo := generateCertificateWithDates(t, time.Now().Add(-48*time.Hour), time.Now().Add(-24*time.Hour))

	err := validator.Validate(certInfo)
	if err != nil {
		t.Errorf("Expected no error when AllowExpired is true, got: %v", err)
	}
}

func TestCertificateValidator_ReferenceTime(t *testing.T) {
	// Set reference time to a specific point
	validator := &CertificateValidator{
		ReferenceTime: time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC),
	}

	// Certificate valid from 2024-01-01 to 2024-12-31
	certInfo := generateCertificateWithDates(t,
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
	)

	err := validator.Validate(certInfo)
	if err != nil {
		t.Errorf("Expected no error for reference time within validity period, got: %v", err)
	}

	// Set reference time to after expiration
	validator.ReferenceTime = time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	err = validator.Validate(certInfo)
	if err == nil {
		t.Error("Expected error for reference time after expiration")
	}
}

// Helper functions

func generateValidCertificate(t *testing.T) *CertificateInfo {
	t.Helper()
	return generateCertificateWithDates(t, time.Now().Add(-time.Hour), time.Now().Add(365*24*time.Hour))
}

func generateRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}
	return key
}

func generateCertificateWithDates(t *testing.T, notBefore, notAfter time.Time) *CertificateInfo {
	t.Helper()

	privateKey := generateRSAKey(t)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName:   "Test Certificate",
			Organization: []string{"Test Org"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,
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

func generateCertificateWithKeyUsage(t *testing.T, keyUsage x509.KeyUsage) *CertificateInfo {
	t.Helper()

	privateKey := generateRSAKey(t)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: "Test Certificate",
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:  keyUsage,
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
