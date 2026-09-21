# 0010 — Substituição de NFS-e, e por que a DANFSe ficou de fora

**Status:** Aceita
**Data:** 2026-09-21

## Contexto

Duas entregas pretendidas para a mesma versão. Uma foi feita. A outra foi
construída, e então descartada quando o documento certo apareceu.

**A substituição já tinha metade da fundação, desligada.** O `pkg/xmlbuilder`
tinha `DPSSubstitution` e `buildSubstitution()`, espelhando `TCSubstituicao`; a
validação estrutural tinha `validateSubst`. Nada acima disso preenchia o campo,
e o `emitir.go` dizia isso em voz alta:

```go
// Substitution stays nil: this is an ordinary emission, not a replacement.
```

É o mesmo formato dos defeitos da [ADR 0008](0008-digest-sem-namespace.md):
código internamente consistente, testado, e desconectado da única coisa que o
exercitaria.

**A DANFSe parecia ser só um download.** O `GET /DANFSe` da Sefin responde 501 e
aponta para o ADN; o manual do ADN, versionado em `docs/markdown/`, descrevia
`GET /danfse/{chaveAcesso}` devolvendo o PDF gerado pelo governo. Isso tirava do
escopo a biblioteca de PDF, o leiaute e o QR Code.

## Decisão

### Substituição: entregue

`nfse emitir --substitui <chave> --motivo <nome> [--motivo-texto ...]`.

**Não é um comando novo.** Substituição não é evento: o cancelamento é um
`pedRegEvento` com `e101101` num endpoint próprio, enquanto a substituição é uma
**DPS nova** que carrega `subst/chSubstda` apontando para a nota que troca.

**Os nomes dos motivos são deliberadamente diferentes dos do cancelamento**,
porque os conjuntos de códigos são disjuntos — `TSCodJustCanc` é 1, 2, 9;
`TSCodJustSubst` é 01..05, 99.

| `--motivo` | Código |
|---|---|
| `saiu-do-simples` | 01 |
| `entrou-no-simples` | 02 |
| `incluiu-isencao` | 03 |
| `excluiu-isencao` | 04 |
| `recusada-pelo-tomador` | 05 |
| `outros` | 99 |

**A substituição não entra em `config.Nota`**, e portanto não é expressável em
`padroes`. Chega ao `buildDPS` como parâmetro explícito: um
`padroes.substituicao` esquecido substituiria uma nota a cada emissão, em
silêncio.

**A validação passou a conferir tipos, não só presença.** Um código de
cancelamento (`"1"`) é string não vazia e passava.

### DANFSe: revertida

`nfse danfse` foi implementado — cliente do ADN, tratamento de status, testes —
e **removido antes de ser lançado**. A
[NT 008, versão 1.02, de 14 de julho de 2026](../notas-tecnicas/nt-008-se-cgnfse-danfse-20260714-v1-02.pdf)
é explícita:

> Esta nota técnica servirá de base para a geração do DANFSe por meios de
> softwares de emissão de NFS-e, ERPs e sistemas fiscais, motivo pelo qual, a
> **API de geração do DANFSe** (https://adn.nfse.gov.br/danfse/docs/index.html)
> **será sobrestada (suspensa) na data de 03 de agosto de 2026**.

A API está suspensa desde 3 de agosto. O comando não podia funcionar, e publicar
um comando que não funciona é pior que não ter comando. A geração do DANFSe
passa a ser responsabilidade do emissor, e isso é o escopo da
[issue #22](https://github.com/edusouza/nfse-emissor-go/issues/22).

## Como eu errei nesse caminho

Vale registrar, porque o erro foi de método e não de fato.

O contrato da DANFSe foi lido do manual do ADN — documento oficial, versionado
aqui. Isso estava certo. O erro veio depois, quando surgiram indícios contrários
e eu os interpretei a favor da conclusão que já tinha:

1. **A página de documentação do serviço dava 404 e 503.** Tratei como fraca
   demais para concluir alguma coisa. Era, isoladamente. Mas a URL que a NT cita
   como a API suspensa é exatamente aquela.
2. **Um resumo de terceiro afirmou que a NT 008 desligou a API.** Conferi contra
   os manuais do repositório, não achei menção, e escrevi que *"a afirmação não
   se sustenta"*. Os manuais são de outubro de 2025; a NT é de julho de 2026.
   **A ausência num documento mais antigo nunca foi evidência contra um mais
   novo**, e eu usei como se fosse.
3. **O manual do ADN continuava descrevendo a API.** Verdade, e irrelevante: um
   manual não se reescreve sozinho quando uma nota técnica o supera.

O ceticismo estava certo — o resumo vinha sem fonte e com detalhes vagos. O que
faltou foi separar *"não posso confirmar"* de *"não se sustenta"*. A primeira
frase é honesta; a segunda é uma conclusão que os meus dados não davam.

O que resolveu foi o documento primário, entregue por quem tinha acesso à rede.
Nenhuma quantidade de raciocínio sobre documentos velhos substituiu ler a NT.

## Consequências

- A v0.7.0 entrega só a substituição.
- O `subst` deixa de ser código inalcançável.
- A validação estrutural ganhou o primeiro caso em que confere uma enumeração do
  XSD, e não apenas presença e formato.
- **Gerar o DANFSe volta ao escopo, e grande**: leiaute com coordenadas, QR
  Code, canhoto, fontes — as dependências que tinham sido evitadas voltam a ser
  necessárias. A NT 008 está versionada em `docs/notas-tecnicas/` e é a
  especificação.
- O `internal/infrastructure/adn` foi removido inteiro. Se um dia houver API de
  novo, ele está no histórico.

## Aprendizado

Uma fonte oficial pode estar desatualizada, e a data importa tanto quanto o
selo. Os manuais deste repositório são de outubro de 2025; os schemas, de
dezembro e fevereiro; a NT, de julho. Confrontar código com documento — o método
que achou os defeitos das ADRs 0003, 0005, 0006 e 0009 — só funciona se o
documento for o vigente.

Falta a este repositório uma forma de saber quando um artefato versionado
envelheceu. Hoje ele guarda a cópia, e não a data em que ela deixou de valer.
