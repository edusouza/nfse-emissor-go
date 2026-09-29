package esquema

import (
	"fmt"
	"sort"
	"strings"

	"github.com/beevik/etree"
)

const nsXSI = "http://www.w3.org/2001/XMLSchema-instance"

// Erro is one way a document departs from the schema.
type Erro struct {
	// Caminho locates the element, as /DPS/infDPS/prest/CNPJ; an attribute
	// ends in /@name.
	Caminho  string
	Mensagem string
}

func (e Erro) Error() string {
	return e.Caminho + ": " + e.Mensagem
}

// Validar checks a document against the schema and returns every problem it
// finds, in document order. A document that is not well-formed XML is one
// problem.
func (e *Esquema) Validar(documento []byte) []Erro {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(documento); err != nil {
		return []Erro{{Caminho: "/", Mensagem: fmt.Sprintf("XML malformado: %v", err)}}
	}
	raiz := doc.Root()
	if raiz == nil {
		return []Erro{{Caminho: "/", Mensagem: "documento vazio"}}
	}

	v := &validador{esquema: e}
	decl, ok := e.raizes[nomeDe(raiz)]
	if !ok {
		return []Erro{{Caminho: "/" + raiz.Tag, Mensagem: fmt.Sprintf(
			"o elemento raiz %s (namespace %q) nao e declarado pelo schema", raiz.Tag, raiz.NamespaceURI())}}
	}
	v.elemento(raiz, decl, "/"+raiz.Tag)
	return v.erros
}

type validador struct {
	esquema *Esquema
	erros   []Erro
}

func (v *validador) erro(caminho, formato string, args ...any) {
	v.erros = append(v.erros, Erro{Caminho: caminho, Mensagem: fmt.Sprintf(formato, args...)})
}

func (v *validador) elemento(el *etree.Element, decl *elemento, caminho string) {
	if decl.simples != nil {
		v.atributos(el, nil, caminho)
		if len(el.ChildElements()) > 0 {
			v.erro(caminho, "e de tipo simples (%s) e nao pode conter elementos", decl.simples.nome.local)
			return
		}
		if problema := decl.simples.validar(texto(el)); problema != "" {
			v.erro(caminho, "%s", problema)
		}
		return
	}

	t := decl.complexo
	v.atributos(el, t.atributos, caminho)

	switch {
	case t.simples != nil:
		if len(el.ChildElements()) > 0 {
			v.erro(caminho, "nao pode conter elementos, so texto")
			return
		}
		if problema := t.simples.validar(texto(el)); problema != "" {
			v.erro(caminho, "%s", problema)
		}
		return
	case !t.misto && strings.TrimSpace(texto(el)) != "":
		v.erro(caminho, "nao pode conter texto, so elementos")
	}

	filhos := el.ChildElements()
	if t.conteudo == nil {
		if len(filhos) > 0 {
			v.erro(caminho+"/"+filhos[0].Tag, "nao e permitido: %s nao tem conteudo", el.Tag)
		}
		return
	}

	c := &casamento{filhos: filhos, longe: -1}
	if !contemPosicao(c.ocorrencias(t.conteudo, 0), len(filhos)) {
		v.erro(c.onde(caminho), "%s", c.explicar())
		return
	}

	declaracoes := map[nome]*elemento{}
	coletar(t.conteudo, declaracoes)
	contagem := map[string]int{}
	for _, filho := range filhos {
		contagem[filho.Tag]++
		caminhoFilho := caminho + "/" + filho.Tag
		if n := contarIrmaos(filhos, filho.Tag); n > 1 {
			caminhoFilho = fmt.Sprintf("%s[%d]", caminhoFilho, contagem[filho.Tag])
		}

		if d, ok := declaracoes[nomeDe(filho)]; ok {
			v.elemento(filho, d, caminhoFilho)
			continue
		}
		// Matched a wildcard. Whether to look inside depends on how the
		// wildcard processes its contents; the elements the NFS-e schemas
		// reach this way are the signature's, which are declared.
		v.qualquer(filho, t.conteudo, caminhoFilho)
	}
}

// qualquer validates an element that matched xs:any.
func (v *validador) qualquer(el *etree.Element, conteudo *particula, caminho string) {
	processamento := "strict"
	if p := wildcardQueAceita(conteudo, el.NamespaceURI()); p != nil {
		processamento = p.processamento
	}
	if processamento == "skip" {
		return
	}
	// The root table holds every global declaration of every schema loaded.
	if decl, ok := v.esquema.raizes[nomeDe(el)]; ok {
		v.elemento(el, decl, caminho)
		return
	}
	if processamento == "strict" {
		v.erro(caminho, "o elemento %s nao e declarado pelo schema", el.Tag)
	}
}

func (v *validador) atributos(el *etree.Element, declarados []atributo, caminho string) {
	presentes := map[nome]string{}
	for _, at := range el.Attr {
		if at.Space == "xmlns" || (at.Space == "" && at.Key == "xmlns") {
			continue
		}
		n := nome{local: at.Key}
		if at.Space != "" {
			n.ns = namespaceDoPrefixo(el, at.Space)
		}
		if n.ns == nsXSI {
			continue
		}
		presentes[n] = at.Value
	}

	for _, d := range declarados {
		valor, ok := presentes[d.nome]
		delete(presentes, d.nome)
		switch {
		case !ok && d.obrigatorio:
			v.erro(caminho, "falta o atributo obrigatorio %s", d.nome.local)
		case ok:
			if problema := d.tipo.validar(valor); problema != "" {
				v.erro(caminho+"/@"+d.nome.local, "%s", problema)
			}
		}
	}

	sobras := make([]string, 0, len(presentes))
	for n := range presentes {
		sobras = append(sobras, n.local)
	}
	sort.Strings(sobras)
	for _, s := range sobras {
		v.erro(caminho+"/@"+s, "atributo nao permitido em %s", el.Tag)
	}
}

// casamento matches a list of child elements against a content model.
//
// It tracks the set of positions each particle can end at rather than a
// single greedy choice, so an optional element followed by one with the same
// name, or a choice whose branches start alike, is decided by what comes
// after. The sets are bounded by the number of children, which in a DPS is a
// few dozen.
type casamento struct {
	filhos []*etree.Element

	// longe is the furthest position at which an element was expected, and
	// esperados what was expected there: the material for the message when
	// nothing matches. alcance is the furthest position any element reached;
	// past longe, it means the content model was already complete there.
	longe     int
	esperados []string
	alcance   int
}

// ocorrencias matches p, repeated between its bounds, from pos.
func (c *casamento) ocorrencias(p *particula, pos int) []int {
	fins := map[int]bool{}
	if p.min == 0 {
		fins[pos] = true
	}

	atual := []int{pos}
	visto := map[int]bool{}
	for i := 1; len(atual) > 0 && (p.max == ilimitado || i <= p.max); i++ {
		var prox []int
		for _, x := range atual {
			for _, y := range c.uma(p, x) {
				// Past the minimum, a position already reached adds
				// nothing: whatever follows it has been explored.
				if i > p.min && visto[y] {
					continue
				}
				visto[y] = true
				prox = append(prox, y)
				if i >= p.min {
					fins[y] = true
				}
			}
		}
		atual = unicos(prox)
		if i > p.min+len(c.filhos)+1 {
			break
		}
	}

	out := make([]int, 0, len(fins))
	for f := range fins {
		out = append(out, f)
	}
	sort.Ints(out)
	return out
}

// uma matches a single occurrence of p from pos.
func (c *casamento) uma(p *particula, pos int) []int {
	switch p.tipo {
	case particulaElemento:
		if pos < len(c.filhos) && nomeDe(c.filhos[pos]) == p.elemento.nome {
			c.alcance = max(c.alcance, pos+1)
			return []int{pos + 1}
		}
		c.esperar(pos, p.elemento.nome.local)
		return nil
	case particulaQualquer:
		if pos < len(c.filhos) && aceitaNamespace(p, c.filhos[pos].NamespaceURI()) {
			c.alcance = max(c.alcance, pos+1)
			return []int{pos + 1}
		}
		c.esperar(pos, "um elemento de outro namespace")
		return nil
	case particulaSequencia:
		atual := []int{pos}
		for _, f := range p.filhos {
			var prox []int
			for _, x := range atual {
				prox = append(prox, c.ocorrencias(f, x)...)
			}
			atual = unicos(prox)
			if len(atual) == 0 {
				return nil
			}
		}
		return atual
	case particulaEscolha:
		var fins []int
		for _, f := range p.filhos {
			fins = append(fins, c.ocorrencias(f, pos)...)
		}
		return unicos(fins)
	}
	return nil
}

func (c *casamento) esperar(pos int, oQue string) {
	if pos > c.longe {
		c.longe, c.esperados = pos, nil
	}
	if pos == c.longe && !contem(c.esperados, oQue) {
		c.esperados = append(c.esperados, oQue)
	}
}

// onde is the path the error is reported at: the unexpected child, or the
// parent when what is missing comes at the end.
func (c *casamento) onde(caminho string) string {
	if p := c.posicaoDoErro(); p < len(c.filhos) {
		return caminho + "/" + c.filhos[p].Tag
	}
	return caminho
}

func (c *casamento) posicaoDoErro() int {
	return max(c.longe, c.alcance)
}

func (c *casamento) explicar() string {
	p := c.posicaoDoErro()
	esperado := strings.Join(c.esperados, " ou ")
	switch {
	case c.alcance > c.longe && p < len(c.filhos):
		// Everything before this child matched and the model asked for
		// nothing more: the child is one too many.
		return "elemento nao permitido aqui: o conteudo ja esta completo antes dele"
	case p < len(c.filhos) && esperado != "":
		return fmt.Sprintf("elemento fora de lugar ou nao permitido; aqui o schema espera %s", esperado)
	case p < len(c.filhos):
		return "elemento nao permitido aqui"
	case esperado != "":
		return fmt.Sprintf("falta %s", esperado)
	}
	return "conteudo nao corresponde ao schema"
}

func aceitaNamespace(p *particula, ns string) bool {
	for _, aceito := range p.namespaces {
		switch aceito {
		case "##any":
			return true
		case "##other":
			if ns != p.alvo && ns != "" {
				return true
			}
		case "##targetNamespace":
			if ns == p.alvo {
				return true
			}
		case "##local":
			if ns == "" {
				return true
			}
		default:
			if ns == aceito {
				return true
			}
		}
	}
	return false
}

func wildcardQueAceita(p *particula, ns string) *particula {
	if p.tipo == particulaQualquer && aceitaNamespace(p, ns) {
		return p
	}
	for _, f := range p.filhos {
		if w := wildcardQueAceita(f, ns); w != nil {
			return w
		}
	}
	return nil
}

// coletar indexes the element declarations of a content model by name. XSD's
// "Element Declarations Consistent" rule guarantees that one name means one
// declaration within a content model.
func coletar(p *particula, out map[nome]*elemento) {
	if p.tipo == particulaElemento {
		out[p.elemento.nome] = p.elemento
	}
	for _, f := range p.filhos {
		coletar(f, out)
	}
}

func nomeDe(el *etree.Element) nome {
	return nome{ns: el.NamespaceURI(), local: el.Tag}
}

// texto concatenates the character data directly inside an element.
func texto(el *etree.Element) string {
	var b strings.Builder
	for _, t := range el.Child {
		if cd, ok := t.(*etree.CharData); ok {
			b.WriteString(cd.Data)
		}
	}
	return b.String()
}

func contarIrmaos(filhos []*etree.Element, tag string) int {
	n := 0
	for _, f := range filhos {
		if f.Tag == tag {
			n++
		}
	}
	return n
}

func contemPosicao(posicoes []int, p int) bool {
	for _, x := range posicoes {
		if x == p {
			return true
		}
	}
	return false
}

func unicos(xs []int) []int {
	if len(xs) < 2 {
		return xs
	}
	sort.Ints(xs)
	out := xs[:1]
	for _, x := range xs[1:] {
		if x != out[len(out)-1] {
			out = append(out, x)
		}
	}
	return out
}
