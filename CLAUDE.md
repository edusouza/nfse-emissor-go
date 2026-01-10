# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

NFS-e emission backend service for Brazil's Sistema Nacional NFS-e (National Electronic Service Invoice System). The project contains:
1. **Documentation**: Government specs, XSD schemas, and reference data for NFS-e compliance
2. **Backend Service**: Go REST API for NFS-e emission targeting SIMPLES NACIONAL service providers (MEI/ME/EPP)

## Tech Stack

- **Language**: Go 1.21+ with Gin web framework
- **Queue**: Asynq (Redis-based async job processing)
- **Database**: MongoDB (request status, API keys)
- **Cache**: Redis (job queue, rate limiting)

## Commands

```bash
# Run API server
go run ./src/cmd/api

# Run async worker
go run ./src/cmd/worker

# Run tests
go test ./...

# Run single test
go test -run TestEmission ./internal/domain/emission/

# Run with coverage
go test -coverprofile=coverage.out ./...

# Integration tests (requires MongoDB + Redis)
go test -tags=integration ./tests/integration/...

# PDF to markdown conversion (documentation utility)
pip install PyMuPDF && python convert_pdfs.py
```

## Architecture

```
src/
├── cmd/api/          # HTTP server entry point
├── cmd/worker/       # Async job worker entry point
├── internal/
│   ├── api/          # HTTP handlers, middleware, routes
│   ├── domain/       # Business logic (emission, validation)
│   ├── infrastructure/  # External integrations (sefin, mongodb, redis, xmlsigner)
│   └── jobs/         # Async job definitions
└── pkg/              # Shared utilities (cnpjcpf, xmlbuilder)
```

**Data Flow**: API receives JSON → validates → queues job → worker generates DPS XML → signs with certificate → submits to Sefin Nacional → webhook callback with result

## Documentation Structure

```
docs/
├── nfse-nacional/    # Original PDFs (6 guides)
├── markdown/         # Converted markdown + images
├── schemas/          # XSD files for XML validation
└── anexos/           # XLSX reference data (IBGE codes, service list)
```

## Feature Specs (Speckit)

Feature specifications are in `specs/` with this workflow:
- `/speckit.specify` - Create feature spec
- `/speckit.clarify` - Resolve ambiguities
- `/speckit.plan` - Generate implementation plan
- `/speckit.tasks` - Generate task breakdown

Current feature: `001-nfse-emission-core` (NFS-e emission API)

## Key XSD Schemas

| Schema | Purpose |
|--------|---------|
| DPS_v1.00.xsd | Service declaration (input) |
| NFSe_v1.00.xsd | Electronic invoice (output) |
| evento_v1.00.xsd | Events (cancellation, etc.) |

Target namespace: `http://www.sped.fazenda.gov.br/nfse`

## Domain Terms

- **DPS**: Declaração de Prestação de Serviço (Service Declaration) - input XML
- **NFS-e**: Nota Fiscal de Serviço Eletrônica - output invoice
- **SIMPLES NACIONAL**: Simplified tax regime (MEI = individual, ME/EPP = small business)
- **cTribNac**: National service code (6 digits per LC 116/2003)
- **chaveAcesso**: 50-character NFS-e access key
