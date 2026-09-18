package xmlbuilder

import (
	"strings"
	"testing"

	"github.com/beevik/etree"
)

// The assertions follow TCPedRegEvt, TCInfPedReg and TE101101 in
// tiposEventos_v1.00.xsd, plus TSIdPedRegEvt in tiposSimples_v1.00.xsd.

func validCancellation() CancellationConfig {
	return CancellationConfig{
		Environment:        2,
		ApplicationVersion: "nfse-cli v0.4.0",
		AuthorCNPJ:         "12345678000195",
		AccessKey:          strings.Repeat("1", 50),
		ReasonCode:         CancelReasonIssuingError,
		Reason:             "Valor do servico lancado incorretamente na nota",
	}
}

func TestBuildCancellation(t *testing.T) {
	result, err := BuildCancellation(validCancellation())
	if err != nil {
		t.Fatalf("BuildCancellation falhou: %v", err)
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromString(result.XML); err != nil {
		t.Fatalf("XML gerado nao e parseavel: %v", err)
	}

	// TSIdPedRegEvt is "PRE" followed by 56 digits: the access key plus the
	// event code. The three-digit sequence that appears in the resulting
	// event's identifier is assigned by the government, not sent here.
	wantID := "PRE" + strings.Repeat("1", 50) + "101101"
	if result.RequestID != wantID {
		t.Errorf("Id = %q, esperava %q", result.RequestID, wantID)
	}
	if digits := len(result.RequestID) - 3; digits != 56 {
		t.Errorf("Id tem %d digitos apos PRE, esperava 56", digits)
	}

	checks := map[string]string{
		"pedRegEvento/infPedReg/tpAmb":           "2",
		"pedRegEvento/infPedReg/verAplic":        "nfse-cli v0.4.0",
		"pedRegEvento/infPedReg/CNPJAutor":       "12345678000195",
		"pedRegEvento/infPedReg/chNFSe":          strings.Repeat("1", 50),
		"pedRegEvento/infPedReg/e101101/xDesc":   "Cancelamento de NFS-e",
		"pedRegEvento/infPedReg/e101101/cMotivo": "1",
	}
	for p, want := range checks {
		el := doc.FindElement(p)
		if el == nil {
			t.Errorf("%s ausente", p)
			continue
		}
		if el.Text() != want {
			t.Errorf("%s = %q, esperava %q", p, el.Text(), want)
		}
	}

	if got := doc.FindElement("pedRegEvento/infPedReg").SelectAttrValue("Id", ""); got != wantID {
		t.Errorf("atributo Id = %q", got)
	}
	// CNPJAutor and CPFAutor are a choice: only one may appear.
	if doc.FindElement("pedRegEvento/infPedReg/CPFAutor") != nil {
		t.Error("CPFAutor nao deveria estar presente junto de CNPJAutor")
	}
}

func TestBuildCancellation_ElementOrder(t *testing.T) {
	result, err := BuildCancellation(validCancellation())
	if err != nil {
		t.Fatal(err)
	}
	doc := etree.NewDocument()
	if err := doc.ReadFromString(result.XML); err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, child := range doc.FindElement("pedRegEvento/infPedReg").ChildElements() {
		got = append(got, child.Tag)
	}

	// TCInfPedReg declares a sequence, so order is part of validity.
	want := []string{"tpAmb", "verAplic", "dhEvento", "CNPJAutor", "chNFSe", "e101101"}
	if len(got) != len(want) {
		t.Fatalf("filhos = %v, esperava %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("filhos = %v, esperava %v", got, want)
		}
	}
}

func TestBuildCancellation_Validation(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*CancellationConfig)
		wantErr string
	}{
		{
			name:    "chave curta",
			mutate:  func(c *CancellationConfig) { c.AccessKey = "123" },
			wantErr: "50 digitos",
		},
		{
			name:    "sem autor",
			mutate:  func(c *CancellationConfig) { c.AuthorCNPJ = "" },
			wantErr: "CNPJ ou o CPF",
		},
		{
			name:    "CNPJ e CPF juntos",
			mutate:  func(c *CancellationConfig) { c.AuthorCPF = "12345678900" },
			wantErr: "nunca os dois",
		},
		{
			name:    "motivo invalido",
			mutate:  func(c *CancellationConfig) { c.ReasonCode = "7" },
			wantErr: "invalido",
		},
		{
			// TSMotivo requires at least 15 characters. The floor is the
			// schema's way of demanding a real explanation on the fiscal record.
			name:    "justificativa curta demais",
			mutate:  func(c *CancellationConfig) { c.Reason = "erro" },
			wantErr: "ao menos 15",
		},
		{
			name:    "justificativa longa demais",
			mutate:  func(c *CancellationConfig) { c.Reason = strings.Repeat("a", 256) },
			wantErr: "excede 255",
		},
		{
			name:    "ambiente invalido",
			mutate:  func(c *CancellationConfig) { c.Environment = 3 },
			wantErr: "ambiente",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validCancellation()
			tc.mutate(&cfg)

			_, err := BuildCancellation(cfg)
			if err == nil {
				t.Fatalf("esperava erro mencionando %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("a mensagem nao menciona %q: %v", tc.wantErr, err)
			}
		})
	}
}
