# 0003 — Ancorar o gerador de XML no XSD oficial

**Status:** Aceita
**Data:** 2026-09-18

## Contexto

Ao ligar o gerador de DPS ao validador estrutural pela primeira vez, os dois
discordaram. Um dizia que `cTribNac` estava em `serv/cServ/cTribNac`, o outro
procurava em `serv/cTribNac`.

Como o XSD oficial está versionado no repositório (`docs/schemas/`), foi
possível decidir a disputa sem chutar. A resposta: **os dois estavam errados em
pontos diferentes**, e o gerador estava errado em mais lugares do que o
desacordo revelava.

Comparando `tiposComplexos_v1.00.xsd` com o XML gerado, apareceram sete
divergências:

| # | O que era gerado | O que o schema define |
|---|------------------|-----------------------|
| 1 | `regTrib` só com `opSimpNac` | `regEspTrib` é obrigatório |
| 2 | `<subst>2</subst>` | `subst` é uma estrutura (`chSubstda`, `cMotivo`), omitida quando não há substituição |
| 3 | `xDescServ` irmão de `cServ` | `xDescServ` é filho de `cServ` |
| 4 | `vDescIncond`/`vDescCond` dentro de `vServPrest` | ficam em `vDescCondIncond`, irmão de `vServPrest` |
| 5 | `BM` carregando `vBCCalc`, `pAliq` e `vISS` | `BM` é benefício municipal (`nBM`, `vRedBCBM`, `pRedBCBM`) |
| 6 | `tpRetISSQN` ausente; `pAliq` dentro de `BM` | ambos são filhos diretos de `tribMun`, e `tpRetISSQN` é obrigatório |
| 7 | `totTrib` irmão de `trib`, com dois filhos | fica dentro de `trib`, e é um `xs:choice` de exatamente um filho |

O item 5 é o mais revelador. A DPS **não tem** base de cálculo nem valor de ISS:
quem calcula é o governo, e os valores voltam na NFS-e. O gerador inventava os
dois e os enfiava num elemento de benefício municipal.

## Decisão

Reescrever as seções `prest`, `serv` e `valores` do `pkg/xmlbuilder` a partir do
XSD, corrigir os caminhos do validador estrutural, e escrever os testes
afirmando caminhos de elemento em vez de trechos de string.

`DPSValues` perdeu os campos `TaxBase` e `ISSAmount`, que não existem na DPS.
`DPSConfig.Substitution` deixou de ser `int` e virou `*DPSSubstitution`.

## Consequências

- O XML gerado passa a ter a forma que a Sefin espera. Antes seria rejeitado.
- Testes agora navegam a árvore (`DPS/infDPS/valores/trib/tribMun/pAliq`) em vez
  de procurar `strings.Contains(xml, "<vISS>")`. Um teste de substring passa
  mesmo com o elemento no lugar errado — foi assim que os sete defeitos
  sobreviveram a uma suíte verde.
- Há um teste dedicado à ordem dos filhos de `valores`: o schema declara
  `xs:sequence`, então ordem é parte da validade.

## Aprendizado

Os XSDs estavam no repositório desde o primeiro commit e nunca foram lidos por
código nem por gente. Um schema versionado que ninguém consulta é documentação
morta.

Quando duas partes do sistema discordam sobre um formato externo, a tentação é
escolher a que parece mais certa e seguir. O desacordo era, na verdade, um sinal
de que **nenhuma** das duas tinha ido à fonte.
