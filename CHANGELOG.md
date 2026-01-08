# Changelog

Todas as mudanças notáveis neste projeto serão documentadas neste arquivo.

O formato é baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.0.0/),
e este projeto adere ao [Versionamento Semântico](https://semver.org/lang/pt-BR/).

## [Unreleased]

### Planejado
- **002-nfse-query**: Consulta e recuperação de NFS-e emitidas
- **003-nfse-events**: Eventos de cancelamento e substituição
- **004-municipal-parameters**: Configurações específicas por município

---

## [1.0.0] - 2026-01-08

### Adicionado

#### API REST
- `POST /v1/nfse` - Submissão de emissão via JSON com certificado
- `POST /v1/nfse/xml` - Submissão de XML DPS pré-assinado
- `GET /v1/nfse/status/{requestId}` - Consulta de status da emissão
- `GET /health` - Health check com status dos componentes
- `GET /health/live` - Liveness probe para Kubernetes
- `GET /health/ready` - Readiness probe para Kubernetes
- `GET /metrics` - Métricas no formato Prometheus

#### Autenticação e Segurança
- Autenticação via API Key no header `X-API-Key`
- Hash SHA-256 das chaves na base de dados
- Rate limiting por chave (GCRA algorithm)
- Headers `X-RateLimit-*` nas respostas
- Resposta 429 com `Retry-After` quando limite excedido

#### Assinatura Digital (XMLDSig)
- Parser de certificados PFX/P12 (A1)
- Validação de certificados (expiração, uso de chave)
- Assinatura RSA-SHA256 com canonicalização exc-c14n
- Verificação de assinaturas em XML pré-assinado
- Implementação pure Go (sem dependência CGO)

#### Processamento Assíncrono
- Fila de jobs com Redis/Asynq
- Worker com concorrência configurável
- Retry automático com backoff exponencial
- Graceful shutdown para API e Worker

#### Webhooks
- Notificações de sucesso e falha
- Assinatura HMAC-SHA256 no header `X-Webhook-Signature`
- Retry com backoff exponencial (3 tentativas)
- Registro de tentativas de entrega

#### Validação
- CNPJ com verificação de dígitos (módulo 11)
- CPF com verificação de dígitos (módulo 11)
- NIF para tomadores estrangeiros
- Código de serviço nacional (cTribNac)
- Código de município (IBGE 7 dígitos)
- Regime tributário (MEI, ME/EPP)

#### Cálculo Fiscal
- Valor do serviço com precisão de 2 casas decimais
- Desconto incondicional (reduz base de cálculo)
- Desconto condicional (não reduz base)
- Deduções conforme legislação
- Cálculo automático da base de cálculo
- Percentual de dedução (pDR)

#### Construção de XML
- Geração de DPS conforme schema XSD v1.00
- ID do DPS no formato padrão nacional
- Seção de prestador com regime tributário
- Seção de tomador (CNPJ/CPF/NIF)
- Seção de serviço com códigos
- Seção de valores com tributos
- Endereços nacionais e estrangeiros

#### Infraestrutura
- Docker Compose com MongoDB, Redis, API e Worker
- Dockerfiles multi-stage para builds otimizados
- Configuração via variáveis de ambiente
- Logging estruturado (JSON)
- Métricas Prometheus para observabilidade

#### Documentação
- README com quick start e exemplos
- OpenAPI 3.1 specification
- Quickstart guide para desenvolvedores
- Documentação de variáveis de ambiente

### Técnico

#### Dependências Principais
- `github.com/gin-gonic/gin` v1.11.0 - Framework HTTP
- `github.com/hibiken/asynq` v0.25.1 - Fila de jobs
- `go.mongodb.org/mongo-driver` v1.17.6 - Driver MongoDB
- `github.com/redis/go-redis/v9` v9.17.2 - Cliente Redis
- `github.com/go-redis/redis_rate/v10` v10.0.1 - Rate limiting
- `github.com/beevik/etree` v1.6.0 - Manipulação XML
- `github.com/google/uuid` v1.6.0 - Geração de UUIDs
- `golang.org/x/crypto` v0.40.0 - Criptografia

#### Schemas XSD Incluídos
- `DPS_v1.00.xsd` - Declaração de Prestação de Serviço
- `NFSe_v1.00.xsd` - Nota Fiscal de Serviço Eletrônica
- `evento_v1.00.xsd` - Eventos (cancelamento, etc.)
- `tiposComplexos_v1.00.xsd` - Tipos complexos
- `tiposSimples_v1.00.xsd` - Tipos simples
- `xmldsig-core-schema.xsd` - Assinatura digital XML

---

## Tipos de Mudanças

- **Adicionado** para novas funcionalidades
- **Modificado** para mudanças em funcionalidades existentes
- **Obsoleto** para funcionalidades que serão removidas em breve
- **Removido** para funcionalidades removidas
- **Corrigido** para correções de bugs
- **Segurança** para correções de vulnerabilidades

[Unreleased]: https://github.com/edusouza/nfse-emissor-go/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/edusouza/nfse-emissor-go/releases/tag/v1.0.0
