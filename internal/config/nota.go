package config

import (
	"fmt"
	"strings"
	"time"
)

// CompetenciaLayout is the date format accepted in the "competencia" field.
const CompetenciaLayout = "2006-01-02"

// Merge returns n with every field set in override applied on top of it.
// A zero value in override means "not specified", so the underlying value from
// the defaults survives. The one exception is ISSAliquota, which is a pointer
// precisely so that an explicit rate of 0 can override a non-zero default.
func (n Nota) Merge(override Nota) Nota {
	merged := n

	if override.Numero != "" {
		merged.Numero = override.Numero
	}
	if override.Competencia != "" {
		merged.Competencia = override.Competencia
	}

	if override.Servico.CodigoTributacaoNacional != "" {
		merged.Servico.CodigoTributacaoNacional = override.Servico.CodigoTributacaoNacional
	}
	if override.Servico.MunicipioPrestacao != "" {
		merged.Servico.MunicipioPrestacao = override.Servico.MunicipioPrestacao
	}
	if override.Servico.Descricao != "" {
		merged.Servico.Descricao = override.Servico.Descricao
	}

	if override.Valores.ValorServico != 0 {
		merged.Valores.ValorServico = override.Valores.ValorServico
	}
	if override.Valores.DescontoIncondicionado != 0 {
		merged.Valores.DescontoIncondicionado = override.Valores.DescontoIncondicionado
	}
	if override.Valores.DescontoCondicionado != 0 {
		merged.Valores.DescontoCondicionado = override.Valores.DescontoCondicionado
	}
	if override.Valores.Deducoes != 0 {
		merged.Valores.Deducoes = override.Valores.Deducoes
	}
	if override.Valores.ISSAliquota != nil {
		merged.Valores.ISSAliquota = override.Valores.ISSAliquota
	}

	// The taker is replaced wholesale rather than field by field. Merging an
	// identified taker over a "nao_identificado" default field by field would
	// leave the flag set and silently drop the client from the invoice.
	if override.Tomador != nil {
		tomador := *override.Tomador
		merged.Tomador = &tomador
	}

	return merged
}

// CompetenciaDate parses the competence date, defaulting to today when unset.
func (n Nota) CompetenciaDate() (time.Time, error) {
	if n.Competencia == "" {
		return time.Now(), nil
	}
	t, err := time.Parse(CompetenciaLayout, n.Competencia)
	if err != nil {
		return time.Time{}, fmt.Errorf("competencia %q invalida: use o formato AAAA-MM-DD", n.Competencia)
	}
	return t, nil
}

// ISSRate returns the ISS rate, treating an unset rate as zero.
func (n Nota) ISSRate() float64 {
	if n.Valores.ISSAliquota == nil {
		return 0
	}
	return *n.Valores.ISSAliquota
}

// Validate checks a fully merged invoice.
func (n Nota) Validate() error {
	var problems []string

	if n.Numero == "" {
		problems = append(problems, "numero: obrigatorio (use --numero)")
	} else if len(n.Numero) > 15 {
		problems = append(problems, fmt.Sprintf("numero: %q excede 15 digitos", n.Numero))
	}

	if _, err := n.CompetenciaDate(); err != nil {
		problems = append(problems, err.Error())
	}

	if n.Servico.CodigoTributacaoNacional == "" {
		problems = append(problems, "servico.codigo_tributacao_nacional: obrigatorio (6 digitos, LC 116/2003)")
	} else if len(n.Servico.CodigoTributacaoNacional) != 6 {
		problems = append(problems, fmt.Sprintf("servico.codigo_tributacao_nacional: %q deve ter 6 digitos",
			n.Servico.CodigoTributacaoNacional))
	}

	if n.Servico.Descricao == "" {
		problems = append(problems, "servico.descricao: obrigatorio (use --descricao)")
	}

	if n.Valores.ValorServico <= 0 {
		problems = append(problems, "valores.valor_servico: deve ser maior que zero (use --valor)")
	}

	if t := n.Tomador; t != nil && !t.NaoIdentificado {
		if t.CNPJ == "" && t.CPF == "" {
			problems = append(problems, "tomador: informe cnpj ou cpf, ou marque nao_identificado")
		}
		if t.CNPJ != "" && t.CPF != "" {
			problems = append(problems, "tomador: informe cnpj ou cpf, nunca os dois")
		}
		if t.Nome == "" {
			problems = append(problems, "tomador.nome: obrigatorio quando o tomador e identificado")
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("dados da nota invalidos:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}
