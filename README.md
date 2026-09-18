# nfse

Emissor de NFS-e (Nota Fiscal de Serviço eletrônica) em linha de comando, para o
**Sistema Nacional NFS-e**.

Voltado a prestadores de serviço do Simples Nacional — MEI, ME e EPP — que
querem emitir as próprias notas a partir do terminal ou de um script, sem
depender de portal web.

> **Estado atual:** em desenvolvimento. O envio à Sefin Nacional ainda **não**
> está disponível — veja o [roadmap](#roadmap). O que já funciona está descrito
> abaixo.

## Instalação

```bash
go install github.com/edusouza/nfse-emissor-go/cmd/nfse@latest
```

Ou compilando a partir do código:

```bash
git clone https://github.com/edusouza/nfse-emissor-go.git
cd nfse-emissor-go
go build -o nfse ./cmd/nfse
```

Requer Go 1.25 ou superior. O resultado é um binário único, sem `cgo` e sem
dependência de serviço externo.

## Uso

### Verificar o certificado

Antes de emitir qualquer coisa, confirme que seu certificado A1 está legível e
dentro da validade:

```bash
export NFSE_CERT_SENHA='sua-senha'
nfse cert info --arquivo certificado.pfx
```

```
Titular                 EMPRESA EXEMPLO LTDA:12345678000199
Emissor                 AC CERTISIGN RFB G5
Numero de serie         4A3B2C1D...
Valido de               10/03/2026 09:14
Valido ate              10/03/2027 09:14
Dias restantes          173
Tamanho da chave        2048 bits
Certificados na cadeia  2

Certificado apto a assinar uma DPS.
```

O comando sai com código diferente de zero se o certificado não puder assinar
uma DPS — útil para usar em script.

### A senha do certificado

Há três formas de informá-la, nesta ordem de precedência:

1. `--senha` — direto na linha de comando;
2. `NFSE_CERT_SENHA` — variável de ambiente;
3. prompt interativo, quando nenhuma das anteriores é usada e há um terminal.

**Prefira a variável de ambiente ou o prompt.** Argumentos de linha de comando
ficam visíveis para qualquer processo que consiga ler a lista de processos do
sistema, e costumam ficar gravados no histórico do shell.

## Roadmap

| Versão | Entrega | Estado |
|--------|---------|--------|
| v0.1.0 | Pipeline offline: montar + validar + assinar a DPS | em andamento |
| v0.2.0 | Envio à Sefin Nacional | planejado |
| v0.3.0 | Consulta de NFS-e por chave de acesso | planejado |
| v0.4.0 | Cancelamento e substituição | planejado |

Detalhes na [issue #6](https://github.com/edusouza/nfse-emissor-go/issues/6).

## Como o projeto está organizado

```
cmd/nfse/          binário do CLI
internal/
  cli/             comandos e apresentação
  domain/          regras de negócio (cálculo de valores, validações, rejeições)
  infrastructure/
    xmlsigner/     assinatura XMLDSig e leitura do certificado A1
    sefin/         cliente da API do governo
pkg/
  xmlbuilder/      montagem do XML da DPS
  cnpjcpf/         validação de CNPJ e CPF
  dpsid/           identificador da DPS (42 caracteres)
docs/
  decisoes/        registro de decisões de arquitetura (ADRs)
  markdown/        manuais oficiais convertidos para markdown
  schemas/         XSDs oficiais
```

## Desenvolvimento

```bash
go test ./...          # suíte completa (~75s: inclui testes de backoff reais)
go test -short ./...   # rápida (~2s), pulando os testes dependentes de relógio
go vet ./...
gofmt -l ./cmd ./internal ./pkg
```

## O que a validação local cobre

O CLI valida a DPS antes de assinar, mas essa validação **não substitui** a da
Sefin Nacional. Ela confere estrutura, tipos, formatos e regras de valores
monetários; não faz validação XSD completa nem conhece as parametrizações
municipais. A palavra final é sempre do governo.

Veja as issues [#4](https://github.com/edusouza/nfse-emissor-go/issues/4) e
[#5](https://github.com/edusouza/nfse-emissor-go/issues/5).

## Glossário

| Termo | Significado |
|-------|-------------|
| **DPS** | Declaração de Prestação de Serviço — o XML que você envia |
| **NFS-e** | Nota Fiscal de Serviço eletrônica — o XML que o governo devolve |
| **chaveAcesso** | Identificador de 50 caracteres da NFS-e emitida |
| **cTribNac** | Código nacional do serviço, 6 dígitos (LC 116/2003) |
| **A1** | Certificado digital em arquivo (`.pfx`/`.p12`), válido por 1 ano |

## Licença

[MIT](LICENSE).
