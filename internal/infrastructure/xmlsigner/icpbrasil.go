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
// subjectAltName extension.
//
// found reports whether the extension holds a CNPJ otherName at all. When it
// does, its answer is final: an entry that fails to decode or to validate, or
// two entries naming different companies, make the certificate ambiguous, and
// an ambiguous certificate returns "" rather than letting the common name
// speak for it. DOC-ICP-04 allows a single entry of exactly fourteen digits.
//
// crypto/x509 parses only the DNS, e-mail, IP and URI alternative names and
// drops otherName, so the extension is walked here. It parses no more than
// once per certificate, and x509.ParseCertificate already refuses a
// certificate with two subjectAltName extensions.
func subjectAltNameCNPJ(cert *x509.Certificate) (cnpj string, found bool) {
	for _, ext := range cert.Extensions {
		if !ext.Id.Equal(oidSubjectAltName) {
			continue
		}

		var names []asn1.RawValue
		if rest, err := asn1.Unmarshal(ext.Value, &names); err != nil || len(rest) > 0 {
			return "", false
		}

		for _, name := range names {
			// GeneralName's otherName is [0] IMPLICIT OtherName.
			if name.Class != asn1.ClassContextSpecific || name.Tag != 0 || !name.IsCompound {
				continue
			}
			typeID, value, ok := parseOtherName(name.Bytes)
			if typeID == nil || !typeID.Equal(oidICPBrasilCNPJ) {
				continue
			}

			found = true
			if !ok || !cnpjcpf.ValidateCNPJ(value) || (cnpj != "" && cnpj != value) {
				return "", true
			}
			cnpj = value
		}
		return cnpj, found
	}
	return "", false
}

// parseOtherName decodes the body of an OtherName:
//
//	OtherName ::= SEQUENCE {
//	    type-id    OBJECT IDENTIFIER,
//	    value      [0] EXPLICIT ANY DEFINED BY type-id }
//
// typeID is returned whenever it decodes, so that a malformed value under the
// CNPJ identifier is told apart from an otherName that is not ours. value is
// accepted only as fourteen ASCII digits: DOC-ICP-04 specifies OCTET STRING,
// but authorities have been seen writing PrintableString and UTF8String, so
// any primitive string type is read. Nothing may follow the value.
func parseOtherName(body []byte) (typeID asn1.ObjectIdentifier, value string, ok bool) {
	rest, err := asn1.Unmarshal(body, &typeID)
	if err != nil {
		return nil, "", false
	}

	var wrapper asn1.RawValue
	rest, err = asn1.Unmarshal(rest, &wrapper)
	if err != nil || len(rest) > 0 ||
		wrapper.Class != asn1.ClassContextSpecific || wrapper.Tag != 0 || !wrapper.IsCompound {
		return typeID, "", false
	}

	var raw asn1.RawValue
	rest, err = asn1.Unmarshal(wrapper.Bytes, &raw)
	if err != nil || len(rest) > 0 || raw.Class != asn1.ClassUniversal || raw.IsCompound {
		return typeID, "", false
	}

	switch raw.Tag {
	case asn1.TagOctetString, asn1.TagPrintableString, asn1.TagUTF8String, asn1.TagIA5String:
	default:
		return typeID, "", false
	}
	// Checked before the digits, so that an oversized value costs nothing.
	if len(raw.Bytes) != cnpjcpf.CNPJLength {
		return typeID, "", false
	}
	for _, b := range raw.Bytes {
		if b < '0' || b > '9' {
			return typeID, "", false
		}
	}
	return typeID, string(raw.Bytes), true
}
