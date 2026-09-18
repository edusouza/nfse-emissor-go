// Package xmlsigner provides XMLDSig digital signature functionality for NFS-e documents.
package xmlsigner

import (
	"bytes"
	"sort"
	"strings"

	"github.com/beevik/etree"
)

// Canonicalization algorithm identifiers as defined by W3C standards.
const (
	// AlgorithmExcC14N is the Exclusive XML Canonicalization algorithm.
	// This is the required algorithm for Brazilian NFS-e signatures.
	AlgorithmExcC14N = "http://www.w3.org/2001/10/xml-exc-c14n#"

	// AlgorithmC14N is the standard XML Canonicalization 1.0 algorithm.
	AlgorithmC14N = "http://www.w3.org/TR/2001/REC-xml-c14n-20010315"
)

// namespaceInfo holds namespace information for canonicalization.
type namespaceInfo struct {
	prefix string
	uri    string
}

// Canonicalize transforms an XML element to its canonical form using
// Exclusive XML Canonicalization (exc-c14n) as defined by W3C.
//
// The canonical form is a normalized representation that ensures two
// XML documents with the same logical content produce identical byte sequences,
// which is essential for digital signature verification.
//
// Exclusive C14N rules:
//   - UTF-8 encoding
//   - Line breaks normalized to LF
//   - Attribute values normalized
//   - Attributes sorted alphabetically
//   - Empty elements rendered as start-end tag pairs (not self-closing)
//   - Namespace declarations rendered only when visibly utilized
//   - Superfluous namespace declarations removed
//
// Parameters:
//   - element: The etree.Element to canonicalize
//
// Returns:
//   - []byte: The canonical form of the element as UTF-8 bytes
//   - error: Any error encountered during canonicalization
func Canonicalize(element *etree.Element) ([]byte, error) {
	if element == nil {
		return nil, nil
	}

	// Create a buffer for the output
	buf := &bytes.Buffer{}

	// Collect all namespaces that are in scope for this element
	nsInScope := collectNamespacesInScope(element)

	// Perform exclusive canonicalization
	canonicalizeElement(buf, element, nsInScope, nil)

	return buf.Bytes(), nil
}

// CanonicalizeToBytesWithNamespaces canonicalizes an element with additional
// namespace prefixes to include (InclusiveNamespaces).
//
// The inclusiveNSPrefixes parameter specifies namespace prefixes that should
// be treated as "visibly utilized" even if they are not directly used in the
// canonicalized subtree. This is sometimes needed for XMLDSig.
//
// Parameters:
//   - element: The etree.Element to canonicalize
//   - inclusiveNSPrefixes: Namespace prefixes to force include
//
// Returns:
//   - []byte: The canonical form of the element as UTF-8 bytes
//   - error: Any error encountered during canonicalization
func CanonicalizeToBytesWithNamespaces(element *etree.Element, inclusiveNSPrefixes []string) ([]byte, error) {
	if element == nil {
		return nil, nil
	}

	buf := &bytes.Buffer{}
	nsInScope := collectNamespacesInScope(element)
	canonicalizeElement(buf, element, nsInScope, inclusiveNSPrefixes)

	return buf.Bytes(), nil
}

// collectNamespacesInScope collects all namespace declarations in scope for an element,
// including inherited namespaces from ancestor elements.
func collectNamespacesInScope(element *etree.Element) map[string]string {
	nsInScope := make(map[string]string)

	// Walk up the tree to collect inherited namespaces
	var ancestors []*etree.Element
	for e := element; e != nil; e = e.Parent() {
		ancestors = append([]*etree.Element{e}, ancestors...)
	}

	// Process from root to leaf to get correct namespace inheritance
	for _, e := range ancestors {
		// Check the element's own namespace
		if e.Space != "" {
			// Find the prefix for this namespace
			prefix := findNamespacePrefix(e, e.Space)
			nsInScope[prefix] = e.Space
		}

		// Check all namespace attributes
		for _, attr := range e.Attr {
			if attr.Space == "xmlns" || (attr.Space == "" && attr.Key == "xmlns") {
				if attr.Key == "xmlns" {
					nsInScope[""] = attr.Value
				} else {
					nsInScope[attr.Key] = attr.Value
				}
			}
		}
	}

	return nsInScope
}

// findNamespacePrefix finds the prefix for a namespace URI in an element.
func findNamespacePrefix(element *etree.Element, uri string) string {
	for e := element; e != nil; e = e.Parent() {
		for _, attr := range e.Attr {
			if attr.Space == "xmlns" && attr.Value == uri {
				return attr.Key
			}
			if attr.Space == "" && attr.Key == "xmlns" && attr.Value == uri {
				return ""
			}
		}
	}
	return ""
}

// canonicalizeElement recursively canonicalizes an element and its children.
func canonicalizeElement(buf *bytes.Buffer, element *etree.Element, parentNS map[string]string, inclusiveNSPrefixes []string) {
	// Determine the element name (with prefix if applicable)
	elementName := element.Tag
	if element.Space != "" {
		// Find the prefix for this namespace
		prefix := ""
		for p, uri := range parentNS {
			if uri == element.Space {
				prefix = p
				break
			}
		}
		if prefix != "" {
			elementName = prefix + ":" + element.Tag
		}
	}

	// Write opening tag
	buf.WriteString("<")
	buf.WriteString(elementName)

	// Collect namespace declarations needed for this element
	nsDecls := collectRequiredNamespaces(element, parentNS, inclusiveNSPrefixes)

	// Collect regular attributes
	attrs := collectAttributes(element)

	// Sort namespace declarations (they go before regular attributes in exc-c14n)
	sortedNSDecls := make([]namespaceInfo, 0, len(nsDecls))
	for prefix, uri := range nsDecls {
		sortedNSDecls = append(sortedNSDecls, namespaceInfo{prefix, uri})
	}
	sort.Slice(sortedNSDecls, func(i, j int) bool {
		// Default namespace (empty prefix) comes first, then alphabetically
		if sortedNSDecls[i].prefix == "" {
			return true
		}
		if sortedNSDecls[j].prefix == "" {
			return false
		}
		return sortedNSDecls[i].prefix < sortedNSDecls[j].prefix
	})

	// Write namespace declarations
	for _, ns := range sortedNSDecls {
		if ns.prefix == "" {
			buf.WriteString(" xmlns=\"")
		} else {
			buf.WriteString(" xmlns:")
			buf.WriteString(ns.prefix)
			buf.WriteString("=\"")
		}
		buf.WriteString(escapeAttrValue(ns.uri))
		buf.WriteString("\"")
	}

	// Sort regular attributes alphabetically by qualified name
	sort.Slice(attrs, func(i, j int) bool {
		return getQualifiedAttrName(attrs[i], parentNS) < getQualifiedAttrName(attrs[j], parentNS)
	})

	// Write regular attributes
	for _, attr := range attrs {
		buf.WriteString(" ")
		buf.WriteString(getQualifiedAttrName(attr, parentNS))
		buf.WriteString("=\"")
		buf.WriteString(escapeAttrValue(attr.Value))
		buf.WriteString("\"")
	}

	buf.WriteString(">")

	// Update namespace scope for children
	childNS := make(map[string]string)
	for k, v := range parentNS {
		childNS[k] = v
	}
	for prefix, uri := range nsDecls {
		childNS[prefix] = uri
	}

	// Process children (text nodes and elements)
	for _, child := range element.Child {
		switch c := child.(type) {
		case *etree.Element:
			canonicalizeElement(buf, c, childNS, inclusiveNSPrefixes)
		case *etree.CharData:
			// Write text content, escaped
			buf.WriteString(escapeTextContent(c.Data))
		}
	}

	// Write closing tag (never use self-closing in canonical form)
	buf.WriteString("</")
	buf.WriteString(elementName)
	buf.WriteString(">")
}

// collectRequiredNamespaces determines which namespace declarations are needed
// for this element according to exclusive canonicalization rules.
func collectRequiredNamespaces(element *etree.Element, parentNS map[string]string, inclusiveNSPrefixes []string) map[string]string {
	nsDecls := make(map[string]string)

	// Check if we need to declare the element's namespace
	if element.Space != "" {
		// Find the prefix for this namespace
		prefix := findPrefixForNamespace(element, element.Space)
		if existingURI, ok := parentNS[prefix]; !ok || existingURI != element.Space {
			nsDecls[prefix] = element.Space
		}
	}

	// Check if default namespace needs to be declared (when element has no prefix but uses default ns)
	if element.Space == "" {
		// Check for default namespace in element's own attributes
		for _, attr := range element.Attr {
			if attr.Space == "" && attr.Key == "xmlns" {
				if existingURI, ok := parentNS[""]; !ok || existingURI != attr.Value {
					nsDecls[""] = attr.Value
				}
				break
			}
		}
	}

	// Check attributes for namespace usage
	for _, attr := range element.Attr {
		// Skip namespace declarations themselves
		if attr.Space == "xmlns" || (attr.Space == "" && attr.Key == "xmlns") {
			continue
		}

		if attr.Space != "" {
			prefix := findPrefixForNamespace(element, attr.Space)
			if existingURI, ok := parentNS[prefix]; !ok || existingURI != attr.Space {
				nsDecls[prefix] = attr.Space
			}
		}
	}

	// Add inclusive namespace prefixes if specified
	for _, prefix := range inclusiveNSPrefixes {
		if uri, ok := parentNS[prefix]; ok {
			if _, alreadyDeclared := nsDecls[prefix]; !alreadyDeclared {
				nsDecls[prefix] = uri
			}
		}
	}

	return nsDecls
}

// findPrefixForNamespace finds the prefix for a given namespace URI.
func findPrefixForNamespace(element *etree.Element, uri string) string {
	for e := element; e != nil; e = e.Parent() {
		for _, attr := range e.Attr {
			if attr.Space == "xmlns" && attr.Value == uri {
				return attr.Key
			}
		}
	}
	return ""
}

// collectAttributes collects non-namespace attributes from an element.
func collectAttributes(element *etree.Element) []etree.Attr {
	var attrs []etree.Attr
	for _, attr := range element.Attr {
		// Skip namespace declarations
		if attr.Space == "xmlns" || (attr.Space == "" && attr.Key == "xmlns") {
			continue
		}
		// Skip xml namespace attributes (xml:space, xml:lang, etc.) unless visibly utilized
		if attr.Space == "xml" {
			continue
		}
		attrs = append(attrs, attr)
	}
	return attrs
}

// getQualifiedAttrName returns the qualified name for an attribute.
func getQualifiedAttrName(attr etree.Attr, nsMap map[string]string) string {
	if attr.Space == "" {
		return attr.Key
	}
	// Find prefix for the namespace
	for prefix, uri := range nsMap {
		if uri == attr.Space && prefix != "" {
			return prefix + ":" + attr.Key
		}
	}
	return attr.Key
}

// escapeAttrValue escapes special characters in attribute values.
func escapeAttrValue(s string) string {
	var buf strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			buf.WriteString("&amp;")
		case '<':
			buf.WriteString("&lt;")
		case '"':
			buf.WriteString("&quot;")
		case '\t':
			buf.WriteString("&#x9;")
		case '\n':
			buf.WriteString("&#xA;")
		case '\r':
			buf.WriteString("&#xD;")
		default:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

// escapeTextContent escapes special characters in text content.
func escapeTextContent(s string) string {
	var buf strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			buf.WriteString("&amp;")
		case '<':
			buf.WriteString("&lt;")
		case '>':
			buf.WriteString("&gt;")
		case '\r':
			buf.WriteString("&#xD;")
		default:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

// CanonicalizeSigned creates a canonical form of an element suitable for
// creating or verifying an XMLDSig signature.
// This removes any existing Signature element before canonicalizing.
//
// Parameters:
//   - element: The etree.Element to canonicalize
//
// Returns:
//   - []byte: The canonical form of the element as UTF-8 bytes
//   - error: Any error encountered during canonicalization
func CanonicalizeSigned(element *etree.Element) ([]byte, error) {
	if element == nil {
		return nil, nil
	}

	// Create a deep copy to avoid modifying the original
	doc := etree.NewDocument()
	doc.SetRoot(element.Copy())
	root := doc.Root()

	// Remove any existing Signature element
	removeSignatureElements(root)

	return Canonicalize(root)
}

// removeSignatureElements removes all Signature elements from an element tree.
func removeSignatureElements(element *etree.Element) {
	// Find and remove Signature children
	var toRemove []*etree.Element
	for _, child := range element.ChildElements() {
		if child.Tag == "Signature" &&
			(child.Space == "http://www.w3.org/2000/09/xmldsig#" || child.Space == "") {
			toRemove = append(toRemove, child)
		} else {
			// Recursively process children
			removeSignatureElements(child)
		}
	}

	// Remove the signature elements
	for _, sig := range toRemove {
		element.RemoveChild(sig)
	}
}
