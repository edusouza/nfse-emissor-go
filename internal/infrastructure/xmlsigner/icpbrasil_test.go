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

	"github.com/edusouza/nfse-emissor-go/pkg/cnpjcpf"
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
	const (
		cnpj       = "12345678000195"
		outroCNPJ  = "11222333000181"
		semDigitos = "12345678000100"
	)
	cnpjSAN := func(t *testing.T, tag int, valor string) asn1.RawValue {
		return otherName(t, oidICPBrasilCNPJ, tag, valor)
	}

	tests := []struct {
		name     string
		cn       string
		names    func(t *testing.T) []asn1.RawValue
		wantCNPJ string
		wantNome string
	}{
		{
			name:     "OCTET STRING, como o DOC-ICP-04 especifica",
			cn:       "EMPRESA TESTE LTDA",
			names:    func(t *testing.T) []asn1.RawValue { return []asn1.RawValue{cnpjSAN(t, asn1.TagOctetString, cnpj)} },
			wantCNPJ: cnpj,
			wantNome: "EMPRESA TESTE LTDA",
		},
		{
			name:     "PrintableString",
			cn:       "EMPRESA TESTE LTDA",
			names:    func(t *testing.T) []asn1.RawValue { return []asn1.RawValue{cnpjSAN(t, asn1.TagPrintableString, cnpj)} },
			wantCNPJ: cnpj,
			wantNome: "EMPRESA TESTE LTDA",
		},
		{
			name:     "UTF8String",
			cn:       "EMPRESA TESTE LTDA",
			names:    func(t *testing.T) []asn1.RawValue { return []asn1.RawValue{cnpjSAN(t, asn1.TagUTF8String, cnpj)} },
			wantCNPJ: cnpj,
			wantNome: "EMPRESA TESTE LTDA",
		},
		{
			name:     "IA5String",
			cn:       "EMPRESA TESTE LTDA",
			names:    func(t *testing.T) []asn1.RawValue { return []asn1.RawValue{cnpjSAN(t, asn1.TagIA5String, cnpj)} },
			wantCNPJ: cnpj,
			wantNome: "EMPRESA TESTE LTDA",
		},
		{
			name: "entre os dados do responsavel e um DNS",
			cn:   "EMPRESA TESTE LTDA",
			names: func(t *testing.T) []asn1.RawValue {
				return []asn1.RawValue{
					dnsName("empresa.example"),
					otherName(t, oidICPBrasilResponsavel, asn1.TagOctetString, outroCNPJ),
					cnpjSAN(t, asn1.TagOctetString, cnpj),
				}
			},
			wantCNPJ: cnpj,
			wantNome: "EMPRESA TESTE LTDA",
		},
		{
			// A valid CNPJ under a sibling identifier must never be taken for
			// the holder's: those carry the responsible person's data.
			name: "CNPJ valido sob outro OID",
			cn:   "EMPRESA TESTE LTDA",
			names: func(t *testing.T) []asn1.RawValue {
				return []asn1.RawValue{otherName(t, oidICPBrasilResponsavel, asn1.TagOctetString, cnpj)}
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
			name: "digitos verificadores errados",
			cn:   "EMPRESA TESTE LTDA",
			names: func(t *testing.T) []asn1.RawValue {
				return []asn1.RawValue{cnpjSAN(t, asn1.TagOctetString, semDigitos)}
			},
			wantCNPJ: "",
			wantNome: "EMPRESA TESTE LTDA",
		},
		{
			// The Sefin identifies the signer by the subjectAltName, so a
			// common name that disagrees with it does not get a say.
			name:     "subjectAltName tem precedencia sobre o common name",
			cn:       "EMPRESA TESTE LTDA:" + outroCNPJ,
			names:    func(t *testing.T) []asn1.RawValue { return []asn1.RawValue{cnpjSAN(t, asn1.TagOctetString, cnpj)} },
			wantCNPJ: cnpj,
			wantNome: "EMPRESA TESTE LTDA",
		},
		{
			name:     "common name com digitos invalidos",
			cn:       "EMPRESA TESTE LTDA:" + semDigitos,
			names:    func(t *testing.T) []asn1.RawValue { return []asn1.RawValue{cnpjSAN(t, asn1.TagOctetString, cnpj)} },
			wantCNPJ: cnpj,
			wantNome: "EMPRESA TESTE LTDA:" + semDigitos,
		},
		{
			// A CNPJ entry that is there but unreadable makes the certificate
			// ambiguous; the common name does not get to fill the gap.
			name: "entrada ilegivel nao cede ao common name",
			cn:   "EMPRESA TESTE LTDA:" + outroCNPJ,
			names: func(t *testing.T) []asn1.RawValue {
				return []asn1.RawValue{cnpjSAN(t, asn1.TagOctetString, semDigitos)}
			},
			wantCNPJ: "",
			wantNome: "EMPRESA TESTE LTDA",
		},
		{
			name:     "sem a entrada do CNPJ, vale o common name",
			cn:       "EMPRESA TESTE LTDA:" + outroCNPJ,
			names:    func(t *testing.T) []asn1.RawValue { return []asn1.RawValue{dnsName("empresa.example")} },
			wantCNPJ: outroCNPJ,
			wantNome: "EMPRESA TESTE LTDA",
		},
		{
			name: "duas entradas com o mesmo CNPJ",
			cn:   "EMPRESA TESTE LTDA",
			names: func(t *testing.T) []asn1.RawValue {
				return []asn1.RawValue{cnpjSAN(t, asn1.TagOctetString, cnpj), cnpjSAN(t, asn1.TagPrintableString, cnpj)}
			},
			wantCNPJ: cnpj,
			wantNome: "EMPRESA TESTE LTDA",
		},
		{
			name: "duas entradas com CNPJs diferentes",
			cn:   "EMPRESA TESTE LTDA",
			names: func(t *testing.T) []asn1.RawValue {
				return []asn1.RawValue{cnpjSAN(t, asn1.TagOctetString, cnpj), cnpjSAN(t, asn1.TagOctetString, outroCNPJ)}
			},
			wantCNPJ: "",
			wantNome: "EMPRESA TESTE LTDA",
		},
		{
			name: "entrada invalida seguida de valida",
			cn:   "EMPRESA TESTE LTDA",
			names: func(t *testing.T) []asn1.RawValue {
				return []asn1.RawValue{cnpjSAN(t, asn1.TagOctetString, semDigitos), cnpjSAN(t, asn1.TagOctetString, cnpj)}
			},
			wantCNPJ: "",
			wantNome: "EMPRESA TESTE LTDA",
		},
		{
			// Only fourteen digits are a CNPJ; stripping the rest would
			// assemble one out of whatever text happens to hold digits.
			name: "CNPJ formatado",
			cn:   "EMPRESA TESTE LTDA",
			names: func(t *testing.T) []asn1.RawValue {
				return []asn1.RawValue{cnpjSAN(t, asn1.TagOctetString, "12.345.678/0001-95")}
			},
			wantCNPJ: "",
			wantNome: "EMPRESA TESTE LTDA",
		},
		{
			name:     "valor de tipo inesperado",
			cn:       "EMPRESA TESTE LTDA",
			names:    func(t *testing.T) []asn1.RawValue { return []asn1.RawValue{cnpjSAN(t, asn1.TagInteger, cnpj)} },
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

// cnpjOtherNameBytes builds the DER of an otherName by hand, for the layouts
// asn1.Marshal refuses to produce.
func cnpjOtherNameBytes(t *testing.T, wrapper []byte) []byte {
	t.Helper()

	typeID, err := asn1.Marshal(oidICPBrasilCNPJ)
	if err != nil {
		t.Fatal(err)
	}
	body := append(typeID, wrapper...)
	return append([]byte{0xa0, byte(len(body))}, body...)
}

func sanDe(itens ...[]byte) []byte {
	var body []byte
	for _, item := range itens {
		body = append(body, item...)
	}
	return append([]byte{0x30, byte(len(body))}, body...)
}

// A certificate whose extension does not decode is a certificate without a
// CNPJ in it, not a reason to fail. Each case breaks exactly one layer, under
// the right identifier, so it is that layer's check that has to refuse it.
func TestSubjectCNPJComExtensaoMalformada(t *testing.T) {
	const cnpj = "12345678000195"
	octet := append([]byte{0x04, 14}, cnpj...)

	valido := cnpjOtherNameBytes(t, append([]byte{0xa0, byte(len(octet))}, octet...))

	casos := map[string][]byte{
		"vazia":              {0x30, 0x00},
		"sem OID":            sanDe([]byte{0xa0, 0x02, 0x05, 0x00}),
		"sem o [0] do valor": sanDe(cnpjOtherNameBytes(t, nil)),
		"valor sob [1]":      sanDe(cnpjOtherNameBytes(t, append([]byte{0xa1, byte(len(octet))}, octet...))),
		// Context-specific [4] carries the same number as an OCTET STRING would.
		"valor de contexto":     sanDe(cnpjOtherNameBytes(t, append([]byte{0xa0, byte(len(octet))}, append([]byte{0x84, 14}, cnpj...)...))),
		"bytes depois do valor": sanDe(cnpjOtherNameBytes(t, append([]byte{0xa0, byte(len(octet) + 2)}, append(octet, 0x05, 0x00)...))),
		"bytes depois do [0]":   sanDe(cnpjOtherNameBytes(t, append(append([]byte{0xa0, byte(len(octet))}, octet...), 0x05, 0x00))),
		"bytes depois da lista": append(sanDe(valido), 0x00),
		"GeneralName primitivo": sanDe(append([]byte{0x80, byte(len(valido) - 2)}, valido[2:]...)),
		"OCTET STRING composta": sanDe(cnpjOtherNameBytes(t, append([]byte{0xa0, byte(len(octet))}, append([]byte{0x24, 14}, cnpj...)...))),
		"mais de 14 digitos":    sanDe(cnpjOtherNameBytes(t, append([]byte{0xa0, 17}, append([]byte{0x04, 15}, cnpj+"0"...)...))),
	}

	// The valid layout must pass, or every refusal above proves nothing.
	casos["controle"] = sanDe(valido)

	for nome, value := range casos {
		t.Run(nome, func(t *testing.T) {
			info := &CertificateInfo{Certificate: &x509.Certificate{
				Subject:    pkix.Name{CommonName: "EMPRESA TESTE LTDA"},
				Extensions: []pkix.Extension{{Id: oidSubjectAltName, Value: value}},
			}}
			want := ""
			if nome == "controle" {
				want = cnpj
			}
			if got := info.SubjectCNPJ(); got != want {
				t.Errorf("SubjectCNPJ() = %q, esperava %q", got, want)
			}
		})
	}
}

// The extension comes from a file the user hands over; no byte sequence in
// it may panic the parser, and whatever comes out must be a valid CNPJ.
func FuzzSubjectAltNameCNPJ(f *testing.F) {
	octet := append([]byte{0x04, 14}, "12345678000195"...)
	typeID, _ := asn1.Marshal(oidICPBrasilCNPJ)
	body := append(typeID, append([]byte{0xa0, byte(len(octet))}, octet...)...)
	name := append([]byte{0xa0, byte(len(body))}, body...)

	f.Add(append([]byte{0x30, byte(len(name))}, name...))
	f.Add([]byte{0x30, 0x00})
	f.Add([]byte{0x30, 0x04, 0xa0, 0x02, 0x05, 0x00})

	f.Fuzz(func(t *testing.T, value []byte) {
		cert := &x509.Certificate{Extensions: []pkix.Extension{{Id: oidSubjectAltName, Value: value}}}
		got, found := subjectAltNameCNPJ(cert)
		if got != "" && (!found || !cnpjcpf.ValidateCNPJ(got) || len(got) != cnpjcpf.CNPJLength) {
			t.Errorf("subjectAltNameCNPJ() = %q, %v", got, found)
		}
	})
}
