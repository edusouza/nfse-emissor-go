# 0019 — Rejeições explicadas no terminal, com tabelas geradas dos anexos I e II

**Status:** Aceita
**Data:** 2026-10-01

## Contexto

Quando recusa uma DPS, a Sefin responde com um código e uma frase:

```
erro: documento rejeitado pela Sefin Nacional
  - [E0600] Não é permitido informar a alíquota para prestador de serviço optante do simples nacional do tipo MEI.
```

A frase raramente diz qual campo corrigir e nunca diz qual condição falhou. Os
anexos de regras de negócio dizem as duas coisas: o caminho do campo no XML, o
texto da regra e o nível dela (1, leiaute; 2, regra geral; 3, depende do
município).

- O **ANEXO I** cobre a DPS, que o `emitir --enviar` e o `enviar` transmitem.
- O **ANEXO II** cobre os pedidos de registro de evento, entre eles o
  cancelamento que o `cancelar` transmite.

O site já publicava os códigos da DPS numa página gerada do ANEXO I
([ADR 0016](0016-site-de-documentacao.md)). Na hora da rejeição, porém, quem
emite está no terminal.

O catálogo antigo (`internal/domain/emission/errors.go`) tentava fazer esse
papel com 38 códigos inventados e foi removido na
[issue #37](https://github.com/edusouza/nfse-emissor-go/issues/37). A lição
ficou no CLAUDE.md: um mapa de códigos não se escreve à mão.

## Decisão

**O binário carrega duas tabelas geradas dos anexos** e acrescenta, sob cada
código que a Sefin devolve:

```
  - [E0600] Não é permitido informar a alíquota para prestador de serviço optante do simples nacional do tipo MEI.
      Campo   DPS/infDPS/valores/trib/tribMun/pAliq
      Regra   Não é permitido informar alíquota quando o prestador é optante do simples nacional do tipo MEI (opSimpNac = 2).
      Mais    https://edusouza.github.io/nfse-emissor-go/referencia/rejeicoes/#e0600
```

- **As tabelas são geradas e commitadas**, como a `lista.csv` de serviços
  ([ADR 0009](0009-lista-de-servicos-embutida.md)).
  `internal/domain/rejeicao/gerar.go` escreve `anexo_i.go` e `anexo_ii.go`, e
  um teste lê os anexos e compara com o que está commitado. Ler as planilhas em
  tempo de execução obrigaria o binário a carregar os `.xlsx` e um leitor
  deles. Gerar código não obriga a nada disso.
- **As tabelas ficam separadas.** Dois códigos aparecem nos dois anexos
  (`E1260` e `E1278`). Em cada anexo, é a mesma regra aplicada a um documento
  diferente, com outro campo. Só quem chama sabe qual documento foi recusado:
  o `emitir` e o `enviar` consultam o ANEXO I, e o `cancelar` consulta o
  ANEXO II.
- **O leitor dos anexos é um só.** Ele sai de `internal/sitegen` para
  `internal/anexos` e passa a servir ao site e ao gerador das tabelas. As
  planilhas de regras dos dois anexos têm as mesmas colunas, e o leitor confere
  os cabeçalhos de ambas. O site e o terminal não têm como discordar sobre o
  que um código significa. A página da DPS gerada continua idêntica, byte a
  byte, e o site ganha a página dos códigos de eventos.
- **O caminho é mostrado como está no que o usuário enviou.** O ANEXO I
  enraíza tudo em `NFSe/infNFSe/`, mas a DPS começa em `DPS/`. O ANEXO II
  enraíza em `evento/`, mas o pedido assinado começa em `pedRegEvento/`. Um
  campo que a Sefin preenche no próprio documento, fora do que foi enviado,
  mantém o caminho inteiro.
- **A regra só aparece quando acrescenta algo.** Em muitos códigos ela repete a
  mensagem palavra por palavra.
- **As regras de nível 3 avisam que dependem do município.** É o caso em que a
  mesma DPS passa num município e falha em outro, e o do prazo para cancelar
  (`E0822`).
- **Um código que o anexo dá a duas regras** (o `E1570`) é resolvido pela frase
  que a Sefin devolveu. Se a frase não bater com nenhuma das duas, as duas são
  mostradas, porque escolher uma esconderia a outra.
- **Um código fora do anexo aparece como a Sefin mandou**, sem nada acrescido.
- **As consultas ao ADN ficam de fora.** Elas respondem com códigos do
  ANEXO IV, que não é um anexo de regras com o mesmo leiaute.

## Consequências

- O binário cresce cerca de 290 KB (1,8%) com os 426 códigos do ANEXO I e os
  49 do ANEXO II.
- Um anexo novo pede três passos: versionar a planilha em `docs/anexos/`,
  apontar `anexos.ArquivoI` ou `anexos.ArquivoII` para ela e rodar
  `go generate ./internal/domain/rejeicao`. O teste de sincronia falha até que
  isso seja feito, e o site muda junto, porque lê o mesmo arquivo.
- O link `Mais` depende do endereço do site. Um teste confere que os dois links
  batem com o `site_url` de `site/mkdocs.yml` e com as páginas que o `sitegen`
  grava.
- O ANEXO II cobre todos os eventos, mas o `nfse` só envia o cancelamento. A
  tabela leva os 49 códigos assim mesmo: filtrar pelo evento exigiria
  interpretar as colunas do anexo, e um código que nunca chega não custa nada.

## Alternativas consideradas

**Embutir os `.xlsx` e ler em tempo de execução.** O leitor de planilha
entraria no binário e cada rejeição pagaria a leitura. As tabelas geradas fazem
o mesmo trabalho uma única vez, na máquina de quem atualiza o anexo.

**Uma tabela só, com os códigos dos dois anexos.** Os códigos repetidos teriam
de escolher um dos significados, e o errado apontaria o campo errado.

**Só o link para o site.** Custaria uma linha, mas exige rede e um navegador
justamente quando a pessoa está tentando corrigir a nota. O campo e a regra
cabem no terminal.
