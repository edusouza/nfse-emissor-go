package cli

import (
	"strings"
	"testing"

	"github.com/edusouza/nfse-emissor-go/internal/domain/esquema"
)

// What leaves the emitter is the signed DPS, so that is what has to pass the
// schema — ds:Signature included, which the XSD imports from XML-DSig.
func TestEmitir_DPSAssinadaPassaNoSchema(t *testing.T) {
	for nome, args := range map[string][]string{
		"simples": {"--numero", "7", "--valor", "1500", "--descricao", "Consultoria"},
		// --deducoes used to put pDR and vDR side by side, which the
		// schema's choice refuses.
		"com deducoes": {"--numero", "8", "--valor", "1500", "--deducoes", "200", "--descricao", "Consultoria"},
	} {
		t.Run(nome, func(t *testing.T) {
			dir := workspace(t)
			out, err := runEmit(t, dir, args...)
			if err != nil {
				t.Fatalf("emitir falhou: %v\n%s", err, out)
			}

			xml := onlyXML(t, dir)
			if !strings.Contains(xml, "<Signature") {
				t.Fatalf("a DPS nao foi assinada:\n%s", xml)
			}

			schema, err := esquema.DPS()
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range schema.Validar([]byte(xml)) {
				t.Errorf("%v", e)
			}
		})
	}
}

// The signature is validated too, by the XML-DSig schema the DPS schema
// imports: a signature missing its value is caught locally.
func TestEmitir_SchemaConfereAAssinatura(t *testing.T) {
	dir := workspace(t)
	if out, err := runEmit(t, dir, "--numero", "10", "--valor", "1500", "--descricao", "Consultoria"); err != nil {
		t.Fatalf("emitir falhou: %v\n%s", err, out)
	}
	xml := onlyXML(t, dir)

	i, j := strings.Index(xml, "<SignatureValue>"), strings.Index(xml, "</SignatureValue>")
	if i < 0 || j < 0 {
		t.Fatalf("a DPS nao tem SignatureValue:\n%s", xml)
	}
	adulterado := xml[:i] + xml[j+len("</SignatureValue>"):]

	schema, err := esquema.DPS()
	if err != nil {
		t.Fatal(err)
	}
	erros := schema.Validar([]byte(adulterado))
	if len(erros) == 0 || !strings.Contains(erros[0].Error(), "SignatureValue") {
		t.Errorf("esperava a falta de SignatureValue; veio %v", erros)
	}
}

// The message lists every problem with its path and says nothing was signed.
func TestValidarContraOSchema_Mensagem(t *testing.T) {
	err := validarContraOSchema(`<DPS xmlns="http://www.sped.fazenda.gov.br/nfse" versao="1.00"/>`)
	if err == nil {
		t.Fatal("aceitou uma DPS vazia")
	}
	for _, want := range []string{"schema oficial", "/DPS", "infDPS", "Nada foi assinado"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("a mensagem nao traz %q:\n%v", want, err)
		}
	}
}
