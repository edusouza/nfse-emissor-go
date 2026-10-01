# 0019 — Rejeições explicadas no terminal, com uma tabela gerada do ANEXO I

**Status:** Aceita
**Data:** 2026-10-01

## Contexto

Quando recusa uma DPS, a Sefin responde com um código e uma frase:

```
erro: documento rejeitado pela Sefin Nacional
  - [E0600] Não é permitido informar a alíquota para prestador de serviço optante do simples nacional do tipo MEI.
```

A frase raramente diz qual campo corrigir e nunca diz qual condição falhou. O
ANEXO I diz as duas coisas: o caminho do campo no XML, o texto da regra e o
nível dela (1, leiaute; 2, regra geral; 3, depende do município). O site já
publica isso numa página gerada do anexo
([ADR 0016](0016-site-de-documentacao.md)). Na hora da rejeição, porém, quem
emite está no terminal.

O catálogo antigo (`internal/domain/emission/errors.go`) tentava fazer esse
papel com 38 códigos inventados e foi removido na
[issue #37](https://github.com/edusouza/nfse-emissor-go/issues/37). A lição
ficou no CLAUDE.md: um mapa de códigos não se escreve à mão.

## Decisão

**O binário carrega uma tabela gerada do ANEXO I** e acrescenta, sob cada
código que a Sefin devolve:

```
  - [E0600] Não é permitido informar a alíquota para prestador de serviço optante do simples nacional do tipo MEI.
      Campo   DPS/infDPS/valores/trib/tribMun/pAliq
      Regra   Não é permitido informar alíquota quando o prestador é optante do simples nacional do tipo MEI (opSimpNac = 2).
      Mais    https://edusouza.github.io/nfse-emissor-go/referencia/rejeicoes/#e0600
```

- **A tabela é gerada e commitada**, como a `lista.csv` de serviços
  ([ADR 0009](0009-lista-de-servicos-embutida.md)).
  `internal/domain/rejeicao/gerar.go` escreve `anexo_i.go`, e um teste lê o
  anexo e compara com o que está commitado. Ler a planilha em tempo de execução
  obrigaria o binário a carregar o `.xlsx` e um leitor dele. Gerar código não
  obriga a nada disso.
- **O leitor do anexo é um só.** Ele sai de `internal/sitegen` para
  `internal/anexoi` e passa a servir ao site e ao gerador da tabela. A página e
  o terminal não têm como discordar sobre o que um código significa. A página
  gerada continua idêntica, byte a byte.
- **O caminho é mostrado como está no arquivo do usuário.** O anexo enraíza
  tudo em `NFSe/infNFSe/`, mas a DPS que o usuário tem começa em `DPS/`. Um
  campo que a Sefin preenche fora da DPS mantém o caminho inteiro.
- **A regra só aparece quando acrescenta algo.** Em muitos códigos ela repete a
  mensagem palavra por palavra.
- **As regras de nível 3 avisam que dependem do município.** É o caso em que a
  mesma DPS passa num município e falha em outro.
- **Um código que o anexo dá a duas regras** (o `E1570`) é resolvido pela frase
  que a Sefin devolveu. Se a frase não bater com nenhuma das duas, as duas são
  mostradas, porque escolher uma esconderia a outra.
- **Um código fora do anexo aparece como a Sefin mandou**, sem nada acrescido.
- **Só a emissão é explicada.** `emitir --enviar` e `enviar` passam por aqui. As
  rejeições de eventos (cancelamento) estão no ANEXO II, e as consultas ao ADN
  respondem com códigos do ANEXO IV. Nenhum dos dois entrou nesta decisão.

## Consequências

- O binário cresce cerca de 250 KB (1,6%) com os 426 códigos.
- Um anexo novo pede três passos: versionar a planilha em `docs/anexos/`,
  apontar `anexoi.Arquivo` para ela e rodar
  `go generate ./internal/domain/rejeicao`. O teste de sincronia falha até que
  isso seja feito, e o site muda junto, porque lê o mesmo `anexoi.Arquivo`.
- O link `Mais` depende do endereço do site. Um teste confere que ele bate com
  o `site_url` de `site/mkdocs.yml` e com a página que o `sitegen` grava.

## Alternativas consideradas

**Embutir o `.xlsx` e ler em tempo de execução.** São 212 KB em vez de 200 KB,
mas o leitor de planilha entraria no binário e cada rejeição pagaria a leitura.
A tabela gerada faz o mesmo trabalho uma única vez, na máquina de quem atualiza
o anexo.

**Só o link para o site.** Custaria uma linha, mas exige rede e um navegador
justamente quando a pessoa está tentando corrigir a nota. O campo e a regra
cabem no terminal.
