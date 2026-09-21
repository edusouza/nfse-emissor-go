# 0010 — Substituição de NFS-e e DANFSe

**Status:** Aceita
**Data:** 2026-09-21

## Contexto

Duas entregas que o roadmap tratava como distantes e que, olhando o código e os
documentos já versionados, estavam mais perto do que pareciam — e por motivos
opostos.

**A substituição já tinha metade da fundação, desligada.** O `pkg/xmlbuilder`
tinha `DPSSubstitution` e `buildSubstitution()`, espelhando `TCSubstituicao`; a
validação estrutural tinha `validateSubst`. Nada acima disso preenchia o campo,
e o `emitir.go` dizia isso em voz alta:

```go
// Substitution stays nil: this is an ordinary emission, not a replacement.
```

É o mesmo formato dos defeitos da [ADR 0008](0008-digest-sem-namespace.md):
código internamente consistente, testado, e desconectado da única coisa que o
exercitaria. A diferença é que aqui o buraco estava escrito no comentário, e
não escondido.

**A DANFSe não precisava ser desenhada.** O `GET /DANFSe` da Sefin Nacional
responde 501 e o próprio swagger diz para onde foi. O manual do ADN, versionado
em `docs/markdown/`, descreve o serviço:

> 1.5. API DANFSe — Serviço que gera o arquivo PDF da NFS-e a partir de uma
> consulta pela chave de acesso. **a) GET – /danfse/{chaveAcesso}**

Quem gera o PDF é o governo, a partir do XML que já tem. Isso tira do escopo a
biblioteca de PDF, o leiaute oficial e a geração de QR Code — e mantém o
binário único, sem `cgo`, que a [ADR 0001](0001-cli-em-vez-de-api.md) preserva.

## Decisão

### Substituição

`nfse emitir --substitui <chave> --motivo <nome> [--motivo-texto ...]`.

**Não é um comando novo.** Substituição não é evento: o cancelamento é um
`pedRegEvento` com `e101101` num endpoint próprio, enquanto a substituição é
uma **DPS nova** que carrega `subst/chSubstda` apontando para a nota que troca.
Modelá-la como um `nfse substituir` esconderia que ela exige todos os dados de
uma emissão inteira.

**Os nomes dos motivos são deliberadamente diferentes dos do cancelamento.** Os
conjuntos de códigos são disjuntos — `TSCodJustCanc` é 1, 2, 9 e responde "por
que esta nota não vale"; `TSCodJustSubst` é 01..05, 99 e responde "por que
outra nota está tomando o lugar dela". Se o `emitir` aceitasse `erro-emissao`,
alguém mandaria um código que o schema recusa.

| `--motivo` | Código | Significado |
|---|---|---|
| `saiu-do-simples` | 01 | Desenquadramento do Simples Nacional |
| `entrou-no-simples` | 02 | Enquadramento no Simples Nacional |
| `incluiu-isencao` | 03 | Inclusão retroativa de imunidade/isenção |
| `excluiu-isencao` | 04 | Exclusão retroativa de imunidade/isenção |
| `recusada-pelo-tomador` | 05 | Rejeição pelo tomador ou intermediário responsável |
| `outros` | 99 | Outros |

**A substituição não entra em `config.Nota`**, e portanto não é expressável em
`padroes`. Ela chega ao `buildDPS` como parâmetro explícito. Se morasse no
`Nota`, um `padroes.substituicao` esquecido no `nfse.yaml` substituiria uma nota
a cada emissão, em silêncio — exatamente o tipo de erro que este projeto passou
a versão inteira encontrando.

**A validação passou a conferir os tipos, não só a presença.** `validateSubst`
checava que `chSubstda` e `cMotivo` existiam e não eram vazios. Um código de
cancelamento (`"1"`) é uma string não vazia e passava. Agora a chave vai por
`query.ValidateAccessKey` e o motivo pela enumeração.

### DANFSe

`nfse danfse <chave>` baixa o PDF do ADN e grava em `notas/`.

O cliente vive em `internal/infrastructure/adn`, separado do `sefin` porque é
outro host e outro contrato. Reaproveita a renegociação TLS que a v0.5.0
descobriu ser obrigatória.

## O que não foi verificado, e como o código lida com isso

Este é o ponto que a [ADR 0005](0005-contrato-da-sefin-verificado.md) obriga a
declarar. Duas coisas na DANFSe vêm de inferência, não de especificação:

1. **Se o certificado de um prestador é aceito.** A API DANFSe é descrita no
   manual **dos municípios**. O manual dos contribuintes não a menciona, e o
   `adn-contribuinte-swagger.json` que o repositório carrega tem apenas
   `/DFe/{NSU}` e `/NFSe/{ChaveAcesso}/Eventos`.
2. **O host de produção.** `adn.producaorestrita.nfse.gov.br/danfse` está
   escrito no 501 da própria Sefin; `adn.nfse.gov.br/danfse` é a mesma forma que
   a Sefin usa para o par produção/restrita, aplicada por simetria.

O ambiente onde isto foi escrito não alcança `gov.br`, então nada disso pôde ser
conferido. O cliente foi construído para essa incerteza:

- **Nunca grava o que não reconhece.** Um 200 cujo corpo não começa com `%PDF-`
  vira erro com o `Content-Type` e um resumo do corpo, em vez de um `.pdf` que
  leitor nenhum abre.
- **403 e 401 explicam a hipótese mais provável** — o manual dos municípios — em
  vez de um "acesso negado" que manda procurar no lugar errado.
- **501 diz que o endereço mudou** e aponta para `--url`.
- **`--url` existe** para que um endereço errado seja contornável sem esperar uma
  versão nova.

## Consequências

- Nenhuma dependência nova. O PDF vem pronto do governo.
- O `subst` deixa de ser código inalcançável: o que o `xmlbuilder` montava desde
  a v0.1.0 passa a ter caminho de entrada e teste ponta a ponta.
- A validação estrutural ganhou o primeiro caso em que confere uma enumeração do
  XSD, e não apenas presença e formato. Há mais campos assim; este abriu o
  caminho.
- O `nfse danfse` é o primeiro comando que fala com um serviço do governo que
  **não** é a Sefin Nacional. A configuração não ganhou campo para o endereço do
  ADN: `--url` cobre o caso raro, e inventar uma chave de configuração para algo
  não verificado seria formalizar um palpite.

## Alternativas consideradas

**Gerar o DANFSe localmente.** Exigiria o leiaute oficial — que não está no
repositório —, uma biblioteca de PDF e geração de QR Code. Três dependências e
uma superfície de manutenção grande para reproduzir, com risco de divergir, um
documento que o governo entrega pronto.

**`nfse substituir` como comando próprio.** Rejeitado por descrever mal o que
acontece: seria um comando que aceita todas as flags do `emitir` mais duas.

**Esperar a verificação da DANFSe antes de entregar.** O ambiente não alcança
`gov.br` e não vai passar a alcançar. Entregar com as incertezas declaradas e
mensagens de erro que as antecipam custa um comando para descobrir a verdade;
esperar custa a funcionalidade inteira.

## Aprendizado

Duas peças foram encontradas lendo o que já estava no repositório, não
escrevendo código novo: um `TODO` honesto num comentário e uma seção de manual
que ninguém tinha aberto. A ADR 0009 já dizia que a tabela de serviços estava
versionada desde o começo; aqui foram o endpoint da DANFSe e metade da
substituição.

E há uma diferença entre "não verificado" e "inventado" que vale marcar. A ADR
0005 catalogou um cliente que falava SOAP com uma API REST — protocolo
imaginado. A DANFSe aqui tem endpoint e semântica lidos de um documento oficial;
o que falta é saber se o *nosso* certificado entra. Isso não justifica esperar:
justifica escrever o erro que essa hipótese produziria, antes de ela acontecer.
