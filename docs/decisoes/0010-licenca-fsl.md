# 0010 — Licença FSL-1.1-MIT no lugar do MIT

**Status:** Aceita
**Data:** 2026-09-21

## Contexto

O projeto nasceu sob MIT. A pergunta que abriu esta decisão foi como proteger o
trabalho de empresas que se apropriem dele.

A palavra "empresa" juntava dois usos muito diferentes:

1. uma empresa que **usa** o emissor para emitir as próprias notas;
2. uma empresa que **embrulha** o emissor num produto ou serviço e revende.

O público declarado do projeto é o primeiro grupo: prestadores do Simples
Nacional, MEI, ME e EPP. A ideia inicial — cobrar de quem fatura acima de
R$100 mil por ano — atinge justamente esse grupo. ME fatura até R$360 mil e EPP
até R$4,8 milhões, de modo que quase todo usuário real cairia no lado pago; e o
emissor existe precisamente para que o prestador pequeno emita sozinho, sem
portal e sem intermediário. Cobrar dele seria cobrar de quem o projeto quis
servir.

O free-riding que incomoda é o segundo caso.

Havia ainda um limite que nenhuma decisão move: **o MIT já publicado é
irrevogável.** As tags `v0.5.0`, `v0.5.1`, `v0.5.2` e `v0.6.0` estão no
`proxy.golang.org`, que é imutável e guarda o zip indefinidamente. Quem baixar
qualquer uma delas recebe o código sob MIT para sempre, inclusive para
concorrer. Relicenciar governa o que vier depois; não recolhe o que saiu.

## Decisão

**FSL-1.1-MIT** — *Functional Source License, Version 1.1, MIT Future License*.

O eixo da licença é concorrência, não tamanho:

- **Permitted Purpose** é tudo que não seja *Competing Use*. O texto lista
  explicitamente o uso interno, sem ressalva de porte ou faturamento, além de
  educação e pesquisa não comerciais e serviços profissionais prestados a quem
  também é licenciado.
- **Competing Use** é disponibilizar o software a terceiros num produto ou
  serviço comercial que substitua o software, substitua outro produto ou
  serviço que eu ofereça com ele, ou ofereça funcionalidade igual ou
  substancialmente similar.
- **Grant of Future License**: cada versão publicada vira MIT dois anos depois,
  de forma irrevogável. A v0.7.0 é MIT em setembro de 2028.

O texto entra **sem alteração**. A FSL não tem parâmetro para preencher além de
ano e titular, e é isso que a torna reconhecível — um jurídico que já a viu não
precisa reler.

Junto com ela:

- cabeçalho `SPDX-License-Identifier` em todo arquivo `.go`;
- um teste que recusa fonte sem cabeçalho e compara o identificador declarado
  com o que está no `LICENSE.md`;
- `nfse versao` passa a informar a licença e o link, porque um binário viaja
  sem o repositório em volta e a cláusula de redistribuição pede que os termos,
  ou um link para eles, acompanhem cada cópia.

## Consequências

- **O projeto deixa de ser código aberto** pela definição da OSI: restrição de
  campo de uso reprova. O `pkg.go.dev` só renderiza documentação de licenças
  reconhecidas, então a página do pacote fica sem doc e marcada como não
  redistribuível. O `go install` continua funcionando — o proxy serve o módulo
  do mesmo jeito.
- **Para quem usa o emissor para emitir as próprias notas, nada muda**, de MEI a
  empresa grande. Era o objetivo.
- Quem quiser oferecer um serviço de emissão construído sobre este código
  precisa de acordo — ou dos dois anos.
- **Contribuição externa fica mais difícil.** Quem manda um PR precisa aceitar
  que o código sai sob FSL. Projetos fair source costumam resolver isso com um
  CLA; não há um aqui, e enquanto não houver, contribuição de terceiro merece
  conversa antes do merge.
- **Não existe ainda caminho para comprar uma licença de uso concorrente.**
  Enquanto não existir, a resposta é falar comigo. Uma licença dupla sem forma
  de comprar a segunda metade é uma porta fechada com placa de "aberto".

## Alternativas consideradas

**Limite por faturamento.** Era a ideia original, em duas formas possíveis. A
[PolyForm Small Business 1.0.0](https://polyformproject.org/licenses/small-business/1.0.0/)
é pronta e reconhecida, mas o limiar é fixo no texto — menos de 100 pessoas e
menos de US$1 milhão de receita no ano anterior — e não se edita sem deixar de
ser PolyForm. A [BSL 1.1](https://mariadb.com/bsl11/) permite escrever o próprio
limiar no campo livre *Additional Use Grant*, ao custo de uma data de conversão
obrigatória em no máximo quatro anos, para uma licença compatível com a GPL.
Ambas foram descartadas pelo mesmo motivo: cobram do usuário final, que aqui é
o prestador pequeno.

**Licença própria com o limite de R$100 mil.** Daria exatamente o número
pedido. Jurídico de empresa costuma recusar licença que nunca viu, o que na
prática afasta justamente o comprador do plano comercial em vez de convertê-lo.

**Continuar MIT.** É o que o projeto era, e tem a virtude de não custar nada a
ninguém. Descartado porque não oferece resposta nenhuma ao caso que motivou a
pergunta.

## Aprendizado

A pergunta era "como impedir que empresas usem meu trabalho", e a resposta útil
não estava em escolher uma licença mais dura: estava em separar dois usos que a
palavra "empresa" tinha juntado. Um deles é o próprio público do projeto.

O outro aprendizado é mais caro e não tem conserto: **a licença inicial de um
projeto é escolhida para sempre, versão por versão.** Tudo que saiu sob MIT
continua sob MIT, no proxy, indefinidamente. A decisão de hoje vale da v0.7.0
em diante — e teria valido de muito mais coisa se tivesse sido tomada antes da
primeira publicação.
