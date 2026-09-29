# 0013 — Município informado pelo nome, resolvido por consulta e guardado em cache

**Status:** Aceita
**Data:** 2026-09-29

## Contexto

A [issue #11](https://github.com/edusouza/nfse-emissor-go/issues/11) pediu que o
`nfse onboard --sem-rede` resolvesse o código IBGE do município sem rede. Com
`--sem-rede`, ou quando a consulta ao cadastro público falha, o município é o
único campo que o `onboard` deixa em branco sem ter nada a oferecer. E é o pior
de digitar: sete dígitos que ninguém sabe de cabeça, com um erro que **não
aparece em lugar nenhum**. A Sefin aceita qualquer código válido, e a nota vai
para a cidade errada.

A proposta original era embutir a tabela do `ANEXO_A` no binário. O trabalho
chegou a ser feito (PR #19, `refs/pull/19/head`) e foi fechado sem merge. A
[ADR 0012](0012-municipio-por-consulta.md) registrou o motivo, que é do autor
do projeto: **não carregar um segundo cadastro inteiro dentro do binário.**

A issue continuou aberta. O problema é real; só a solução tinha sido recusada.

## Decisão

Resolver o problema sem a tabela, com três peças. Cada uma vale por si.

**1. O código carrega a própria conferência.** Um código de município do IBGE
tem o prefixo da UF nos dois primeiros dígitos e um dígito verificador no
último. Isso pega um dígito trocado, um código TOM da Receita (4 dígitos) ou um
código de UF, **sem tabela e sem rede**. O `pkg/codmun` faz essa conferência, e
ela passa a valer em todo lugar por onde um código entra: `--municipio`,
`prestador.municipio` no `config check`, `servico.municipio_prestacao` na nota
e a resposta do cadastro público ([#12](https://github.com/edusouza/nfse-emissor-go/issues/12)).
O IBGE numerou nove municípios sem dígito válido; eles estão listados no
código, e um teste confere o algoritmo contra os 5570 códigos do `ANEXO_A`.

**2. O nome é resolvido pela lista do estado, uma vez.** `--municipio
"Curitiba/PR"` pede ao serviço de localidades do IBGE a lista de municípios da
UF (`/estados/PR/municipios`) e procura o nome nela. A **lista inteira** vai
para o cache em disco que o DANFSe já usa
([ADR 0012](0012-municipio-por-consulta.md)). A UF é obrigatória: 232 nomes se
repetem entre estados, e escolher um poria a nota na cidade errada sem que nada
reclamasse. A consulta revela ao IBGE a UF de quem emite, e não o município.

**3. O cache torna o nome utilizável sem rede.** Depois da primeira consulta,
qualquer município daquela UF é resolvido com `--sem-rede`. Um nome que nunca
foi consultado, sem rede, gera um erro que diz as duas saídas: informar o
código ou rodar uma vez com rede.

## Consequências

- **O binário não cresce.** A decisão da ADR 0012 continua de pé.
- **`--sem-rede` resolve o código, mas não o nome, na primeira vez.** É o custo
  de não embutir a tabela, e é o ponto em que a proposta original era melhor.
  Quem roda sem rede na primeira vez precisa do código. A diferença é que agora
  um código errado é recusado na hora, em vez de virar uma nota na cidade
  errada.
- **Todo código de município digitado à mão passa a ser conferido.** Um
  `nfse.yaml` que o `config check` aceitava pode passar a ser recusado. Isso só
  acontece se o código estiver errado, e nesse caso as notas emitidas com ele
  foram para a cidade errada.
- **A lista do IBGE é conferida, não aceita de boa-fé.** Todo código que vem
  nela precisa ser válido e da UF pedida. Senão, a resposta é descartada
  inteira: uma lista que pode pôr um nome no código errado é pior do que
  nenhuma.
- **A normalização dos nomes é testada contra a tabela oficial.** Acentos,
  caixa, espaços e pontuação caem, e `Alta Floresta D'Oeste` e `alta floresta
  doeste` viram a mesma chave. Um teste lê o `ANEXO_A` e garante duas coisas:
  que dois municípios da mesma UF nunca dão a mesma chave, e que nenhuma letra
  dos nomes oficiais escapa da normalização. A tabela entra nos testes, não no
  binário.

## Alternativas consideradas

**Embutir a tabela** (PR #19). Resolve tudo offline, desde a primeira vez. Foi
recusada pelo autor, e a razão está na ADR 0012.

**Pedir ao usuário um arquivo com a tabela** (`--tabela-municipios
ANEXO_A.xlsx`). Funcionaria offline, mas empurra para o usuário a tarefa de
achar a planilha certa no portal, justo o trabalho que o `onboard` existe para
poupar.

**Consultar o município pelo nome direto no IBGE.** O serviço não tem busca por
nome. Consultar a lista da UF custa uma requisição, e ela serve para todas as
consultas seguintes.

## Aprendizado

"Não embutir a tabela" parecia fechar a porta do `--sem-rede`. Não fechou. O
que o `--sem-rede` precisava de verdade era não deixar passar um código errado,
e isso cabe em nove exceções e um dígito verificador. O nome é conveniência, e
a conveniência pode depender da rede uma vez.
