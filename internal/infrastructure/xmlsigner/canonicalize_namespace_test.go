package xmlsigner

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/beevik/etree"
)

// The canonical forms below were produced by lxml (libxml2), not by this
// package. That is the whole point: every signature defect this project has
// shipped survived a green suite because the tests asserted what the code
// already did. A golden value from another implementation is the only kind
// that can disagree.
//
// Reproduce with:
//
//	python3 -c 'import lxml.etree as ET; d=ET.fromstring(b"<DPS xmlns=\"http://www.sped.fazenda.gov.br/nfse\"><infDPS Id=\"DPS1\"><tpAmb>2</tpAmb></infDPS></DPS>"); print(ET.tostring(d[0], method="c14n", exclusive=True).decode())'
const (
	nsDocument = `<DPS xmlns="http://www.sped.fazenda.gov.br/nfse">` +
		`<infDPS Id="DPS1"><tpAmb>2</tpAmb></infDPS></DPS>`

	// The apex carries the namespace it inherits, because a subtree
	// canonicalized for XMLDSig is a document of its own.
	wantCanonical = `<infDPS xmlns="http://www.sped.fazenda.gov.br/nfse" Id="DPS1">` +
		`<tpAmb>2</tpAmb></infDPS>`

	wantDigest = "dQDGZHV4HoXJDsrf9fRrZYPuujdkurOQRaqX85Ktuy8="
)

func infDPSOf(t *testing.T, xml string) *etree.Element {
	t.Helper()

	doc := etree.NewDocument()
	if err := doc.ReadFromString(xml); err != nil {
		t.Fatal(err)
	}
	el := doc.FindElement("DPS/infDPS")
	if el == nil {
		t.Fatal("infDPS nao encontrado")
	}
	return el
}

// CanonicalizeSigned is what the reference digest is computed over, so it is
// the function whose output the government re-derives. It used to copy the
// element into a fresh document first, which detached it from the ancestors
// carrying the default namespace — and the declaration vanished from the
// digest without a trace.
func TestCanonicalizeSigned_KeepsInheritedNamespace(t *testing.T) {
	got, err := CanonicalizeSigned(infDPSOf(t, nsDocument))
	if err != nil {
		t.Fatal(err)
	}

	if string(got) != wantCanonical {
		t.Errorf("forma canonica divergente\n  obtida:   %s\n  esperada: %s", got, wantCanonical)
	}

	sum := sha256.Sum256(got)
	if digest := base64.StdEncoding.EncodeToString(sum[:]); digest != wantDigest {
		t.Errorf("digest = %s, esperava %s", digest, wantDigest)
	}
}

func TestCanonicalize_KeepsInheritedNamespace(t *testing.T) {
	got, err := Canonicalize(infDPSOf(t, nsDocument))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != wantCanonical {
		t.Errorf("forma canonica divergente\n  obtida:   %s\n  esperada: %s", got, wantCanonical)
	}
}

// The two entry points differ only in that one drops an enveloped signature.
// With no signature to drop they must agree byte for byte — and when they did
// not, signing and verifying agreed with each other while disagreeing with
// every other implementation. This assertion alone would have caught it.
func TestCanonicalizeSigned_AgreesWithCanonicalize(t *testing.T) {
	for name, xml := range map[string]string{
		"namespace padrao herdado": nsDocument,
		"namespace com prefixo": `<n:DPS xmlns:n="http://www.sped.fazenda.gov.br/nfse">` +
			`<n:infDPS Id="DPS1"><n:tpAmb>2</n:tpAmb></n:infDPS></n:DPS>`,
		"sem namespace": `<DPS><infDPS Id="DPS1"><tpAmb>2</tpAmb></infDPS></DPS>`,
	} {
		t.Run(name, func(t *testing.T) {
			doc := etree.NewDocument()
			if err := doc.ReadFromString(xml); err != nil {
				t.Fatal(err)
			}
			el := doc.FindElement("//infDPS")
			if el == nil {
				t.Fatal("infDPS nao encontrado")
			}

			plain, err := Canonicalize(el)
			if err != nil {
				t.Fatal(err)
			}
			signed, err := CanonicalizeSigned(el)
			if err != nil {
				t.Fatal(err)
			}
			if string(plain) != string(signed) {
				t.Errorf("as duas formas divergem\n  Canonicalize:       %s\n  CanonicalizeSigned: %s",
					plain, signed)
			}
		})
	}
}

// CanonicalizeSigned must not mutate the caller's document: the emitter goes on
// to serialize the very tree it just digested.
func TestCanonicalizeSigned_LeavesDocumentUntouched(t *testing.T) {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(nsDocument); err != nil {
		t.Fatal(err)
	}
	before, err := doc.WriteToString()
	if err != nil {
		t.Fatal(err)
	}

	if _, err := CanonicalizeSigned(doc.FindElement("DPS/infDPS")); err != nil {
		t.Fatal(err)
	}

	after, err := doc.WriteToString()
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Errorf("o documento foi alterado\n  antes:  %s\n  depois: %s", before, after)
	}
}
