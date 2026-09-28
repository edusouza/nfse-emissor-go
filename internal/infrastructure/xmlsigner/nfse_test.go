package xmlsigner

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/beevik/etree"
)

// nfseSemAssinatura has the shape that matters: infNFSe carries the DPS, and
// the DPS carries the issuer's own signature. A real one would be valid; here
// it only has to be there, because the government signs over it.
const nfseSemAssinatura = `<NFSe xmlns="http://www.sped.fazenda.gov.br/nfse" versao="1.01">` +
	`<infNFSe Id="NFS41069022212345678000195000000000000126081234567890">` +
	`<nNFSe>126</nNFSe><valores><vLiq>1500.00</vLiq></valores>` +
	`<DPS versao="1.00"><infDPS Id="DPS4106902212345678000195000010000000000126"><nDPS>126</nDPS></infDPS>` +
	`<Signature xmlns="http://www.w3.org/2000/09/xmldsig#"><SignedInfo/><SignatureValue>ZW1pdGVudGU=</SignatureValue></Signature>` +
	`</DPS></infNFSe></NFSe>`

// assinarComoOGoverno signs infNFSe the way the government does: the digest
// covers infNFSe whole, the issuer's signature included.
func assinarComoOGoverno(t *testing.T, xml string) string {
	t.Helper()

	doc := etree.NewDocument()
	if err := doc.ReadFromString(xml); err != nil {
		t.Fatal(err)
	}
	root := doc.Root()
	inf := root.SelectElement("infNFSe")

	canonical, err := Canonicalize(inf)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(canonical)

	signer := NewXMLSigner(generateTestCertificate(t))
	signedInfo := signer.buildSignedInfo("#"+inf.SelectAttrValue("Id", ""), base64.StdEncoding.EncodeToString(digest[:]))
	canonicalSignedInfo, err := Canonicalize(signedInfo)
	if err != nil {
		t.Fatal(err)
	}
	value, err := signer.signData(canonicalSignedInfo)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := signer.certInfo.GetCertificateBase64()
	if err != nil {
		t.Fatal(err)
	}
	root.AddChild(signer.buildSignatureElement(signedInfo, base64.StdEncoding.EncodeToString(value), cert))

	out, err := doc.WriteToString()
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestVerifyNFSeSignature_Valida(t *testing.T) {
	assinada := assinarComoOGoverno(t, nfseSemAssinatura)

	if err := VerifyNFSeSignature([]byte(assinada)); err != nil {
		t.Fatalf("uma NFS-e integra deveria passar: %v", err)
	}
}

// The value is the realistic thing to change after the fact.
func TestVerifyNFSeSignature_ConteudoAlterado(t *testing.T) {
	assinada := assinarComoOGoverno(t, nfseSemAssinatura)
	alterada := strings.Replace(assinada, "<vLiq>1500.00</vLiq>", "<vLiq>15000.00</vLiq>", 1)

	err := VerifyNFSeSignature([]byte(alterada))
	if !errors.Is(err, ErrNFSeTampered) {
		t.Fatalf("esperava ErrNFSeTampered, veio %v", err)
	}
}

func TestVerifyNFSeSignature_SemAssinatura(t *testing.T) {
	err := VerifyNFSeSignature([]byte(nfseSemAssinatura))
	if !errors.Is(err, ErrNFSeUnsigned) {
		t.Fatalf("esperava ErrNFSeUnsigned, veio %v", err)
	}
}

// The issuer's signature inside the DPS is part of what the government signed.
// Stripping every Signature below infNFSe — what CanonicalizeSigned does for
// the DPS — computes a digest no genuine NFS-e has.
func TestVerifyNFSeSignature_NaoRemoveAAssinaturaDaDPS(t *testing.T) {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(nfseSemAssinatura); err != nil {
		t.Fatal(err)
	}
	inf := doc.Root().SelectElement("infNFSe")

	inteiro, _ := Canonicalize(inf)
	semAssinaturas, _ := CanonicalizeSigned(inf)
	if string(inteiro) == string(semAssinaturas) {
		t.Fatal("o exemplo precisa de uma assinatura aninhada para o teste significar algo")
	}

	if err := VerifyNFSeSignature([]byte(assinarComoOGoverno(t, nfseSemAssinatura))); err != nil {
		t.Errorf("a assinatura da DPS nao pode sair do calculo: %v", err)
	}
}

// An Id with a quote would break an XPath built from it, and etree panics on
// an invalid path. The lookup walks the tree instead.
func TestVerifyNFSeSignature_IdComAspas(t *testing.T) {
	xml := `<NFSe><infNFSe Id="a'b"/><Signature><SignedInfo>` +
		`<SignatureMethod Algorithm="http://www.w3.org/2001/04/xmldsig-more#rsa-sha256"/>` +
		`<Reference URI="#x']"><DigestMethod Algorithm="http://www.w3.org/2001/04/xmlenc#sha256"/>` +
		`<DigestValue>AA==</DigestValue></Reference></SignedInfo></Signature></NFSe>`

	err := VerifyNFSeSignature([]byte(xml))
	if !errors.Is(err, ErrNFSeTampered) {
		t.Fatalf("esperava ErrNFSeTampered, veio %v", err)
	}
}

func TestVerifyNFSeSignature_AlgoritmoNaoSuportado(t *testing.T) {
	xml := `<NFSe><infNFSe Id="x"/><Signature><SignedInfo>` +
		`<SignatureMethod Algorithm="http://www.w3.org/2000/09/xmldsig#rsa-sha1"/>` +
		`<Reference URI="#x"><DigestMethod Algorithm="http://www.w3.org/2000/09/xmldsig#sha1"/>` +
		`<DigestValue>AA==</DigestValue></Reference></SignedInfo></Signature></NFSe>`

	err := VerifyNFSeSignature([]byte(xml))
	if !errors.Is(err, ErrVerificationUnsupportedAlgorithm) {
		t.Fatalf("esperava ErrVerificationUnsupportedAlgorithm, veio %v", err)
	}
}
