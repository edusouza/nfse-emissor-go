package xmlsigner

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"testing"
	"time"
)

// Other ICP-Brasil identifiers that sit next to the CNPJ in the same
// extension. 2.16.76.1.3.4 carries the responsible person's birth date and
// CPF on an e-CNPJ; 2.16.76.1.3.1 carries the holder's on an e-CPF.
var (
	oidICPBrasilResponsavel = asn1.ObjectIdentifier{2, 16, 76, 1, 3, 4}
	oidICPBrasilTitularCPF  = asn1.ObjectIdentifier{2, 16, 76, 1, 3, 1}
)

// otherName encodes a GeneralName otherName the way DOC-ICP-04 lays it out.
func otherName(t *testing.T, oid asn1.ObjectIdentifier, tag int, value string) asn1.RawValue {
	t.Helper()

	inner, err := asn1.Marshal(asn1.RawValue{Class: asn1.ClassUniversal, Tag: tag, Bytes: []byte(value)})
	if err != nil {
		t.Fatal(err)
	}
	explicit, err := asn1.Marshal(asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: inner})
	if err != nil {
		t.Fatal(err)
	}
	typeID, err := asn1.Marshal(oid)
	if err != nil {
		t.Fatal(err)
	}
	return asn1.RawValue{
		Class:      asn1.ClassContextSpecific,
		Tag:        0,
		IsCompound: true,
		Bytes:      append(typeID, explicit...),
	}
}

func dnsName(name string) asn1.RawValue {
	return asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 2, Bytes: []byte(name)}
}

// certificateWithSAN issues a real certificate, so the test goes through the
// same parsing crypto/x509 applies to a PFX read from disk.
func certificateWithSAN(t *testing.T, cn string, names ...asn1.RawValue) *CertificateInfo {
	t.Helper()

	san, err := asn1.Marshal(names)
	if err != nil {
		t.Fatal(err)
	}
	return certificateWithExtension(t, cn, san)
}

func certificateWithExtension(t *testing.T, cn string, sanValue []byte) *CertificateInfo {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:    big.NewInt(1),
		Subject:         pkix.Name{CommonName: cn},
		NotBefore:       time.Now().Add(-time.Hour),
		NotAfter:        time.Now().Add(time.Hour),
		ExtraExtensions: []pkix.Extension{{Id: oidSubjectAltName, Value: sanValue}},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return &CertificateInfo{Certificate: cert}
}

func TestSubjectCNPJPeloSubjectAltName(t *testing.T) {
	const cnpj = "12345678000195"

	tests := []struct {
		name     string
		cn       string
		names    func(t *testing.T) []asn1.RawValue
		wantCNPJ string
		wantNome string
	}{
		{
			name: "OCTET STRING, como o DOC-ICP-04 especifica",
			cn:   "EMPRESA TESTE LTDA",
			names: func(t *testing.T) []asn1.RawValue {
				return []asn1.RawValue{otherName(t, oidICPBrasilCNPJ, asn1.TagOctetString, cnpj)}
			},
			wantCNPJ: cnpj,
			wantNome: "EMPRESA TESTE LTDA",
		},
		{
			name: "PrintableString",
			cn:   "EMPRESA TESTE LTDA",
			names: func(t *testing.T) []asn1.RawValue {
				return []asn1.RawValue{otherName(t, oidICPBrasilCNPJ, asn1.TagPrintableString, cnpj)}
			},
			wantCNPJ: cnpj,
			wantNome: "EMPRESA TESTE LTDA",
		},
		{
			name: "entre os dados do responsavel e um DNS",
			cn:   "EMPRESA TESTE LTDA",
			names: func(t *testing.T) []asn1.RawValue {
				return []asn1.RawValue{
					dnsName("empresa.example"),
					otherName(t, oidICPBrasilResponsavel, asn1.TagOctetString, "0101198012345678909000000000000000000000000000"),
					otherName(t, oidICPBrasilCNPJ, asn1.TagOctetString, cnpj),
				}
			},
			wantCNPJ: cnpj,
			wantNome: "EMPRESA TESTE LTDA",
		},
		{
			name: "digitos verificadores errados",
			cn:   "EMPRESA TESTE LTDA",
			names: func(t *testing.T) []asn1.RawValue {
				return []asn1.RawValue{otherName(t, oidICPBrasilCNPJ, asn1.TagOctetString, "12345678000100")}
			},
			wantCNPJ: "",
			wantNome: "EMPRESA TESTE LTDA",
		},
		{
			// The responsible person's CPF must never be mistaken for the
			// holder's CNPJ, even when it is the only number around.
			name: "so os dados do responsavel",
			cn:   "EMPRESA TESTE LTDA",
			names: func(t *testing.T) []asn1.RawValue {
				return []asn1.RawValue{otherName(t, oidICPBrasilResponsavel, asn1.TagOctetString, "01011980"+cnpj)}
			},
			wantCNPJ: "",
			wantNome: "EMPRESA TESTE LTDA",
		},
		{
			name: "e-CPF",
			cn:   "FULANO DE TAL:12345678909",
			names: func(t *testing.T) []asn1.RawValue {
				return []asn1.RawValue{otherName(t, oidICPBrasilTitularCPF, asn1.TagOctetString, "0101198012345678909")}
			},
			wantCNPJ: "",
			wantNome: "FULANO DE TAL:12345678909",
		},
		{
			// The common name is what every A1 in practice carries, so it
			// is read first and wins a disagreement.
			name: "common name tem precedencia",
			cn:   "EMPRESA TESTE LTDA:11222333000181",
			names: func(t *testing.T) []asn1.RawValue {
				return []asn1.RawValue{otherName(t, oidICPBrasilCNPJ, asn1.TagOctetString, cnpj)}
			},
			wantCNPJ: "11222333000181",
			wantNome: "EMPRESA TESTE LTDA",
		},
		{
			name: "valor de tipo inesperado",
			cn:   "EMPRESA TESTE LTDA",
			names: func(t *testing.T) []asn1.RawValue {
				return []asn1.RawValue{otherName(t, oidICPBrasilCNPJ, asn1.TagInteger, cnpj)}
			},
			wantCNPJ: "",
			wantNome: "EMPRESA TESTE LTDA",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := certificateWithSAN(t, tt.cn, tt.names(t)...)

			if got := info.SubjectCNPJ(); got != tt.wantCNPJ {
				t.Errorf("SubjectCNPJ() = %q, esperava %q", got, tt.wantCNPJ)
			}
			if got := info.SubjectHolderName(); got != tt.wantNome {
				t.Errorf("SubjectHolderName() = %q, esperava %q", got, tt.wantNome)
			}
		})
	}
}

// A certificate whose extension does not decode is a certificate without a
// CNPJ in it, not a reason to fail.
func TestSubjectCNPJComExtensaoMalformada(t *testing.T) {
	for name, value := range map[string][]byte{
		"vazia":             {0x30, 0x00},
		"otherName sem OID": {0x30, 0x04, 0xa0, 0x02, 0x05, 0x00},
		"otherName sem [0]": {0x30, 0x07, 0xa0, 0x05, 0x06, 0x03, 0x60, 0x4c, 0x01},
	} {
		t.Run(name, func(t *testing.T) {
			info := &CertificateInfo{Certificate: &x509.Certificate{
				Subject:    pkix.Name{CommonName: "EMPRESA TESTE LTDA"},
				Extensions: []pkix.Extension{{Id: oidSubjectAltName, Value: value}},
			}}
			if got := info.SubjectCNPJ(); got != "" {
				t.Errorf("SubjectCNPJ() = %q, esperava vazio", got)
			}
		})
	}
}

// The extension comes from a file the user hands over; no byte sequence in
// it may panic the parser.
func FuzzSubjectAltNameCNPJ(f *testing.F) {
	f.Add([]byte{0x30, 0x00})
	f.Add([]byte{0x30, 0x04, 0xa0, 0x02, 0x05, 0x00})

	f.Fuzz(func(t *testing.T, value []byte) {
		cert := &x509.Certificate{Extensions: []pkix.Extension{{Id: oidSubjectAltName, Value: value}}}
		if got := subjectAltNameCNPJ(cert); got != "" && len(got) != 14 {
			t.Errorf("subjectAltNameCNPJ() = %q", got)
		}
	})
}
