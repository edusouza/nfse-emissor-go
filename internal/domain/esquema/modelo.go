// Package esquema validates documents against the official XSDs of the
// Sistema Nacional NFS-e, read from the schema files themselves.
//
// The emitter used to check the DPS with rules written by hand, which drifted
// from the schema versioned next to them in ways the tests could not see:
// elements looked up at the wrong path, a required group treated as optional,
// fixtures written with the same mistakes (issue #4, ADR 0003). Reading the
// XSD makes that class of divergence impossible: whatever the schema says is
// what is checked.
//
// It is not a general XSD processor. It implements the subset the NFS-e
// schemas and the XML-DSig schema they import actually use — sequence,
// choice, any, occurrence bounds, attributes, simple and complex types,
// simpleContent extension, and the enumeration, pattern, length and
// whiteSpace facets — and refuses to load a schema that uses anything else,
// so that a new construct in a future version fails loudly instead of being
// silently ignored. No cgo, no dependency beyond etree (ADR 0014).
package esquema

import "regexp"

const (
	nsXSD = "http://www.w3.org/2001/XMLSchema"
	nsXML = "http://www.w3.org/XML/1998/namespace"
)

// ilimitado is maxOccurs="unbounded".
const ilimitado = -1

type nome struct {
	ns, local string
}

func (n nome) String() string { return n.local }

// tipoSimples is a simple type with the facets declared at its own level;
// the base carries the rest. XSD combines them that way: patterns declared
// together are alternatives, patterns at different derivation levels must all
// hold, and every other facet only narrows.
type tipoSimples struct {
	nome    nome
	base    *tipoSimples
	builtin string // the XSD primitive at the root of the derivation

	enumeracoes []string // nil when the level declares none
	padroes     []padrao // alternatives at this level
	minimo      int      // minLength, -1 when unset
	maximo      int      // maxLength, -1 when unset
	exato       int      // length, -1 when unset
	espacos     string   // whiteSpace: preserve, replace or collapse; "" inherits
}

type padrao struct {
	fonte string
	re    *regexp.Regexp
}

type atributo struct {
	nome        nome
	tipo        *tipoSimples
	obrigatorio bool
}

// tipoComplexo is element-only, mixed, simple content, or empty.
type tipoComplexo struct {
	nome      nome
	conteudo  *particula   // nil for empty or simple content
	simples   *tipoSimples // simpleContent
	misto     bool
	atributos []atributo
}

// elemento is an element declaration: exactly one of the two types is set.
type elemento struct {
	nome     nome
	simples  *tipoSimples
	complexo *tipoComplexo
}

type tipoParticula int

const (
	particulaElemento tipoParticula = iota
	particulaSequencia
	particulaEscolha
	particulaQualquer
)

// particula is a node of a content model.
type particula struct {
	tipo     tipoParticula
	min, max int

	elemento *elemento    // particulaElemento
	filhos   []*particula // particulaSequencia and particulaEscolha

	// particulaQualquer: the namespaces accepted ("##any", "##other" or a
	// list) relative to alvo, the target namespace of the declaring schema.
	namespaces []string
	alvo       string

	// processamento is processContents: "strict" requires a declaration for
	// the element that matched, "lax" validates it only when one exists, and
	// "skip" does not look inside.
	processamento string
}
