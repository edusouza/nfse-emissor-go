package esquema

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/beevik/etree"
)

// arquivos holds the schemas the emitter validates against: package v1.01 of
// the Sistema Nacional, 09/02/2026, the one the Sefin runs. Its TVerNFSe
// accepts "1.00|1.01", so it validates the documents the emitter writes
// today. The copies are byte-identical to docs/schemas, and a test keeps them
// that way.
//
//go:embed xsd/*.xsd
var arquivos embed.FS

// ArquivoDPS is the entry point of the DPS schema.
const ArquivoDPS = "xsd/DPS_v1.01.xsd"

// Esquema is a loaded schema: the global elements a document may start with.
type Esquema struct {
	raizes map[nome]*elemento
}

var dps = sync.OnceValues(func() (*Esquema, error) {
	return Carregar(arquivos, ArquivoDPS)
})

// DPS returns the schema of the DPS, loaded once per process from the
// embedded XSDs.
func DPS() (*Esquema, error) {
	return dps()
}

// Carregar reads a schema and everything it includes or imports from fsys.
func Carregar(fsys fs.FS, entrada string) (*Esquema, error) {
	c := &carregador{
		fs:        fsys,
		arquivos:  map[string]*arquivo{},
		brutos:    map[string]map[nome]declaracao{"simpleType": {}, "complexType": {}, "element": {}},
		simples:   map[nome]*tipoSimples{},
		complexos: map[nome]*tipoComplexo{},
		elementos: map[nome]*elemento{},
	}
	if _, err := c.carregarArquivo(entrada); err != nil {
		return nil, err
	}

	// Every global declaration is resolved now, not on first use, so that a
	// construct this package does not implement fails when the schema loads
	// rather than in the middle of someone's invoice.
	for tipo, decls := range c.brutos {
		for n := range decls {
			var err error
			switch tipo {
			case "simpleType":
				_, err = c.tipoSimples(n)
			case "complexType":
				_, err = c.tipoComplexo(n)
			case "element":
				_, err = c.elemento(n)
			}
			if err != nil {
				return nil, err
			}
		}
	}
	return &Esquema{raizes: c.elementos}, nil
}

type arquivo struct {
	nome        string
	alvo        string // targetNamespace
	qualificado bool   // elementFormDefault="qualified"
}

type declaracao struct {
	el  *etree.Element
	arq *arquivo
}

type carregador struct {
	fs       fs.FS
	arquivos map[string]*arquivo
	brutos   map[string]map[nome]declaracao

	simples   map[nome]*tipoSimples
	complexos map[nome]*tipoComplexo
	elementos map[nome]*elemento
}

func (c *carregador) carregarArquivo(caminho string) (*arquivo, error) {
	if a, ok := c.arquivos[caminho]; ok {
		return a, nil
	}

	conteudo, err := fs.ReadFile(c.fs, caminho)
	if err != nil {
		return nil, fmt.Errorf("schema %s: %w", caminho, err)
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(conteudo); err != nil {
		return nil, fmt.Errorf("schema %s: %w", caminho, err)
	}
	raiz := doc.Root()
	if raiz == nil || !deXSD(raiz, "schema") {
		return nil, fmt.Errorf("schema %s: a raiz nao e xs:schema", caminho)
	}

	a := &arquivo{
		nome:        caminho,
		alvo:        raiz.SelectAttrValue("targetNamespace", ""),
		qualificado: raiz.SelectAttrValue("elementFormDefault", "unqualified") == "qualified",
	}
	c.arquivos[caminho] = a

	for _, filho := range raiz.ChildElements() {
		if filho.NamespaceURI() != nsXSD {
			return nil, naoSuportado(a, filho)
		}
		switch filho.Tag {
		case "annotation":
		case "include", "import":
			local := filho.SelectAttrValue("schemaLocation", "")
			if local == "" {
				return nil, fmt.Errorf("schema %s: xs:%s sem schemaLocation", caminho, filho.Tag)
			}
			outro, err := c.carregarArquivo(path.Join(path.Dir(caminho), local))
			if err != nil {
				return nil, err
			}
			esperado := a.alvo
			if filho.Tag == "import" {
				esperado = filho.SelectAttrValue("namespace", "")
			}
			if outro.alvo != esperado {
				return nil, fmt.Errorf("schema %s: %s declara o namespace %q, esperava %q", caminho, local, outro.alvo, esperado)
			}
		case "simpleType", "complexType", "element":
			n := nome{ns: a.alvo, local: filho.SelectAttrValue("name", "")}
			if n.local == "" {
				return nil, fmt.Errorf("schema %s: xs:%s global sem nome", caminho, filho.Tag)
			}
			if _, dup := c.brutos[filho.Tag][n]; dup {
				return nil, fmt.Errorf("schema %s: xs:%s %s declarado duas vezes", caminho, filho.Tag, n.local)
			}
			c.brutos[filho.Tag][n] = declaracao{el: filho, arq: a}
		default:
			return nil, naoSuportado(a, filho)
		}
	}
	return a, nil
}

// tipoSimples resolves a named simple type. The type is registered before it
// is filled in, so a reference back to it finds it instead of looping.
func (c *carregador) tipoSimples(n nome) (*tipoSimples, error) {
	if t, ok := c.simples[n]; ok {
		return t, nil
	}
	if n.ns == nsXSD {
		return c.primitivo(n.local)
	}
	d, ok := c.brutos["simpleType"][n]
	if !ok {
		return nil, fmt.Errorf("tipo simples %s nao declarado", n.local)
	}
	t := &tipoSimples{nome: n}
	c.simples[n] = t
	return t, c.preencherSimples(t, d.el, d.arq)
}

func (c *carregador) primitivo(local string) (*tipoSimples, error) {
	n := nome{ns: nsXSD, local: local}
	if t, ok := c.simples[n]; ok {
		return t, nil
	}
	if _, ok := primitivos[local]; !ok {
		return nil, fmt.Errorf("tipo xs:%s nao suportado", local)
	}
	espacos := "collapse"
	if local == "string" {
		espacos = "preserve"
	}
	t := &tipoSimples{nome: n, builtin: local, minimo: -1, maximo: -1, exato: -1, espacos: espacos}
	c.simples[n] = t
	return t, nil
}

func (c *carregador) preencherSimples(t *tipoSimples, el *etree.Element, a *arquivo) error {
	t.minimo, t.maximo, t.exato = -1, -1, -1

	var restricao *etree.Element
	for _, filho := range el.ChildElements() {
		switch {
		case deXSD(filho, "annotation"):
		case deXSD(filho, "restriction") && restricao == nil:
			restricao = filho
		default:
			return naoSuportado(a, filho)
		}
	}
	if restricao == nil {
		return fmt.Errorf("schema %s: tipo simples %s sem xs:restriction", a.nome, t.nome.local)
	}

	var base *tipoSimples
	var err error
	if b := restricao.SelectAttrValue("base", ""); b != "" {
		base, err = c.tipoSimples(resolverNome(restricao, b))
	} else if inline := filhoXSD(restricao, "simpleType"); inline != nil {
		base = &tipoSimples{nome: nome{ns: a.alvo, local: t.nome.local + "/base"}}
		err = c.preencherSimples(base, inline, a)
	} else {
		err = fmt.Errorf("schema %s: restricao de %s sem base", a.nome, t.nome.local)
	}
	if err != nil {
		return err
	}
	t.base, t.builtin = base, base.builtin

	for _, faceta := range restricao.ChildElements() {
		if faceta.NamespaceURI() != nsXSD {
			return naoSuportado(a, faceta)
		}
		valor := faceta.SelectAttrValue("value", "")
		switch faceta.Tag {
		case "annotation", "simpleType":
		case "enumeration":
			if t.enumeracoes == nil {
				t.enumeracoes = []string{}
			}
			t.enumeracoes = append(t.enumeracoes, valor)
		case "pattern":
			re, err := traduzirPadrao(valor)
			if err != nil {
				return fmt.Errorf("schema %s: padrao de %s: %w", a.nome, t.nome.local, err)
			}
			t.padroes = append(t.padroes, padrao{fonte: valor, re: re})
		case "minLength", "maxLength", "length":
			n, err := strconv.Atoi(valor)
			if err != nil || n < 0 {
				return fmt.Errorf("schema %s: %s=%q em %s", a.nome, faceta.Tag, valor, t.nome.local)
			}
			switch faceta.Tag {
			case "minLength":
				t.minimo = n
			case "maxLength":
				t.maximo = n
			default:
				t.exato = n
			}
		case "whiteSpace":
			if valor != "preserve" && valor != "replace" && valor != "collapse" {
				return fmt.Errorf("schema %s: whiteSpace=%q em %s", a.nome, valor, t.nome.local)
			}
			t.espacos = valor
		default:
			return naoSuportado(a, faceta)
		}
	}
	return nil
}

func (c *carregador) tipoComplexo(n nome) (*tipoComplexo, error) {
	if t, ok := c.complexos[n]; ok {
		return t, nil
	}
	d, ok := c.brutos["complexType"][n]
	if !ok {
		return nil, fmt.Errorf("tipo complexo %s nao declarado", n.local)
	}
	t := &tipoComplexo{nome: n}
	c.complexos[n] = t
	return t, c.preencherComplexo(t, d.el, d.arq)
}

func (c *carregador) preencherComplexo(t *tipoComplexo, el *etree.Element, a *arquivo) error {
	t.misto = el.SelectAttrValue("mixed", "false") == "true"

	for _, filho := range el.ChildElements() {
		if filho.NamespaceURI() != nsXSD {
			return naoSuportado(a, filho)
		}
		switch filho.Tag {
		case "annotation":
		case "sequence", "choice":
			if t.conteudo != nil {
				return fmt.Errorf("schema %s: %s tem dois modelos de conteudo", a.nome, t.nome.local)
			}
			p, err := c.particula(filho, a)
			if err != nil {
				return err
			}
			t.conteudo = p
		case "attribute":
			at, err := c.atributo(filho, a)
			if err != nil {
				return err
			}
			t.atributos = append(t.atributos, at)
		case "simpleContent":
			ext := filhoXSD(filho, "extension")
			if ext == nil || len(filho.ChildElements()) != 1 {
				return naoSuportado(a, filho)
			}
			base, err := c.tipoSimples(resolverNome(ext, ext.SelectAttrValue("base", "")))
			if err != nil {
				return err
			}
			t.simples = base
			for _, at := range ext.ChildElements() {
				if !deXSD(at, "attribute") {
					return naoSuportado(a, at)
				}
				atributo, err := c.atributo(at, a)
				if err != nil {
					return err
				}
				t.atributos = append(t.atributos, atributo)
			}
		default:
			return naoSuportado(a, filho)
		}
	}
	return nil
}

func (c *carregador) atributo(el *etree.Element, a *arquivo) (atributo, error) {
	for _, proibido := range []string{"ref", "default", "fixed", "form"} {
		if el.SelectAttr(proibido) != nil {
			return atributo{}, fmt.Errorf("schema %s: atributo com %q nao suportado", a.nome, proibido)
		}
	}

	// attributeFormDefault is unqualified in every schema here: a local
	// attribute has no namespace.
	at := atributo{
		nome:        nome{local: el.SelectAttrValue("name", "")},
		obrigatorio: el.SelectAttrValue("use", "optional") == "required",
	}

	var err error
	if tipo := el.SelectAttrValue("type", ""); tipo != "" {
		at.tipo, err = c.tipoSimples(resolverNome(el, tipo))
	} else if inline := filhoXSD(el, "simpleType"); inline != nil {
		at.tipo = &tipoSimples{nome: nome{ns: a.alvo, local: "@" + at.nome.local}}
		err = c.preencherSimples(at.tipo, inline, a)
	} else {
		at.tipo, err = c.primitivo("string")
	}
	return at, err
}

func (c *carregador) particula(el *etree.Element, a *arquivo) (*particula, error) {
	p := &particula{min: 1, max: 1}

	if v := el.SelectAttrValue("minOccurs", ""); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("schema %s: minOccurs=%q", a.nome, v)
		}
		p.min = n
	}
	if v := el.SelectAttrValue("maxOccurs", ""); v == "unbounded" {
		p.max = ilimitado
	} else if v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n < p.min {
			return nil, fmt.Errorf("schema %s: maxOccurs=%q", a.nome, v)
		}
		p.max = n
	}

	switch {
	case deXSD(el, "element"):
		p.tipo = particulaElemento
		e, err := c.elementoLocal(el, a)
		if err != nil {
			return nil, err
		}
		p.elemento = e
	case deXSD(el, "sequence"), deXSD(el, "choice"):
		p.tipo = particulaSequencia
		if el.Tag == "choice" {
			p.tipo = particulaEscolha
		}
		for _, filho := range el.ChildElements() {
			if deXSD(filho, "annotation") {
				continue
			}
			f, err := c.particula(filho, a)
			if err != nil {
				return nil, err
			}
			p.filhos = append(p.filhos, f)
		}
	case deXSD(el, "any"):
		p.tipo = particulaQualquer
		p.namespaces = strings.Fields(el.SelectAttrValue("namespace", "##any"))
		p.alvo = a.alvo
		p.processamento = el.SelectAttrValue("processContents", "strict")
	default:
		return nil, naoSuportado(a, el)
	}
	return p, nil
}

// elementoLocal is an element inside a content model: a reference to a
// global declaration, or a local one.
func (c *carregador) elementoLocal(el *etree.Element, a *arquivo) (*elemento, error) {
	if ref := el.SelectAttrValue("ref", ""); ref != "" {
		return c.elemento(resolverNome(el, ref))
	}

	n := nome{local: el.SelectAttrValue("name", "")}
	if a.qualificado {
		n.ns = a.alvo
	}
	e := &elemento{nome: n}
	return e, c.tipoDoElemento(e, el, a)
}

func (c *carregador) elemento(n nome) (*elemento, error) {
	if e, ok := c.elementos[n]; ok {
		return e, nil
	}
	d, ok := c.brutos["element"][n]
	if !ok {
		return nil, fmt.Errorf("elemento %s nao declarado", n.local)
	}
	e := &elemento{nome: n}
	c.elementos[n] = e
	return e, c.tipoDoElemento(e, d.el, d.arq)
}

func (c *carregador) tipoDoElemento(e *elemento, el *etree.Element, a *arquivo) error {
	for _, proibido := range []string{"default", "fixed", "nillable", "abstract", "substitutionGroup"} {
		if el.SelectAttr(proibido) != nil {
			return fmt.Errorf("schema %s: elemento %s com %q nao suportado", a.nome, e.nome.local, proibido)
		}
	}

	if tipo := el.SelectAttrValue("type", ""); tipo != "" {
		n := resolverNome(el, tipo)
		if _, ok := c.brutos["complexType"][n]; ok {
			t, err := c.tipoComplexo(n)
			e.complexo = t
			return err
		}
		t, err := c.tipoSimples(n)
		e.simples = t
		return err
	}
	if inline := filhoXSD(el, "complexType"); inline != nil {
		e.complexo = &tipoComplexo{nome: nome{ns: a.alvo, local: e.nome.local}}
		return c.preencherComplexo(e.complexo, inline, a)
	}
	if inline := filhoXSD(el, "simpleType"); inline != nil {
		e.simples = &tipoSimples{nome: nome{ns: a.alvo, local: e.nome.local}}
		return c.preencherSimples(e.simples, inline, a)
	}
	return fmt.Errorf("schema %s: elemento %s sem tipo (xs:anyType nao e suportado)", a.nome, e.nome.local)
}

// resolverNome turns a QName written in the schema into a namespace and a
// local name, using the prefixes declared on the element or its ancestors.
func resolverNome(el *etree.Element, valor string) nome {
	prefixo, local := "", valor
	if i := strings.IndexByte(valor, ':'); i >= 0 {
		prefixo, local = valor[:i], valor[i+1:]
	}
	return nome{ns: namespaceDoPrefixo(el, prefixo), local: local}
}

func namespaceDoPrefixo(el *etree.Element, prefixo string) string {
	if prefixo == "xml" {
		return nsXML
	}
	for e := el; e != nil; e = e.Parent() {
		for _, at := range e.Attr {
			if prefixo == "" && at.Space == "" && at.Key == "xmlns" {
				return at.Value
			}
			if prefixo != "" && at.Space == "xmlns" && at.Key == prefixo {
				return at.Value
			}
		}
	}
	return ""
}

func deXSD(el *etree.Element, tag string) bool {
	return el.Tag == tag && el.NamespaceURI() == nsXSD
}

func filhoXSD(el *etree.Element, tag string) *etree.Element {
	for _, filho := range el.ChildElements() {
		if deXSD(filho, tag) {
			return filho
		}
	}
	return nil
}

func naoSuportado(a *arquivo, el *etree.Element) error {
	return fmt.Errorf("schema %s: <%s> nao e suportado por este validador", a.nome, el.FullTag())
}
