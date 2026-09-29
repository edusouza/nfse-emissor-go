package xmlsigner

import (
	"crypto/x509"
	"encoding/asn1"

	"github.com/edusouza/nfse-emissor-go/pkg/cnpjcpf"
)

var (
	// oidSubjectAltName is the X.509 subjectAltName extension (RFC 5280).
	oidSubjectAltName = asn1.ObjectIdentifier{2, 5, 29, 17}

	// oidICPBrasilCNPJ is the otherName DOC-ICP-04 reserves for the CNPJ of
	// the legal entity holding an e-CNPJ. Its siblings under 2.16.76.1.3 carry
	// the responsible person's data — birth date and CPF among them — so only
	// this exact identifier is read.
	oidICPBrasilCNPJ = asn1.ObjectIdentifier{2, 16, 76, 1, 3, 3}
)

// subjectAltNameCNPJ reads the CNPJ from the ICP-Brasil otherName in the
// subjectAltName extension, or returns an empty string.
//
// crypto/x509 parses only the DNS, e-mail, IP and URI alternative names and
// drops otherName, so the extension is walked here. Anything that does not
// decode is treated as absent: this is a fallback, and a malformed extension
// is reason to ask the user for the CNPJ, not to fail.
func subjectAltNameCNPJ(cert *x509.Certificate) string {
	for _, ext := range cert.Extensions {
		if !ext.Id.Equal(oidSubjectAltName) {
			continue
		}

		var names []asn1.RawValue
		if rest, err := asn1.Unmarshal(ext.Value, &names); err != nil || len(rest) > 0 {
			return ""
		}

		for _, name := range names {
			// GeneralName's otherName is [0] IMPLICIT OtherName.
			if name.Class != asn1.ClassContextSpecific || name.Tag != 0 || !name.IsCompound {
				continue
			}
			typeID, value, ok := parseOtherName(name.Bytes)
			if !ok || !typeID.Equal(oidICPBrasilCNPJ) {
				continue
			}
			candidate := cnpjcpf.CleanCNPJ(value)
			if cnpjcpf.ValidateCNPJ(candidate) {
				return candidate
			}
		}
	}
	return ""
}

// parseOtherName decodes the body of an OtherName:
//
//	OtherName ::= SEQUENCE {
//	    type-id    OBJECT IDENTIFIER,
//	    value      [0] EXPLICIT ANY DEFINED BY type-id }
//
// DOC-ICP-04 specifies OCTET STRING for the ICP-Brasil values, but authorities
// have been seen writing PrintableString and UTF8String, so any of the string
// types is accepted and read as text.
func parseOtherName(body []byte) (asn1.ObjectIdentifier, string, bool) {
	var typeID asn1.ObjectIdentifier
	rest, err := asn1.Unmarshal(body, &typeID)
	if err != nil {
		return nil, "", false
	}

	var wrapper asn1.RawValue
	if _, err := asn1.Unmarshal(rest, &wrapper); err != nil ||
		wrapper.Class != asn1.ClassContextSpecific || wrapper.Tag != 0 || !wrapper.IsCompound {
		return nil, "", false
	}

	var value asn1.RawValue
	if _, err := asn1.Unmarshal(wrapper.Bytes, &value); err != nil || value.Class != asn1.ClassUniversal {
		return nil, "", false
	}

	switch value.Tag {
	case asn1.TagOctetString, asn1.TagPrintableString, asn1.TagUTF8String, asn1.TagIA5String:
		return typeID, string(value.Bytes), true
	}
	return nil, "", false
}
