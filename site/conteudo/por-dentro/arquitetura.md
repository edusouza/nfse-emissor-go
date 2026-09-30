# Como o projeto está organizado

O `nfse` é um binário único em Go, sem `cgo` e sem servidor, fila ou banco de
dados. Já foi uma API REST com worker, MongoDB e Redis; a
[ADR 0001](../decisoes/0001-cli-em-vez-de-api.md) conta por que deixou de ser.

## O caminho de uma nota

1. **Entrada.** `internal/cli` lê as flags; `internal/config` lê o
   `nfse.yaml` e o arquivo da nota passado em `--yaml`.
2. **Montagem.** `pkg/xmlbuilder` monta o XML da DPS, e `pkg/dpsid` o
   identificador de 42 caracteres.
3. **Validação.** `internal/domain/esquema` confere a DPS contra os XSDs
   oficiais, e `internal/domain/validation` as regras que o XSD não expressa.
4. **Assinatura.** `internal/infrastructure/xmlsigner` assina com o A1, em
   XMLDSig com canonicalização exc-c14n.
5. **Envio.** `internal/infrastructure/sefin` transmite à Sefin Nacional e lê
   a NFS-e autorizada, ou a rejeição.
6. **Documento auxiliar.** `internal/domain/danfse` monta o DANFSe a partir do
   XML da nota, e `internal/infrastructure/danfsepdf` o desenha em PDF.

`internal/domain` não faz I/O: rede, disco e apresentação ficam em
`internal/infrastructure` e `internal/cli`.

## Os diretórios

```
cmd/nfse/              binário do CLI
internal/
  cli/                 comandos, apresentação e leitura de entrada
  config/              o nfse.yaml e os dados de cada nota
  domain/              regras de negócio, sem I/O
    emission/          cálculo de valores
    validation/        regras de negócio da DPS que o XSD não expressa
    esquema/           validação contra os XSDs oficiais (v1.01, embutidos)
    servico/           a lista nacional de serviços, embutida
    query/             chave de acesso e respostas de consulta
    danfse/            modelo do documento auxiliar, montado do XML da NFS-e
    texto/             normalização de texto das buscas (acentos, caixa)
  infrastructure/
    xmlsigner/         XMLDSig, canonicalização exc-c14n, certificado A1
    sefin/             cliente HTTP da API do governo
    brasilapi/         consulta do cadastro público de CNPJ (só no onboard)
    ibge/              municípios do IBGE, com cache
    danfsepdf/         desenho do DANFSe em PDF, nas coordenadas da NT 008
  anexoa/              leitor do ANEXO_A oficial — só para testes
  docs/                testes que conferem a documentação contra o código
  sitegen/             gera as páginas deste site que vêm do repositório
pkg/
  xmlbuilder/          montagem do XML da DPS
  cnpjcpf/             validação de CNPJ e CPF
  dpsid/               identificador da DPS (42 caracteres)
  codmun/              código de município do IBGE (UF e dígito verificador)
docs/
  decisoes/            registro de decisões de arquitetura (ADRs)
  api/                 especificações OpenAPI oficiais do governo
  schemas/             XSDs oficiais
  anexos/              planilhas oficiais (municípios, serviços, regras)
  notas-tecnicas/      notas técnicas, que superam os manuais
  markdown/            manuais oficiais convertidos para markdown
exemplos/              passo a passo executável
site/                  este site
```

## Dependências

Poucas, de propósito: toda dependência num binário que lida com certificado
digital é superfície de risco. O comando usa
[cobra](https://github.com/spf13/cobra); o XML é manipulado com
[etree](https://github.com/beevik/etree); o certificado A1 é lido com
[go-pkcs12](https://software.sslmate.com/src/go-pkcs12). As planilhas do
governo são lidas com a biblioteca padrão, sem biblioteca de planilha.
