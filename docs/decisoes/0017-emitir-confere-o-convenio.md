# 0017 — O `emitir` consulta o convênio para aplicar E0635 e E0640

**Status:** Aceita
**Data:** 2026-09-30
**Complementa:** [ADR 0006](0006-validacao-de-aliquota-do-iss.md)

## Contexto

A [ADR 0006](0006-validacao-de-aliquota-do-iss.md) deixou duas regras de
alíquota de fora, de propósito: E0635 e E0640 dependem de o convênio do
município estar ativo, e isso "não está na máquina". Para um ME/EPP que apura o
ISSQN pela alíquota do município (`regime_apuracao` `iss-municipio` ou
`fora-do-sn`), as duas dizem o contrário uma da outra:

| Convênio | pAliq | Regra |
|---|---|---|
| ativo | proibida: a Sefin aplica a parametrizada | E0635 |
| inativo | obrigatória | E0640 |

Com o cliente do ADN e o cache de 24 horas
([ADR 0016](0016-parametros-municipais-com-cache.md)), a resposta passou a
estar a uma consulta de distância.

## Decisão

**O `emitir` consulta o convênio antes de assinar, e recusa com a regra
citada.** A escolha foi do autor, entre recusar, só avisar e não mexer no
`emitir`.

- **Só quem depende paga a viagem.** A consulta acontece apenas para ME/EPP
  apurando fora do Simples (`ISSRateContext.DependsOnConvenio`). MEI e ME/EPP no
  Simples, a quase totalidade dos usuários, continuam sem nenhuma chamada nova.
- **Antes de assinar.** Uma DPS que a Sefin certamente recusaria não gasta o
  certificado nem um número da série.
- **Uma falha não impede a emissão.** Se o ADN não responde, recusa o
  certificado ou devolve algo inesperado, as regras ficam com a Sefin, como
  antes, e um aviso diz que não foram conferidas. Uma recusa local precisa ser
  certa, e uma resposta que não veio não é.
- **O município de incidência não é adivinhado.** A LC 116/2003, art. 3º, põe o
  ISSQN no estabelecimento do prestador ou no local da prestação conforme o
  serviço, e essa tabela não está no emissor. Quando os dois municípios são o
  mesmo, que é o padrão, não há dúvida. Quando diferem, os dois convênios são
  consultados: se concordam, a regra vale; se discordam, o emissor não opina e
  avisa. É o mesmo corte da ADR 0006: validar onde os dois mundos concordam.
- **`--sem-assinar` não consulta.** O ADN só responde a um certificado, e esse
  caminho não carrega nenhum. Um aviso diz isso.
- **A recusa diz de onde veio a resposta** e, quando ela veio do cache, como
  consultar de novo (`nfse parametros ... --sem-cache`).

## Consequências

- Mais uma classe de rejeição vira mensagem no terminal, antes de assinar.
- Quem apura fora do Simples passa a depender de uma segunda viagem ao governo,
  no máximo uma vez por dia e por município, graças ao cache. Uma falha nessa
  viagem só custa o aviso.
- **A rota do convênio não recebe a competência.** A resposta é a situação de
  hoje, e as regras falam da situação na competência da nota. Para uma nota com
  competência antiga num município que mudou de situação depois, o emissor pode
  errar para os dois lados: recusar uma DPS que a Sefin aceitaria, ou deixar
  passar uma que ela rejeita. Nos dois casos quem decide é a Sefin, e o pior
  resultado é uma rejeição, nunca uma nota errada.
- O critério de "ativo" é o que o serviço mostrou em 30/09/2026: parâmetros
  para um convênio ativo e 404 com o motivo para um inativo
  ([docs/convenio-municipal.md](../convenio-municipal.md)). Um 404 sem o
  envelope do serviço é tratado como falha, nunca como "inativo".

## Alternativas consideradas

**Só avisar.** Não bloquearia nada, mas um aviso sobre uma rejeição certa
treina o usuário a ignorar avisos.

**Não mexer no `emitir`** e deixar a resposta no `nfse parametros`. É o que a
ADR 0006 fazia, com o custo de o usuário precisar lembrar de consultar.

**Embutir a tabela do art. 3º** para decidir o município de incidência. Resolve
os casos em que os convênios discordam, mas é o tipo de tabela que envelhece em
silêncio, e o ganho é pequeno: os dois municípios só diferem quando a nota diz
isso explicitamente.
