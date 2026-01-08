# NFS-e Emissor Go

Backend REST API para emissão de NFS-e (Nota Fiscal de Serviço Eletrônica) através do Sistema Nacional NFS-e do Brasil.

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Visão Geral

Este projeto implementa uma API REST para emissão de notas fiscais de serviço eletrônicas, focado em prestadores do **SIMPLES NACIONAL** (MEI, ME e EPP). A solução oferece:

- **Emissão de NFS-e** via JSON com assinatura digital automática
- **Submissão de XML pré-assinado** para integradores com sua própria assinatura
- **Processamento assíncrono** com notificações via webhook
- **Rastreamento de status** em tempo real
- **Tradução de erros** do governo para mensagens amigáveis

## Arquitetura

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Client    │────▶│   API       │────▶│   Queue     │
│  (JSON/XML) │     │   (Gin)     │     │  (Asynq)    │
└─────────────┘     └─────────────┘     └──────┬──────┘
                           │                    │
                           ▼                    ▼
                    ┌─────────────┐     ┌─────────────┐
                    │  MongoDB    │     │   Worker    │
                    │  (Status)   │     │ (Processor) │
                    └─────────────┘     └──────┬──────┘
                                               │
                           ┌───────────────────┼───────────────────┐
                           ▼                   ▼                   ▼
                    ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
                    │  XMLDSig    │     │   SEFIN     │     │  Webhook    │
                    │  (Signing)  │     │   (Gov API) │     │  (Callback) │
                    └─────────────┘     └─────────────┘     └─────────────┘
```

## Stack Tecnológico

| Componente | Tecnologia | Propósito |
|------------|------------|-----------|
| **Runtime** | Go 1.21+ | Linguagem principal |
| **HTTP** | Gin | Framework web |
| **Queue** | Asynq (Redis) | Processamento assíncrono |
| **Database** | MongoDB | Persistência de status e chaves |
| **Cache** | Redis | Fila de jobs e rate limiting |
| **XML** | etree + crypto/x509 | Assinatura XMLDSig (sem CGO) |

## Quick Start

### Pré-requisitos

- Go 1.21+
- Docker e Docker Compose
- Git

### Instalação

```bash
# Clone o repositório
git clone git@github.com:edusouza/nfse-emissor-go.git
cd nfse-emissor-go

# Configure o ambiente
cd src
cp .env.example .env

# Inicie a infraestrutura
docker compose up -d mongodb redis

# Execute a API
go run ./cmd/api

# Em outro terminal, execute o worker
go run ./cmd/worker
```

A API estará disponível em `http://localhost:8080`.

### Verificar Instalação

```bash
# Health check
curl http://localhost:8080/health

# Resposta esperada:
{
  "status": "healthy",
  "version": "1.0.0",
  "components": {
    "mongodb": "healthy",
    "redis": "healthy"
  }
}
```

## Endpoints da API

### Públicos

| Método | Endpoint | Descrição |
|--------|----------|-----------|
| GET | `/health` | Health check completo |
| GET | `/health/live` | Liveness probe (K8s) |
| GET | `/health/ready` | Readiness probe (K8s) |
| GET | `/metrics` | Métricas Prometheus |

### Protegidos (requer `X-API-Key`)

| Método | Endpoint | Descrição |
|--------|----------|-----------|
| POST | `/v1/nfse` | Submeter emissão (JSON + certificado) |
| POST | `/v1/nfse/xml` | Submeter XML pré-assinado |
| GET | `/v1/nfse/status/{id}` | Consultar status da emissão |

## Exemplo de Uso

### Emitir NFS-e

```bash
curl -X POST http://localhost:8080/v1/nfse \
  -H "Content-Type: application/json" \
  -H "X-API-Key: sua-chave-api" \
  -d '{
    "provider": {
      "cnpj": "12345678000199",
      "tax_regime": "mei",
      "name": "Empresa Teste LTDA"
    },
    "service": {
      "national_code": "010101",
      "description": "Consultoria em tecnologia",
      "municipality_code": "3550308"
    },
    "values": {
      "service_value": 1500.00
    },
    "dps": {
      "series": "00001",
      "number": "1"
    },
    "certificate": {
      "pfx_base64": "<certificado-base64>",
      "password": "senha-certificado"
    }
  }'
```

### Resposta

```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending",
  "message": "Request queued for processing",
  "status_url": "http://localhost:8080/v1/nfse/status/550e8400-e29b-41d4-a716-446655440000"
}
```

## Funcionalidades Implementadas

### User Stories

- [x] **US1**: Emissão básica de NFS-e (P1)
- [x] **US2**: XML assinado pelo serviço com certificados (P1)
- [x] **US3**: Submissão de XML pré-assinado (P2)
- [x] **US4**: Informações do tomador (P2)
- [x] **US5**: Descontos e deduções (P3)

### Capacidades

| Feature | Descrição |
|---------|-----------|
| **Autenticação** | API Key com hash SHA-256 |
| **Rate Limiting** | GCRA por chave (100 req/min default) |
| **XMLDSig** | Assinatura RSA-SHA256 com exc-c14n |
| **Certificados** | Suporte a PFX/P12 (A1) |
| **Validação** | CNPJ/CPF/NIF com dígitos verificadores |
| **Cálculo Fiscal** | Base de cálculo com descontos/deduções |
| **Webhooks** | Callbacks HMAC-SHA256 com retry |
| **Métricas** | Prometheus para observabilidade |

## Estrutura do Projeto

```
.
├── docs/                    # Documentação do governo (PDFs, XSDs)
├── specs/                   # Especificações Speckit
│   └── 001-nfse-emission-core/
│       ├── spec.md          # Especificação funcional
│       ├── plan.md          # Plano de implementação
│       ├── tasks.md         # Breakdown de tarefas
│       └── contracts/       # OpenAPI spec
├── src/                     # Código fonte
│   ├── cmd/
│   │   ├── api/             # Entry point do servidor HTTP
│   │   └── worker/          # Entry point do worker assíncrono
│   ├── internal/
│   │   ├── api/             # Handlers, middleware, rotas
│   │   ├── domain/          # Entidades e validação
│   │   ├── infrastructure/  # MongoDB, Redis, Sefin, Webhook
│   │   └── jobs/            # Processador de emissão
│   └── pkg/
│       ├── cnpjcpf/         # Validação CNPJ/CPF
│       └── xmlbuilder/      # Construção de DPS XML
└── CLAUDE.md                # Contexto para Claude Code
```

## Testes

```bash
cd src

# Executar todos os testes
go test ./...

# Com cobertura
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Testes específicos
go test -v ./internal/domain/...
```

## Docker

### Desenvolvimento

```bash
cd src
docker compose up -d
```

### Produção

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

## Variáveis de Ambiente

| Variável | Default | Descrição |
|----------|---------|-----------|
| `PORT` | `8080` | Porta do servidor |
| `ENV` | `development` | Ambiente |
| `MONGODB_URI` | `mongodb://localhost:27017` | URI do MongoDB |
| `REDIS_URL` | `redis://localhost:6379` | URL do Redis |
| `SEFIN_API_URL` | `https://hom.nfse.gov.br/api` | URL da API do governo |
| `SEFIN_ENVIRONMENT` | `homologacao` | Ambiente SEFIN |
| `LOG_LEVEL` | `info` | Nível de log |
| `WORKER_CONCURRENCY` | `10` | Jobs paralelos |

Veja [src/.env.example](src/.env.example) para lista completa.

## Documentação

- [README detalhado](src/README.md) - Documentação técnica completa
- [OpenAPI Spec](specs/001-nfse-emission-core/contracts/openapi.yaml) - Especificação da API
- [Quickstart](specs/001-nfse-emission-core/quickstart.md) - Guia rápido

## Roadmap

### Próximos EPICs

- [ ] **002-nfse-query**: Consulta e recuperação de NFS-e
- [ ] **003-nfse-events**: Eventos (cancelamento, substituição)
- [ ] **004-municipal-parameters**: Parâmetros municipais

## Contribuição

1. Fork o projeto
2. Crie sua feature branch (`git checkout -b feature/nova-funcionalidade`)
3. Commit suas mudanças (`git commit -m 'feat: adiciona nova funcionalidade'`)
4. Push para a branch (`git push origin feature/nova-funcionalidade`)
5. Abra um Pull Request

## Licença

Este projeto está sob a licença MIT. Veja [LICENSE](LICENSE) para mais detalhes.

## Suporte

Para dúvidas e sugestões, abra uma [issue](https://github.com/edusouza/nfse-emissor-go/issues).

---

Desenvolvido com Go e Claude Code
