package xmlsigner

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"github.com/beevik/etree"
)

// Errors returned by VerifyNFSeSignature.
var (
	// ErrNFSeUnsigned indicates an NFS-e without the government's signature.
	ErrNFSeUnsigned = errors.New("NFS-e has no signature of its own")

	// ErrNFSeTampered indicates a signature that does not match the content:
	// the XML was changed after the government signed it, or the signature
	// was never the government's.
	ErrNFSeTampered = errors.New("NFS-e signature does not match its content")
)

// sha256Suffix ends both the digest and the signature algorithm URIs the
// national system uses.
const sha256Suffix = "sha256"

// VerifyNFSeSignature checks the signature the government puts on an
// authorised NFS-e: the Signature that is a direct child of the NFSe root and
// references infNFSe.
//
// It proves integrity, not origin. The certificate comes from inside the very
// XML being checked, and nothing here chains it to a trusted authority, so a
// document forged whole and signed with any certificate passes. What it does
// catch is an authorised invoice edited afterwards — a value, a name — which
// is the realistic way a DANFSe ends up saying something the government never
// issued. The QR Code, which leads to the public lookup, remains the proof of
// authenticity.
//
// The enveloped-signature transform removes only the signature being
// verified. An NFS-e carries a second one, the issuer's, inside infNFSe/DPS,
// and that one is part of what the government signed: CanonicalizeSigned,
// which strips every Signature below the element, computes a different digest
// for a genuine NFS-e.
//
// The government canonicalises with exc-c14n#WithComments. Canonicalize drops
// comments, so the two agree on any NFS-e the government produced — it writes
// none — and an XML with comments added inside infNFSe fails, which is right:
// it is no longer the document that was signed.
func VerifyNFSeSignature(nfseXML []byte) error {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(nfseXML); err != nil {
		return fmt.Errorf("failed to parse XML document: %w", err)
	}

	root := doc.Root()
	if root == nil {
		return fmt.Errorf("%w: empty document", ErrNFSeUnsigned)
	}
	signature := root.SelectElement("Signature")
	if signature == nil {
		return ErrNFSeUnsigned
	}

	signedInfo := signature.SelectElement("SignedInfo")
	if signedInfo == nil {
		return fmt.Errorf("%w: %v", ErrNFSeTampered, ErrVerificationNoSignedInfo)
	}
	reference := signedInfo.SelectElement("Reference")
	if reference == nil {
		return fmt.Errorf("%w: %v", ErrNFSeTampered, ErrVerificationNoReference)
	}

	if err := requireSHA256(signedInfo.SelectElement("SignatureMethod")); err != nil {
		return err
	}
	if err := requireSHA256(reference.SelectElement("DigestMethod")); err != nil {
		return err
	}

	id := strings.TrimPrefix(reference.SelectAttrValue("URI", ""), "#")
	signed := elementByID(root, id)
	if signed == nil {
		return fmt.Errorf("%w: %v: %q", ErrNFSeTampered, ErrVerificationReferencedElementNotFound, id)
	}

	// The enveloped-signature transform, applied to this signature alone.
	if contains(signed, signature) {
		signature.Parent().RemoveChild(signature)
	}

	canonical, err := Canonicalize(signed)
	if err != nil {
		return fmt.Errorf("failed to canonicalize %s: %w", signed.Tag, err)
	}
	digest := sha256.Sum256(canonical)

	expected := cleanBase64(reference.SelectElement("DigestValue").Text())
	if base64.StdEncoding.EncodeToString(digest[:]) != expected {
		return fmt.Errorf("%w: %v", ErrNFSeTampered, ErrVerificationDigestMismatch)
	}

	verifier := &XMLVerifier{}
	cert, err := verifier.extractCertificate(signature)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNFSeTampered, err)
	}

	value := signature.SelectElement("SignatureValue")
	if value == nil {
		return fmt.Errorf("%w: %v", ErrNFSeTampered, ErrVerificationNoSignatureValue)
	}
	if err := verifier.verifySignatureValue(signedInfo, cleanBase64(value.Text()), cert); err != nil {
		return fmt.Errorf("%w: %v", ErrNFSeTampered, err)
	}
	return nil
}

// requireSHA256 refuses an algorithm this package cannot check, so that it is
// reported as such rather than as a document that does not match.
func requireSHA256(method *etree.Element) error {
	if method == nil {
		return fmt.Errorf("%w: algorithm not declared", ErrVerificationUnsupportedAlgorithm)
	}
	algorithm := method.SelectAttrValue("Algorithm", "")
	if !strings.HasSuffix(algorithm, sha256Suffix) {
		return fmt.Errorf("%w: %s", ErrVerificationUnsupportedAlgorithm, algorithm)
	}
	return nil
}

// elementByID walks the tree instead of building an XPath from the Id: the
// value comes from the document being checked, and a quote in it would make
// the path invalid, which etree answers with a panic.
func elementByID(element *etree.Element, id string) *etree.Element {
	if id == "" {
		return nil
	}
	if element.SelectAttrValue("Id", "") == id {
		return element
	}
	for _, child := range element.ChildElements() {
		if found := elementByID(child, id); found != nil {
			return found
		}
	}
	return nil
}

// contains reports whether descendant sits somewhere below ancestor.
func contains(ancestor, descendant *etree.Element) bool {
	for parent := descendant.Parent(); parent != nil; parent = parent.Parent() {
		if parent == ancestor {
			return true
		}
	}
	return false
}
