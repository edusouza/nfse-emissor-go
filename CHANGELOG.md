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
- Registro de decisões de arquitetura em [`docs/decisoes/`](docs/decisoes/).

### Corrigido

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
- Dependências diretas reduzidas de 12 para 4 (`etree`, `cobra`, `go-pkcs12`,
  `golang.org/x/{crypto,term}`).
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
- A validação chamada de "XSD" é estrutural e não lê os schemas oficiais.
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
