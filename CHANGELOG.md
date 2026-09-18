# Changelog

Todas as mudanças notáveis neste projeto serão documentadas neste arquivo.

O formato é baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.0.0/),
e este projeto adere ao [Versionamento Semântico](https://semver.org/lang/pt-BR/).

## [Não lançado]

### Adicionado

- **CLI `nfse`** — binário único, sem `cgo`, instalável com
  `go install github.com/edusouza/nfse-emissor-go/cmd/nfse@latest`.
- `nfse versao` — versão, plataforma e versão do Go.
- `nfse cert info` — inspeciona um certificado A1 (PFX/P12): titular, emissor,
  validade, dias restantes, tamanho da chave e cadeia. Sai com código de erro
  quando o certificado não pode assinar uma DPS, e avisa quando faltam menos de
  30 dias para o vencimento.
- Resolução da senha do certificado por `--senha`, pela variável de ambiente
  `NFSE_CERT_SENHA` ou por prompt interativo, nessa ordem.
- Esteira de CI: formatação, `go mod tidy` limpo, `go vet`, build, testes com
  detector de corrida e relatório de cobertura.
- `nfse config init` — gera um `nfse.yaml` comentado; `nfse config check`
  verifica se está completo e coerente.
- `nfse emitir` — monta, valida e assina o XML da DPS. Os dados vêm da seção
  `padroes` do `nfse.yaml`, de um arquivo passado em `--yaml` e das flags, cada
  camada sobrescrevendo a anterior, de modo que quem sempre emite o mesmo tipo
  de serviço só precisa informar número e valor. `--sem-assinar` gera o XML sem
  assinatura, para inspeção.
- Campos desconhecidos no YAML são rejeitados: num emissor fiscal, um `aliquota_iss`
  digitado no lugar de `iss_aliquota` produziria uma nota errada em silêncio.
- Registro de decisões de arquitetura em [`docs/decisoes/`](docs/decisoes/).

### Corrigido

- **A assinatura digital nunca verificou.** Três defeitos somados faziam com que
  toda DPS assinada fosse rejeitada: o documento era reindentado depois de
  assinado (invalidando digest e assinatura), a canonicalização perdia a
  declaração de namespace no ápice da subárvore, e a verificação exigia chave
  privada — que quem verifica nunca tem. Não existia nenhum teste que assinasse
  e depois verificasse; agora existe, cobrindo os três caminhos de assinatura e
  a detecção de adulteração.
  Ver [ADR 0004](docs/decisoes/0004-assinatura-que-nao-verificava.md).
- **O XML da DPS não seguia o schema oficial.** Sete divergências em relação a
  `DPS_v1.00.xsd`, entre elas `regEspTrib` e `tpRetISSQN` ausentes (ambos
  obrigatórios), descontos no elemento errado, `xDescServ` fora de `cServ`,
  `totTrib` no nível errado e com dois filhos onde o schema admite um, `subst`
  emitido como `<subst>2</subst>`, e base de cálculo e valor de ISS inventados
  dentro do elemento de benefício municipal — campos que a DPS não tem, porque
  quem os calcula é o governo.
  Ver [ADR 0003](docs/decisoes/0003-xml-conforme-o-xsd.md).
- O validador estrutural procurava `cTribNac`, `cLocPrest` e `vServPrest` em
  caminhos que não existem no schema, e exigia `subst`, que é opcional.

- **Certificados A1 em formato moderno não eram lidos.** `ParsePFX` usava
  `golang.org/x/crypto/pkcs12`, que só decodifica PKCS#12 com 3DES e MAC SHA-1.
  Arquivos gerados pelo OpenSSL 3 (AES-256-CBC, MAC SHA-256) — o padrão atual —
  falhavam com `unknown digest algorithm`, e a mensagem sugeria erroneamente que
  a senha estava errada. Migrado para `software.sslmate.com/src/go-pkcs12`, com
  teste de regressão cobrindo os dois formatos.
  Ver [ADR 0002](docs/decisoes/0002-parser-pkcs12.md).
- A cadeia de certificados intermediários passa a ser extraída do PFX; antes era
  descartada (`Chain: nil`).

### Alterado

- **O projeto deixou de ser uma API REST e passou a ser um CLI.**
  Ver [ADR 0001](docs/decisoes/0001-cli-em-vez-de-api.md).
- Módulo Go renomeado de `github.com/eduardo/nfse-nacional` para
  `github.com/edusouza/nfse-emissor-go`, agora coincidindo com o repositório —
  requisito para `go install` funcionar.
- Código Go movido de `src/` para a raiz do repositório, como é convenção em Go.
- Dependências diretas reduzidas de 12 para 5 (`etree`, `cobra`, `go-pkcs12`,
  `golang.org/x/term`, `yaml.v3`).
- Versão mínima do Go passou a 1.26, exigida por `golang.org/x/term` e pela
  cadeia do `cobra`. Manter 1.25 custaria rebaixar o `golang.org/x/crypto`, o
  que não compensa num programa que lida com certificado digital.
- Testes dependentes de relógio agora são pulados com `-short`, reduzindo a
  suíte local de ~75s para ~2s.

### Removido

- API REST (Gin), worker assíncrono (Asynq), MongoDB, Redis, envio de webhooks,
  autenticação por chave de API, *rate limiting*, métricas Prometheus,
  health checks e `docker-compose.yml`.
- `internal/config`, que lia apenas variáveis de ambiente de servidor.
- Entidades de persistência em `internal/domain/entities.go`, mantidos apenas
  `Address` e `Values`, que são usados pelo domínio.

### Problemas conhecidos

- O cliente da Sefin Nacional implementa um contrato **SOAP incorreto**; a API
  real é REST/JSON. Nenhuma emissão real funciona até isso ser corrigido.
  Ver [#3](https://github.com/edusouza/nfse-emissor-go/issues/3).
- A validação chamada de "XSD" é estrutural e não lê os schemas oficiais. O tipo
  foi renomeado para `StructuralValidator` e o parâmetro `schemaDir`, que era
  ignorado, foi removido.
  Ver [#4](https://github.com/edusouza/nfse-emissor-go/issues/4).
- A alíquota de ISS é informada pelo usuário, sem consulta aos parâmetros
  municipais. Ver [#5](https://github.com/edusouza/nfse-emissor-go/issues/5).

---

## [1.0.0] - 2026-01-08

Versão da API REST, anterior à mudança para CLI. Mantida aqui como registro
histórico; o código correspondente está no histórico do git.

### Adicionado

- API REST para emissão de NFS-e com processamento assíncrono, autenticação por
  chave de API, *rate limiting*, webhooks de notificação e métricas Prometheus.
- Assinatura XMLDSig (RSA-SHA256, canonicalização exc-c14n) e leitura de
  certificados A1.
- Montagem do XML da DPS, cálculo de valores e tradução de códigos de rejeição.
