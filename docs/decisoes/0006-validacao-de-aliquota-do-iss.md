# 0006 — Validar a alíquota do ISS antes de enviar

**Status:** Aceita
**Data:** 2026-09-18

## Contexto

A pergunta que originou esta decisão foi direta: *"o que acontece se eu mandar a
alíquota errada na emissão? a apuração não vai pegar?"*

Não pega. Quem valida `pAliq` é a Sefin Nacional, na recepção da DPS, e a
resposta é uma rejeição — não uma nota com o imposto recalculado. A nota
simplesmente não existe. Pior: até aqui o emissor mandava a alíquota que
estivesse no `nfse.yaml` sem olhar para ela, e o usuário só descobria o erro
depois de gastar a viagem até o governo.

As regras estão na planilha oficial
`docs/anexos/ANEXO_I-SEFIN_ADN-DPS_NFSe-SNNFSe-v1.00-20251226.xlsx`, aba
"RN DPS_NFS-e". Lidas juntas, elas dizem que *quem pode declarar alíquota*
depende de três campos que já estavam no XML e não conversavam entre si:
`opSimpNac` (situação no Simples), `regApTribSN` (regime de apuração) e
`tpRetISSQN` (quem retém).

| Regra | Situação | Alíquota |
|---|---|---|
| E0595 | qualquer um | nunca acima de 5% |
| E0600 | MEI | não pode informar — o ISS sai no DAS |
| E0625 / E0631 | ME/EPP no Simples, sem retenção | não pode informar |
| E0621 / E0628 | ME/EPP no Simples, com retenção | **precisa** informar, mínimo 1,8% |
| E0635 / E0640 | ME/EPP apurando fora do Simples | depende do convênio do município |

Havia ainda um buraco de entrada: `tpRetISSQN` era sempre 1 porque não existia
nenhuma forma de dizer que o tomador retém. Com isso, o único caso em que
declarar alíquota é *obrigatório* era inalcançável pelo CLI.

## Decisão

Três coisas, nesta ordem.

**Dar entrada ao que faltava.** `--retencao` (`nao` | `tomador` |
`intermediario`) e `padroes.valores.retencao_issqn` alimentam `tpRetISSQN`;
`prestador.regime_apuracao` (`sn` | `iss-municipio` | `fora-do-sn`) alimenta
`regApTribSN` e tem `sn` como padrão, que é o caso de quem está dentro dos
limites do Simples.

**Validar localmente, em `internal/domain/validation`.** A função recebe os
quatro valores e devolve um erro em português dizendo o que fazer e citando o
código da regra, para quem quiser conferir na planilha:

```
erro: sem retencao do ISSQN, um ME/EPP do Simples nao pode informar aliquota
(a Sefin rejeita: E0625). Use iss_aliquota: 0, ou informe --retencao se o
tomador retem
```

**Não opinar onde não dá para saber.** E0635 e E0640 dependem de o município ter
convênio ativo com o Sistema Nacional. Isso não está na máquina — está numa
consulta ao ADN. Nesses casos a validação devolve `nil` e sai da frente.

O corte é este: a regra é aplicada localmente quando o caso ativo e o inativo
**concordam**. Para MEI e para ME/EPP dentro do Simples eles concordam, e é aí
que mora a quase totalidade dos usuários deste emissor.

## Consequências

- Uma classe inteira de rejeição vira mensagem no terminal, antes de assinar.
  O certificado não é usado para produzir um documento que já se sabe recusado.
- A validação é ingênua de propósito: só o que a planilha garante. Não há tabela
  de alíquotas por município embutida no binário, que envelheceria em silêncio.
- Existe um caminho que o emissor deixa passar e o governo pode recusar (ME/EPP
  apurando fora do Simples). Está documentado em
  [docs/convenio-municipal.md](../convenio-municipal.md), com como consultar.
- `validation.ValidateISSRate` é uma função pura sobre quatro inteiros e um
  float. Os testes não precisam de certificado, rede nem XML.

## Aprendizado

O padrão que já apareceu nos ADRs 0003 e 0005 apareceu de novo, com outra
roupa. Lá, o código estava errado porque ninguém o tinha confrontado com o XSD
nem com o swagger. Aqui, o código estava *incompleto* porque ninguém o tinha
confrontado com a planilha de regras de negócio — que estava versionada no
repositório desde o começo, em `docs/anexos/`.

Os três artefatos oficiais (esquema, contrato e regras) respondem perguntas
diferentes. O XSD diz que `pAliq` é um decimal válido. O swagger diz para onde
mandar. Só a planilha diz *quando* mandar. Um emissor que só consulta os dois
primeiros gera XML perfeitamente bem formado e perfeitamente rejeitado.

A segunda lição é sobre o que fazer com a incerteza. A tentação era chutar o
caso do convênio — provavelmente ativo, Curitiba é cidade grande. A alternativa
escolhida foi explicitar a fronteira: validar onde a resposta é a mesma nos dois
mundos, calar onde não é, e escrever onde perguntar. Uma validação que às vezes
mente é pior que uma validação que não existe, porque a primeira é confiada.
