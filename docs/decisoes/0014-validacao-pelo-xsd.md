# 0014 — Validar a DPS lendo os XSDs oficiais, com um validador próprio

**Status:** Aceita
**Data:** 2026-09-29

## Contexto

A [issue #4](https://github.com/edusouza/nfse-emissor-go/issues/4) apontou que o
validador "XSD" do emissor não lia XSD nenhum: eram 718 linhas de regras
escritas à mão, que conferiam um subconjunto do schema e divergiam dele. A
[ADR 0003](0003-xml-conforme-o-xsd.md) já tinha encontrado a pior forma disso:
elementos procurados no caminho errado, grupos obrigatórios tratados como
opcionais e, por último, fixtures de teste escritas com os mesmos erros. Com
isso, a suíte passava e a Sefin rejeitava.

A issue listou três saídas: libxml2 via `cgo`, um validador em Go gerado a
partir do XSD, ou manter o validador manual com honestidade. A terceira foi
feita em #7. A primeira quebra o binário único e o cross-compile
([ADR 0001](0001-cli-em-vez-de-api.md)).

## Decisão

**Um validador em Go puro que lê os XSDs oficiais**, embutidos no binário, e
confere a DPS contra o que eles dizem.

Ele não é um processador XSD genérico. Implementa o subconjunto que os schemas
da NFS-e e o XML-DSig importado por eles usam de fato: `sequence`, `choice`,
`any`, `minOccurs`/`maxOccurs`, atributos, tipos simples e complexos,
`simpleContent` com `extension`, e as facetas `enumeration`, `pattern`,
`minLength`, `maxLength`, `length` e `whiteSpace`. **Qualquer outra construção
faz o carregamento falhar.** Uma versão futura do schema que traga algo novo
quebra os testes, em vez de ter uma regra ignorada em silêncio.

Os schemas são os do **pacote v1.01** (09/02/2026), que a Sefin usa. O
`TVerNFSe` desse pacote aceita `1.00|1.01`, e a DPS que o emissor gera
(`versao="1.00"`, sem o grupo IBS/CBS) passa nele. As cópias embutidas são
idênticas byte a byte às de `docs/schemas`, e um teste garante isso.

O modelo de conteúdo é casado por **conjunto de posições**, não de forma
gulosa. Um elemento opcional seguido de outro com o mesmo nome, ou uma escolha
cujos ramos começam igual, é decidido pelo que vem depois.

Uma **interpretação** foi necessária. Em XSD, `^` e `$` são caracteres comuns,
e o `TSSerieDPS` é escrito `^0{0,4}\d{1,5}$`. Lido ao pé da letra, nenhuma série
seria válida. A Sefin trata os dois como âncoras (uma DPS com série `00001` foi
autorizada em 18/09/2026), e o validador faz o mesmo quando o padrão vem
envolvido nos dois.

## Consequências

- **O validador achou dois defeitos no gerador no primeiro dia**, os dois
  invisíveis para a suíte anterior:
  - **deduções:** `vDedRed` é uma escolha entre `pDR`, `vDR` e documentos, e o
    gerador escrevia `pDR` e `vDR` juntos. Toda DPS emitida com `--deducoes`
    seria recusada. O teste do gerador exigia os dois elementos, ou seja,
    prendia o código ao defeito;
  - **endereço do tomador:** `TCEndereco` começa por `endNac` (`cMun`, `CEP`)
    ou `endExt`, e o gerador escrevia `cMun`, `UF`, `CEP` e `cPais` soltos
    depois da rua. Nenhum caminho do CLI monta um endereço hoje, por isso
    ninguém tinha visto.
- **A fixture "válida" do validador antigo falhava no schema real de três
  jeitos**: sem o atributo `versao`, com um `Id` fora do `TSIdDPS` e sem
  `regTrib`. É a divergência que a #4 descrevia, até nos dados de teste. O
  validador antigo foi removido.
- **O binário cresce ~270 KB (1,8%)**, ~200 KB dos quais são os quatro XSDs. É o custo de a regra estar
  no arquivo oficial, e não numa cópia escrita à mão.
- **Carregar o schema leva ~7 ms**, uma vez por processo. Validar uma DPS leva
  ~0,1 ms.
- **As mensagens citam o caminho e o tipo do schema** (`TSCNPJ`, `TSString`).
  Para os dois jeitos mais comuns de um texto livre quebrar o `TSString` (um
  caractere fora do Latin-1, como o travessão que editores inserem sozinhos, e
  um espaço no começo ou no fim), a mensagem explica em português o que o
  padrão diz.
- **O que o schema não expressa continua em `internal/domain/validation`:**
  alíquota do ISS por regime, valores monetários e dígitos verificadores.

## Alternativas consideradas

**libxml2 via `cgo`.** Validação completa e de referência, mas quebra o binário
único e o cross-compile. Recusada pela ADR 0001.

**Gerar código Go a partir do XSD** (`go generate`), com as regras compiladas
em tabelas. Não carregaria XSD em tempo de execução, mas exigiria um gerador
com o mesmo tamanho deste validador e ainda um passo de geração a cada versão
do schema. Interpretar os arquivos embutidos deixa a atualização em "copiar
os XSDs novos".

**Manter as regras manuais.** Foi o que a #7 fez, e é o que produziu as
fixtures erradas.

## Aprendizado

Uma regra copiada à mão do schema tem duas versões, e os testes escritos a
partir da cópia confirmam a cópia, não o schema. A saída não foi escrever
regras melhores. Foi parar de copiá-las.
