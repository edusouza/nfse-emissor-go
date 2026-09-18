package xmlsigner

import (
	"strings"
	"testing"

	"github.com/beevik/etree"
)

func TestCanonicalize_SimpleElement(t *testing.T) {
	xml := `<root><child>text</child></root>`

	doc := etree.NewDocument()
	if err := doc.ReadFromString(xml); err != nil {
		t.Fatalf("Failed to parse XML: %v", err)
	}

	canonical, err := Canonicalize(doc.Root())
	if err != nil {
		t.Fatalf("Failed to canonicalize: %v", err)
	}

	// Canonical form should not have self-closing tags
	result := string(canonical)
	if strings.Contains(result, "/>") {
		t.Error("Canonical form should not contain self-closing tags")
	}

	// Should contain the text content
	if !strings.Contains(result, "text") {
		t.Error("Canonical form should contain the text content")
	}
}

func TestCanonicalize_AttributeOrder(t *testing.T) {
	// Attributes should be sorted alphabetically
	xml := `<root z="3" a="1" m="2"></root>`

	doc := etree.NewDocument()
	if err := doc.ReadFromString(xml); err != nil {
		t.Fatalf("Failed to parse XML: %v", err)
	}

	canonical, err := Canonicalize(doc.Root())
	if err != nil {
		t.Fatalf("Failed to canonicalize: %v", err)
	}

	result := string(canonical)

	// Find positions of attributes
	posA := strings.Index(result, `a="1"`)
	posM := strings.Index(result, `m="2"`)
	posZ := strings.Index(result, `z="3"`)

	if posA == -1 || posM == -1 || posZ == -1 {
		t.Fatalf("Missing attributes in canonical form: %s", result)
	}

	if posA > posM || posM > posZ {
		t.Errorf("Attributes not sorted alphabetically: %s", result)
	}
}

func TestCanonicalize_NamespaceHandling(t *testing.T) {
	// Test with prefixed namespace which is more commonly used in NFS-e
	xml := `<ns:root xmlns:ns="http://example.com/ns"><ns:child>text</ns:child></ns:root>`

	doc := etree.NewDocument()
	if err := doc.ReadFromString(xml); err != nil {
		t.Fatalf("Failed to parse XML: %v", err)
	}

	canonical, err := Canonicalize(doc.Root())
	if err != nil {
		t.Fatalf("Failed to canonicalize: %v", err)
	}

	result := string(canonical)

	// Verify it produces valid output
	if result == "" {
		t.Error("Canonical form should not be empty")
	}

	// Should contain the child element
	if !strings.Contains(result, "child") {
		t.Error("Canonical form should contain child element")
	}
}

func TestCanonicalize_EmptyElement(t *testing.T) {
	xml := `<root><empty/></root>`

	doc := etree.NewDocument()
	if err := doc.ReadFromString(xml); err != nil {
		t.Fatalf("Failed to parse XML: %v", err)
	}

	canonical, err := Canonicalize(doc.Root())
	if err != nil {
		t.Fatalf("Failed to canonicalize: %v", err)
	}

	result := string(canonical)

	// Empty elements should be rendered as start-end tag pairs
	if !strings.Contains(result, "<empty></empty>") {
		t.Errorf("Empty element should be rendered as start-end pair: %s", result)
	}
}

func TestCanonicalize_AttributeEscaping(t *testing.T) {
	doc := etree.NewDocument()
	root := doc.CreateElement("root")
	root.CreateAttr("attr", `value with "quotes" and <brackets>`)

	canonical, err := Canonicalize(root)
	if err != nil {
		t.Fatalf("Failed to canonicalize: %v", err)
	}

	result := string(canonical)

	// Should escape special characters in attributes
	if strings.Contains(result, `"quotes"`) {
		t.Error("Quotes should be escaped in attribute values")
	}
	if strings.Contains(result, `<brackets>`) {
		t.Error("Brackets should be escaped in attribute values")
	}
}

func TestCanonicalize_TextEscaping(t *testing.T) {
	doc := etree.NewDocument()
	root := doc.CreateElement("root")
	root.SetText("text with <brackets> and & ampersand")

	canonical, err := Canonicalize(root)
	if err != nil {
		t.Fatalf("Failed to canonicalize: %v", err)
	}

	result := string(canonical)

	// Should escape < and &
	if strings.Contains(result, "<brackets>") {
		t.Error("< should be escaped in text content")
	}
	if strings.Contains(result, "& ") {
		t.Error("& should be escaped in text content")
	}
	if !strings.Contains(result, "&lt;") {
		t.Error("< should be escaped as &lt;")
	}
	if !strings.Contains(result, "&amp;") {
		t.Error("& should be escaped as &amp;")
	}
}

func TestCanonicalize_NilElement(t *testing.T) {
	canonical, err := Canonicalize(nil)
	if err != nil {
		t.Fatalf("Unexpected error for nil element: %v", err)
	}

	if canonical != nil {
		t.Error("Canonical form of nil should be nil")
	}
}

func TestCanonicalize_NestedElements(t *testing.T) {
	xml := `<root><level1><level2><level3>deep</level3></level2></level1></root>`

	doc := etree.NewDocument()
	if err := doc.ReadFromString(xml); err != nil {
		t.Fatalf("Failed to parse XML: %v", err)
	}

	canonical, err := Canonicalize(doc.Root())
	if err != nil {
		t.Fatalf("Failed to canonicalize: %v", err)
	}

	result := string(canonical)

	// Should preserve nesting structure
	if !strings.Contains(result, "<level3>deep</level3>") {
		t.Error("Nested structure should be preserved")
	}
}

func TestCanonicalizeSigned_RemovesSignature(t *testing.T) {
	xml := `<DPS xmlns="http://www.sped.fazenda.gov.br/nfse">
		<infDPS Id="test">content</infDPS>
		<Signature xmlns="http://www.w3.org/2000/09/xmldsig#">
			<SignedInfo>info</SignedInfo>
		</Signature>
	</DPS>`

	doc := etree.NewDocument()
	if err := doc.ReadFromString(xml); err != nil {
		t.Fatalf("Failed to parse XML: %v", err)
	}

	canonical, err := CanonicalizeSigned(doc.Root())
	if err != nil {
		t.Fatalf("Failed to canonicalize: %v", err)
	}

	result := string(canonical)

	// Should not contain Signature element
	if strings.Contains(result, "<Signature") {
		t.Error("Canonical signed form should not contain Signature element")
	}

	// Should still contain infDPS
	if !strings.Contains(result, "infDPS") {
		t.Error("Canonical signed form should still contain infDPS")
	}
}

func TestCanonicalize_DPSDocument(t *testing.T) {
	// Test with a realistic DPS document
	xml := `<DPS xmlns="http://www.sped.fazenda.gov.br/nfse" versao="1.00">
  <infDPS Id="DPS123456789">
    <tpAmb>2</tpAmb>
    <dhEmi>2024-01-15T10:30:00-03:00</dhEmi>
  </infDPS>
</DPS>`

	doc := etree.NewDocument()
	if err := doc.ReadFromString(xml); err != nil {
		t.Fatalf("Failed to parse XML: %v", err)
	}

	infDPS := doc.Root().FindElement("infDPS")
	if infDPS == nil {
		t.Fatal("Failed to find infDPS element")
	}

	canonical, err := Canonicalize(infDPS)
	if err != nil {
		t.Fatalf("Failed to canonicalize infDPS: %v", err)
	}

	result := string(canonical)

	// Verify key content is present
	if !strings.Contains(result, `Id="DPS123456789"`) {
		t.Error("Canonical form should contain Id attribute")
	}
	if !strings.Contains(result, "<tpAmb>2</tpAmb>") {
		t.Error("Canonical form should contain tpAmb element")
	}
}

func BenchmarkCanonicalize(b *testing.B) {
	xml := `<DPS xmlns="http://www.sped.fazenda.gov.br/nfse" versao="1.00">
  <infDPS Id="DPS123456789">
    <tpAmb>2</tpAmb>
    <dhEmi>2024-01-15T10:30:00-03:00</dhEmi>
    <verAplic>1.0.0</verAplic>
    <serie>00001</serie>
    <nDPS>1</nDPS>
  </infDPS>
</DPS>`

	doc := etree.NewDocument()
	if err := doc.ReadFromString(xml); err != nil {
		b.Fatalf("Failed to parse XML: %v", err)
	}

	root := doc.Root()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Canonicalize(root)
		if err != nil {
			b.Fatalf("Failed to canonicalize: %v", err)
		}
	}
}
