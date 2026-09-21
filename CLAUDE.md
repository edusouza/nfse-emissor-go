# CLAUDE.md

Orientações para o Claude Code (claude.ai/code) ao trabalhar neste repositório.

## Visão geral

`nfse` é um emissor de NFS-e em **linha de comando** para o Sistema Nacional
NFS-e do Brasil, voltado a prestadores do Simples Nacional (MEI/ME/EPP).

O projeto já foi uma API REST com worker, MongoDB e Redis. Isso foi removido —
veja [ADR 0001](docs/decisoes/0001-cli-em-vez-de-api.md). Não reintroduza
servidor, fila ou banco de dados sem uma decisão registrada.

## Stack

- Go 1.26+, binário único, **sem `cgo`** (preserva o cross-compile)
- [cobra](https://github.com/spf13/cobra) para o comando
- [etree](https://github.com/beevik/etree) para manipular XML
- [go-pkcs12](https://software.sslmate.com/src/go-pkcs12) para ler o certificado A1

Mantenha as dependências no mínimo. Toda dependência nova em um binário que
lida com certificado digital é superfície de risco.

## Comandos

```bash
go build -o nfse ./cmd/nfse   # compilar
go test -short ./...          # testes rápidos (~2s) — use durante o desenvolvimento
go test ./...                 # suíte completa (~75s, inclui testes de backoff)
go test -race ./...           # o que a CI roda
go vet ./...
gofmt -l ./cmd ./internal ./pkg
```

## Arquitetura

```
cmd/nfse/                  entrypoint
internal/
  cli/                     comandos cobra, apresentação e leitura de entrada
  domain/                  regras de negócio, sem I/O
    emission/              cálculo de valores, tradução de rejeições
    validation/            validação da DPS
    query/                 chave de acesso e respostas de consulta
    servico/               lista nacional de serviços (cTribNac), embutida
    municipio/             tabela do IBGE, embutida
    texto/                 dobra de acentos compartilhada pelas buscas
  infrastructure/
    xmlsigner/             XMLDSig, canonicalização exc-c14n, certificado A1
    sefin/                 cliente HTTP da API do governo
    brasilapi/             consulta do cadastro público de CNPJ (só no `onboard`)
pkg/                       utilidades reutilizáveis fora do projeto
  xmlbuilder/              montagem do XML da DPS
  cnpjcpf/                 validação de CNPJ/CPF
  dpsid/                   identificador da DPS (42 caracteres)
```

**Fluxo de uma emissão:** dados do usuário → montar XML da DPS → validar →
assinar com o A1 → enviar à Sefin Nacional → receber a NFS-e autorizada.

`internal/domain` não faz I/O. Rede, disco e apresentação ficam em
`internal/infrastructure` e `internal/cli`.

## Convenções

- **Idioma:** documentação, mensagens do CLI e issues em **pt-BR**; código,
  comentários e mensagens de commit em **inglês**.
- Comentários explicam *por quê*, não *o quê*. Não narre o óbvio.
- Erros voltam com contexto (`fmt.Errorf("...: %w", err)`) e a mensagem final
  ao usuário precisa dizer o que fazer a respeito.
- Testes dependentes de relógio vão atrás de `testing.Short()`.
- Toda decisão que muda o rumo do projeto vira um ADR em `docs/decisoes/`.
- O CHANGELOG é atualizado na mesma mudança que altera o comportamento.

## Cuidados com segurança

- Nunca registre em log a senha do certificado, a chave privada ou o conteúdo
  do PFX.
- Senha de certificado por argumento de linha de comando é visível na lista de
  processos. Prefira `NFSE_CERT_SENHA` ou o prompt interativo.
- O `.gitignore` bloqueia `*.pfx`, `*.p12`, `*.pem` e `*.key`. Não force a
  adição desses arquivos — gere fixtures em memória nos testes.

## Documentação de referência

```
docs/decisoes/    ADRs — leia antes de mudar arquitetura
docs/markdown/    manuais oficiais do governo convertidos
docs/schemas/     XSDs oficiais (DPS_v1.00.xsd, NFSe_v1.00.xsd, evento_v1.00.xsd)
docs/anexos/      planilhas de referência (códigos IBGE, lista de serviços)
specs/            especificações Speckit do desenho anterior (API REST)
```

Namespace dos XMLs: `http://www.sped.fazenda.gov.br/nfse`

## Glossário

- **DPS** — Declaração de Prestação de Serviço; o XML de entrada
- **NFS-e** — Nota Fiscal de Serviço eletrônica; o XML de saída
- **chaveAcesso** — identificador de 50 caracteres da NFS-e
- **cTribNac** — código nacional do serviço, 6 dígitos (LC 116/2003)
- **A1** — certificado digital em arquivo `.pfx`/`.p12`
