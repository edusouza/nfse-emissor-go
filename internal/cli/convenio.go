package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/edusouza/nfse-emissor-go/internal/config"
	"github.com/edusouza/nfse-emissor-go/internal/domain/validation"
	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/parametrizacao"
	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/xmlsigner"
)

// conferirConvenio applies E0635 and E0640, the two ISS rate rules that turn on
// the municipality's convênio, by asking the ADN (ADR 0017).
//
// Only an ME/EPP assessing the ISSQN outside the Simples depends on them, so
// nobody else pays the round trip. The answer comes from the 24-hour cache when
// it can (ADR 0016).
//
// A lookup that fails does not stop the emission: the rules are left to the
// Sefin, as they were before this check existed, and the user is told so. A
// refusal here has to be certain, and a missing answer is not.
func conferirConvenio(cmd *cobra.Command, cfg *config.Config, nota config.Nota, certInfo *xmlsigner.CertificateInfo) error {
	rateCtx := issRateContext(cfg, nota)
	if !rateCtx.DependsOnConvenio() {
		return nil
	}
	errOut := cmd.ErrOrStderr()

	tlsCert, err := certInfo.TLSCertificate()
	if err != nil {
		avisarConvenioNaoConferido(errOut, fmt.Sprintf("o certificado nao pode ser usado na conexao: %v", err))
		return nil
	}
	client, err := newParametrizacaoClient(parametrizacao.Config{
		Ambiente:    cfg.Ambiente,
		Certificate: tlsCert,
		OnRetry: func(wait time.Duration, _ error) {
			fmt.Fprintf(errOut, "O ADN nao respondeu; tentando de novo em %s...\n", wait)
		},
	})
	if err != nil {
		avisarConvenioNaoConferido(errOut, err.Error())
		return nil
	}

	cache := parametrizacao.NovoCache("")
	defer func() {
		if err := cache.Gravar(); err != nil {
			fmt.Fprintf(errOut, "aviso: nao consegui gravar o cache de parametros: %v\n", err)
		}
	}()
	consulta := parametrizacao.NovaConsulta(client, cache)

	var (
		municipios   = municipiosDeIncidencia(cfg, nota)
		ativo        *bool
		comoSoubemos []string
	)
	for _, municipio := range municipios {
		resposta, err := consulta.Convenio(cmd.Context(), municipio)
		if err != nil {
			avisarConvenioNaoConferido(errOut, fmt.Sprintf("a consulta do convenio de %s falhou: %v", municipio, err))
			return nil
		}

		situacao := descreverSituacao(resposta)
		fmt.Fprintf(errOut, "Convenio do municipio %s: %s\n", municipio, situacao)
		comoSoubemos = append(comoSoubemos, fmt.Sprintf("%s: %s", municipio, situacao))

		if ativo != nil && *ativo != resposta.Valor.Ativo {
			// The ISSQN is due where the provider is established or where the
			// service was provided, depending on the service (LC 116/2003,
			// art. 3º). With the two convênios disagreeing, E0635 and E0640
			// disagree too, and choosing one would be a guess.
			avisarConvenioNaoConferido(errOut, "o municipio do prestador e o da prestacao estao em situacoes "+
				"diferentes, e qual deles recebe o ISSQN depende do servico (LC 116/2003, art. 3o)")
			return nil
		}
		v := resposta.Valor.Ativo
		ativo = &v
	}

	rateCtx.ConvenioAtivo = ativo
	if err := validation.ValidateISSRate(rateCtx); err != nil {
		return fmt.Errorf("%w\nConvenio segundo o ADN, %s. Se o municipio mudou a situacao hoje, consulte de novo com:\n"+
			"  nfse parametros %s %s --sem-cache",
			err, juntar(comoSoubemos), municipios[0], nota.Servico.CodigoTributacaoNacional)
	}
	return nil
}

// municipiosDeIncidencia lists where the ISSQN may be due: the provider's
// municipality and, when it differs, the place of provision. LC 116/2003,
// art. 3º picks between them by service, and that table is not carried here;
// when both answer the same, which one it picks does not matter.
func municipiosDeIncidencia(cfg *config.Config, nota config.Nota) []string {
	municipios := []string{cfg.Prestador.Municipio}
	if p := nota.Servico.MunicipioPrestacao; p != "" && p != cfg.Prestador.Municipio {
		municipios = append(municipios, p)
	}
	return municipios
}

func descreverSituacao(r parametrizacao.Resposta[*parametrizacao.Convenio]) string {
	situacao := "inativo"
	if r.Valor.Ativo {
		situacao = "ativo"
	}
	if r.DoCache {
		return fmt.Sprintf("%s (resposta guardada, obtida em %s)", situacao, r.ConsultadoEm.Local().Format(layoutDataHora))
	}
	return situacao + " (consultado agora no ADN)"
}

func avisarConvenioNaoConferido(errOut io.Writer, motivo string) {
	fmt.Fprintf(errOut, "aviso: as regras E0635 e E0640, que dependem do convenio do municipio, nao foram "+
		"conferidas: %s. A Sefin as confere na recepcao.\n", motivo)
}

func juntar(partes []string) string {
	switch len(partes) {
	case 0:
		return ""
	case 1:
		return partes[0]
	default:
		return partes[0] + " e " + partes[1]
	}
}
