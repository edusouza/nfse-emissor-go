# nfse

Emissor de NFS-e (Nota Fiscal de Serviço eletrônica) em linha de comando, para o
**Sistema Nacional NFS-e**.

Voltado a prestadores de serviço do Simples Nacional — MEI, ME e EPP — que
querem emitir as próprias notas a partir do terminal ou de um script, sem
depender de portal web.

> **Estado atual:** o CLI monta, valida, assina e envia a DPS à Sefin Nacional.
> O envio segue a especificação oficial do governo e é coberto por testes, mas
> ainda não foi exercitado contra o ambiente real — veja o [roadmap](#roadmap).

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

Requer Go 1.26 ou superior. O resultado é um binário único, sem `cgo` e sem
dependência de serviço externo.

## Experimentar em 2 minutos

Há um passo a passo completo em [`exemplos/`](exemplos/), com configuração
pronta e um script que gera um certificado descartável — dá para ver o emissor
funcionando sem ter um A1 em mãos. Em duas versões:
[Linux/macOS](exemplos/README.md) e [Windows/PowerShell](exemplos/README-windows.md).

## Uso

### Verificar o certificado

Antes de emitir qualquer coisa, confirme que seu certificado A1 está legível e
dentro da validade:

```bash
export NFSE_CERT_SENHA='sua-senha'
nfse cert info --arquivo certificado.pfx
```

```
Titular                 EMPRESA EXEMPLO LTDA:12345678000195
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

### Emitir uma nota

Crie a configuração e preencha os campos obrigatórios:

```bash
nfse config init     # gera um nfse.yaml comentado
$EDITOR nfse.yaml
nfse config check    # confere se está completo
```

No `nfse.yaml`, a seção `padroes` guarda tudo que se repete — código do serviço,
município, alíquota, e o tomador não identificado quando você vende ao público
em geral. Com ela preenchida, emitir é uma linha:

```bash
nfse emitir --numero 42 --valor 1500 --descricao "Consultoria - agosto/2026"
```

```
DPS DPS410690211234567800019500001000000000000042
  Ambiente       producao-restrita
  Valor          R$ 1500.00
  Servico        Consultoria - agosto/2026
  Assinatura     aplicada
  Arquivo        notas/DPS4106902...042-dps.xml
```

Para identificar o cliente, use as flags do tomador:

```bash
nfse emitir --numero 43 --valor 2400 --descricao "Manutencao mensal"   --tomador-cnpj 98765432000198 --tomador-nome "CLIENTE EXEMPLO SA"
```

Para notas com muitos campos, ou para versionar a nota junto do projeto, use um
arquivo:

```yaml
# nota.yaml
numero: "44"
competencia: "2026-08-01"
servico:
  descricao: Desenvolvimento de API de pagamentos
valores:
  valor_servico: 8500.00
  desconto_incondicionado: 500.00
tomador:
  cnpj: "98765432000198"
  nome: CLIENTE EXEMPLO SA
  email: financeiro@exemplo.com.br
```

```bash
nfse emitir --yaml nota.yaml
```

As três camadas se combinam nesta ordem — `padroes` do `nfse.yaml`, depois o
arquivo de `--yaml`, depois as flags. Cada uma sobrescreve a anterior, então
`--valor` na linha de comando vence o que estiver no arquivo.

`--sem-assinar` gera o XML sem assinatura, para inspecionar antes de gastar o
certificado.

### Enviar para a Sefin Nacional

```bash
nfse emitir --numero 42 --valor 1500 --descricao "Consultoria" --enviar
```

```
NFS-e emitida
  Chave de acesso  41069021123456780001950000100000000000004212345
  DPS              DPS410690211234567800019500001000000000000042
  Ambiente         producao-restrita
  Valor            R$ 1500.00
  Processada em    18/09/2026 09:57:36
  DPS assinada     notas/DPS4106902...042-dps.xml
  NFS-e            notas/4106902...4212345-nfse.xml

Ambiente de producao restrita: esta nota NAO tem valor fiscal.
```

A emissão é **síncrona**: o governo valida e devolve a nota autorizada ou a
rejeição na mesma requisição. Rejeições vêm com todos os motivos de uma vez:

```
erro: documento rejeitado pela Sefin Nacional
  - [E001] Municipio nao conveniado ao Sistema Nacional
  - [E042] cTribNac invalido (010101)
```

O ambiente vem do `nfse.yaml`. Com `ambiente: producao` a nota tem **valor
fiscal** e o comando pede confirmação no terminal antes de enviar — cancelar uma
nota emitida exige um pedido de evento. Use `--confirmar` para dispensar a
pergunta em scripts.

O envio **não é repetido automaticamente** em caso de falha de rede. Emissão não
é idempotente: uma requisição que chegou ao governo e falhou na volta geraria uma
segunda nota numa retentativa. Se acontecer, consulte pelo identificador da DPS
antes de tentar de novo.

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
| v0.1.0 | Pipeline offline: montar + validar + assinar a DPS | pronto |
| v0.2.0 | Envio à Sefin Nacional | pronto |
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
exemplos/          passo a passo executável
docs/
  api/             especificações OpenAPI oficiais do governo
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
