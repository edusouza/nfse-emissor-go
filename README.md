# nfse

Emissor de NFS-e (Nota Fiscal de Serviço eletrônica) em linha de comando, para o
**Sistema Nacional NFS-e**.

Voltado a prestadores de serviço do Simples Nacional — MEI, ME e EPP — que
querem emitir as próprias notas a partir do terminal ou de um script, sem
depender de portal web.

> **Estado atual — v0.6.0** ([CHANGELOG](CHANGELOG.md)). O CLI monta, valida,
> assina e envia a DPS à Sefin Nacional, e agora se configura sozinho a partir
> do certificado (`nfse onboard`). A conexão com o ambiente real já foi
> exercitada — foi assim que apareceu o defeito de renegociação TLS corrigido
> na v0.5.0. O que ainda falta confirmar é uma emissão completa, do `POST` até
> a NFS-e autorizada; é por isso que a numeração segue em `0.x`. Veja o
> [roadmap](#roadmap).

## Instalação

```bash
go install github.com/edusouza/nfse-emissor-go/cmd/nfse@latest
```

Para fixar a versão, troque `@latest` por `@v0.6.0`. O `nfse versao` mostra o
que está instalado — e é o mesmo identificador que vai no `verAplic` de cada
declaração.

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

| Comando | O que faz |
|---------|-----------|
| `nfse onboard` | cria o `nfse.yaml` já preenchido, a partir do certificado |
| `nfse config init` / `check` | cria um `nfse.yaml` em branco e confere o que está preenchido |
| `nfse cert info` | inspeciona o certificado A1 |
| `nfse emitir` | monta, valida, assina e — com `--enviar` — transmite |
| `nfse enviar <arquivo.xml>` | transmite uma DPS que já foi gerada e assinada |
| `nfse consultar <chave>` | busca a NFS-e, ou a chave a partir do identificador da DPS |
| `nfse cancelar <chave>` | registra o evento de cancelamento |
| `nfse numero ver` / `definir` | consulta e ajusta o contador da série |

Todos aceitam `--help`.

### Configurar em um comando

O `nfse onboard` monta a configuração a partir do que já se sabe sobre você. O
CNPJ sai do próprio certificado — o ICP-Brasil grava o titular como
`RAZÃO SOCIAL:CNPJ` no A1 — e o resto vem do cadastro público da Receita
Federal:

```bash
export NFSE_CERT_SENHA='sua-senha'
nfse onboard --certificado certificado.pfx
```

```
Certificado  certificado.pfx
  Titular     EMPRESA EXEMPLO LTDA:12345678000195
  Valido ate  10/03/2027

Consultando o CNPJ 12.345.678/0001-95 no cadastro publico da Receita Federal, via brasilapi.com.br...
  Razao social  EMPRESA EXEMPLO LTDA
  Municipio     CURITIBA/PR (IBGE 4106902)
  Regime        mei
  Situacao      ATIVA

nfse.yaml criado.

Falta preencher em nfse.yaml:
  - padroes.servico.codigo_tributacao_nacional — 6 digitos da lista nacional (LC 116/2003)
  - padroes.servico.descricao — o que voce presta

Depois:
  nfse config check
  nfse emitir --valor 100,00
```

O código IBGE do município — sete dígitos que ninguém sabe de cabeça — e a
razão social exata vêm prontos. Sobra o código do serviço, que depende do que
você presta ([#10](https://github.com/edusouza/nfse-emissor-go/issues/10)).

Sem o certificado em mãos, `--cnpj 12345678000195` faz o mesmo caminho. E a
consulta é opcional:

```bash
nfse onboard --certificado certificado.pfx --sem-rede
```

A consulta manda **só o seu CNPJ** para um serviço de terceiros
([BrasilAPI](https://brasilapi.com.br), que serve os dados abertos da Receita) —
nada do certificado sai da máquina. O comando avisa antes de sair para a rede,
`--sem-rede` desliga a consulta, e `--fonte` aponta para outro servidor, para
quem roda a própria instância do
[minhareceita](https://docs.minhareceita.org). Se a consulta falhar, o arquivo
é gravado assim mesmo com o que o certificado informou. Por quê:
[ADR 0007](docs/decisoes/0007-preenchimento-da-configuracao.md).

Quem prefere preencher tudo à mão continua com `nfse config init`, que escreve
o mesmo arquivo em branco e comentado.

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

Com o `nfse.yaml` no lugar (veja [Configurar em um comando](#configurar-em-um-comando)),
confira o que ficou faltando:

```bash
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

### Alíquota do ISS e retenção

Quem pode declarar alíquota de ISS na nota depende do regime do prestador, e a
Sefin rejeita a DPS quando a combinação está errada. O `emitir` verifica isso
antes de assinar, então a recusa chega no terminal em vez de voltar como
rejeição:

| Situação | Alíquota |
|---|---|
| MEI | não pode informar (E0600) — o ISS sai no DAS |
| ME/EPP no Simples, sem retenção | não pode informar (E0625) |
| ME/EPP no Simples, com retenção | **precisa** informar, no mínimo 1,8% (E0621) |
| Qualquer um | nunca acima de 5% (E0595) |

A retenção vem de `--retencao`, ou de `padroes.valores.retencao_issqn` no
`nfse.yaml`:

```bash
nfse emitir --numero 45 --valor 3000 --descricao "Consultoria"   --tomador-cnpj 98765432000198 --tomador-nome "CLIENTE EXEMPLO SA"   --retencao tomador --iss-aliquota 2.5
```

Os valores são `nao` (padrão), `tomador` e `intermediario`.

Um ME/EPP que apura o ISSQN fora do Simples declara isso em
`prestador.regime_apuracao` (`sn`, `iss-municipio` ou `fora-do-sn`). Nesses dois
últimos casos a regra depende do convênio do município com o Sistema Nacional,
que não dá para saber sem consultar a Sefin, então o comando não opina — veja
[docs/convenio-municipal.md](docs/convenio-municipal.md).

### Enviar para a Sefin Nacional

```bash
nfse emitir --numero 42 --valor 1500 --descricao "Consultoria" --enviar
```

```
NFS-e emitida
  Chave de acesso  41069022212345678000195000000000000126081234567890
  DPS              DPS410690211234567800019500001000000000000042
  Ambiente         producao-restrita
  Valor            R$ 1500.00
  Processada em    18/09/2026 09:57:36
  DPS assinada     notas/DPS4106902...042-dps.xml
  NFS-e            notas/4106902...67890-nfse.xml

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

### Conferir agora, enviar depois

`nfse emitir` sem `--enviar` para na assinatura e grava a DPS. Para transmitir
**aquele mesmo arquivo** depois, use `nfse enviar`:

```bash
nfse emitir --valor 1500 --descricao "Consultoria - agosto/2026"
# confira notas/DPS4106902...-dps.xml
nfse enviar notas/DPS4106902...-dps.xml
```

Chamar `nfse emitir --enviar` de novo **não** manda o arquivo anterior: monta um
documento novo, com o próximo número da série e outro instante de emissão. A
nota que você conferiu ficaria para trás, e o número já gasto viraria um buraco
na sequência.

`nfse enviar` não assina nada e não mexe no contador — a numeração pertence à
emissão. Um arquivo sem assinatura é recusado antes de sair da máquina, porque
é o engano provável: `--sem-assinar` grava com nome parecido.

### A senha do certificado

Há três formas de informá-la, nesta ordem de precedência:

1. `--senha` — direto na linha de comando;
2. `NFSE_CERT_SENHA` — variável de ambiente;
3. prompt interativo, quando nenhuma das anteriores é usada e há um terminal.

**Prefira a variável de ambiente ou o prompt.** Argumentos de linha de comando
ficam visíveis para qualquer processo que consiga ler a lista de processos do
sistema, e costumam ficar gravados no histórico do shell.

### Consultar uma nota emitida

```bash
nfse consultar 41069022212345678000195000000000000126081234567890
```

```
NFS-e encontrada
  Chave de acesso  41069022212345678000195000000000000126081234567890
  Ambiente         producao-restrita
  Arquivo          notas/4106902...67890-nfse.xml
```

Se uma emissão foi interrompida e você não sabe se a nota saiu, consulte pelo
identificador da declaração:

```bash
nfse consultar --dps DPS410690211234567800019500001000000000000042 --existe
```

`--existe` responde apenas sim ou não — o governo atende essa pergunta a
qualquer certificado válido. Sem a flag, ele devolve a chave de acesso, que por
sigilo fiscal só é informada a quem consta na nota (prestador, tomador ou
intermediário).

#### A chave de acesso

São **50 dígitos**, sem prefixo, compostos assim:

| Posições | Campo | Exemplo |
|---|---|---|
| 1–7 | Código IBGE do município | `4106902` |
| 8 | Ambiente gerador (1 = sistema próprio, 2 = Sefin Nacional) | `2` |
| 9 | Tipo de inscrição (1 = CPF, 2 = CNPJ) | `2` |
| 10–23 | Inscrição federal (CPF preenchido com zeros à esquerda) | `12345678000195` |
| 24–36 | Número da NFS-e | `0000000000001` |
| 37–40 | Ano e mês da emissão | `2608` |
| 41–49 | Código numérico aleatório | `123456789` |
| 50 | Dígito verificador | `0` |

O guia do emissor público diz "44 dígitos", mas o próprio exemplo que ele
apresenta tem 50, e a soma dos campos acima também. O XSD é a fonte: `TSChaveNFSe`
restringe o tipo a `[0-9]{50}`.

### A numeração

`--numero` é opcional: sem ele, o `nfse` usa o próximo número da série. O último
número usado fica em `.nfse-estado.json`, ao lado do `nfse.yaml`.

```console
$ nfse numero ver
Ultimo numero usado por serie (registro local):

  00001  ultimo 2, proximo 3   18/09/2026 13:52  <- serie configurada
```

Quem migra de outro emissor precisa retomar a numeração existente:

```bash
nfse numero definir 500     # a próxima será a 501
```

**O arquivo é uma conveniência local, não a verdade.** Quem decide o que foi
realmente emitido é a Sefin. Emitir da mesma configuração em duas máquinas
dessincroniza a contagem — nesse caso confira na Sefin e realinhe com
`nfse numero definir`.

Informar `--numero` explicitamente continua funcionando e **nunca puxa o
contador para trás**: preencher uma lacuna com um número antigo não faz a
próxima emissão automática colidir com uma nota já emitida.

### Cancelar uma nota

```bash
nfse cancelar 41069022212345678000195000000000000126081234567890 \
  --motivo erro-emissao \
  --justificativa "Valor do servico lancado incorretamente na nota"
```

```
Cancelamento registrado
  NFS-e            41069022212345678000195000000000000126081234567890
  Pedido           PRE41069022212345678000195000000000000126081234567890101101
  Ambiente         producao-restrita
  Evento           notas/PRE4106902...101101-evento.xml
```

O cancelamento é um documento à parte — um pedido de registro de evento,
assinado com o mesmo certificado. Os motivos aceitos são `erro-emissao`,
`nao-prestado` e `outros`.

A justificativa entra no registro fiscal e o schema exige **entre 15 e 255
caracteres**. Não é capricho da ferramenta: `TSMotivo` impõe o mínimo, que na
prática obriga a explicar o que aconteceu em vez de escrever "erro".

Cancelar em `producao` pede confirmação no terminal — a operação é definitiva.

## Roadmap

| Versão | Entrega | Estado |
|--------|---------|--------|
| v0.1.0 | Pipeline offline: montar + validar + assinar a DPS | pronto |
| v0.2.0 | Envio à Sefin Nacional | pronto |
| v0.3.0 | Consulta de NFS-e por chave de acesso | pronto |
| v0.4.0 | Cancelamento de NFS-e | pronto |
| v0.5.0 | `nfse enviar`, validação da alíquota de ISS, renegociação TLS | lançada |
| v0.5.1 | Recusar certificado que não é do prestador, antes de assinar | lançada |
| v0.6.0 | `nfse onboard`: configuração preenchida a partir do certificado | **lançada** |
| v0.7.0 | Busca do código do serviço ([#10](https://github.com/edusouza/nfse-emissor-go/issues/10)) | planejado |
| v0.8.0 | Substituição de NFS-e | planejado |
| v1.0.0 | Depois da primeira emissão real confirmada em produção | planejado |

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
    brasilapi/     consulta do cadastro publico de CNPJ (so no `onboard`)
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
Sefin Nacional. Ela confere estrutura, tipos, formatos, regras de valores
monetários e as regras de alíquota do ISS que dependem só do regime do
prestador (E0595, E0600, E0621, E0625). Não faz validação XSD completa nem
conhece as parametrizações municipais — as regras que dependem do convênio do
município (E0635, E0640) ficam de fora de propósito. A palavra final é sempre
do governo.

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
