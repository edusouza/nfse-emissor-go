# Implementation Plan: NFS-e Emission Core API

**Branch**: `001-nfse-emission-core` | **Date**: 2026-01-08 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-nfse-emission-core/spec.md`

## Summary

Backend REST API service for NFS-e (Brazilian electronic service invoice) emission, targeting software integrators. The service acts as middleware between integrators and Brazil's Sistema Nacional NFS-e, handling:
- JSON to XML transformation (DPS generation)
- Digital certificate signing (XMLDSig)
- Async queue processing with exponential backoff retries
- Webhook callbacks for results

Focus: SIMPLES NACIONAL contributors (MEI and ME/EPP) providing services only.

## Technical Context

**Language/Version**: Go 1.21+ with Gin web framework
**Primary Dependencies**: Gin (HTTP), Asynq (Redis-based job queue), go-xmlsec (XML signing), mongo-driver
**Storage**: MongoDB (request status, API keys), Redis (job queue, rate limiting)
**Testing**: Go testing package + testify, httptest for API tests
**Target Platform**: Linux server (Docker container)
**Project Type**: Single backend service (API + worker)
**Performance Goals**: 100 requests/minute per API key, 95% webhook callback within 60 seconds
**Constraints**: Stateless design (no company data persistence), government API latency variable
**Scale/Scope**: Initial target 10-50 integrators, ~10k NFS-e/day capacity

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

The constitution template is not yet configured for this project. Proceeding with standard best practices:

| Principle | Status | Notes |
|-----------|--------|-------|
| Single Responsibility | PASS | One service, one purpose (NFS-e emission) |
| Test Coverage | PASS | Contract + integration + unit tests planned |
| API-First | PASS | REST API with OpenAPI spec |
| Observability | PASS | Structured logging, request tracing planned |
| Simplicity | PASS | Minimal dependencies, clear data flow |

## Project Structure

### Documentation (this feature)

```text
specs/001-nfse-emission-core/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (OpenAPI spec)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
src/
├── cmd/
│   ├── api/             # HTTP API server entry point
│   │   └── main.go
│   └── worker/          # Async job worker entry point
│       └── main.go
├── internal/
│   ├── api/             # HTTP handlers and middleware
│   │   ├── handlers/    # Request handlers
│   │   ├── middleware/  # Auth, rate limiting, logging
│   │   └── routes.go
│   ├── domain/          # Business entities and logic
│   │   ├── emission/    # NFS-e emission domain
│   │   ├── validation/  # CNPJ/CPF, schema validation
│   │   └── entities.go
│   ├── infrastructure/  # External integrations
│   │   ├── sefin/       # Government API client
│   │   ├── mongodb/     # Database repositories
│   │   ├── redis/       # Queue and rate limiter
│   │   └── xmlsigner/   # Certificate handling, XMLDSig
│   ├── jobs/            # Async job definitions
│   │   └── emission.go
│   └── config/          # Configuration loading
├── pkg/                 # Shared utilities (exportable)
│   ├── cnpjcpf/         # Brazilian document validation
│   └── xmlbuilder/      # DPS XML generation
└── docs/
    └── schemas/         # XSD files (copied from project docs)

tests/
├── contract/            # API contract tests
├── integration/         # Full flow tests with mocks
└── unit/                # Unit tests (co-located with src also)
```

**Structure Decision**: Single project with two entry points (API server + worker). Internal packages keep business logic isolated. Infrastructure layer abstracts external dependencies for testability.

## Complexity Tracking

No violations requiring justification. Design follows standard patterns for async API services.

---

## Planning Phases

### Phase 0: Research ✓

**Status**: Complete

**Output**: [research.md](./research.md)

Key decisions made:
| Decision | Choice | Rationale |
|----------|--------|-----------|
| Web Framework | Gin | High performance, excellent middleware support, strong community |
| Job Queue | Asynq | Native Go, Redis-backed, built-in retry with exponential backoff |
| XML Signing | etree + crypto/x509 | Pure Go (no CGO), full control over XMLDSig |
| Database | MongoDB | Flexible schema for varying payloads, TTL indexes for cleanup |
| Rate Limiting | go-redis/redis_rate | Distributed, token bucket algorithm, Redis already in stack |

### Phase 1: Design & Contracts ✓

**Status**: Complete

**Outputs**:
- [data-model.md](./data-model.md) - MongoDB collections, entities, DTOs, validation rules
- [contracts/openapi.yaml](./contracts/openapi.yaml) - OpenAPI 3.1 specification
- [quickstart.md](./quickstart.md) - Developer setup and usage guide

**Data Model Summary**:
| Collection | Purpose |
|------------|---------|
| `api_keys` | Integrator credentials, webhook config, rate limits |
| `emission_requests` | Request lifecycle tracking, results storage |
| `webhook_deliveries` | Webhook attempt logs |

**API Endpoints**:
| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/nfse` | Submit emission request (JSON + certificate) |
| POST | `/v1/nfse/xml` | Submit pre-signed XML |
| GET | `/v1/nfse/status/{requestId}` | Query request status |
| GET | `/health` | Health check |

### Phase 2: Task Breakdown ✓

**Status**: Complete

**Output**: [tasks.md](./tasks.md)

**Task Summary**:
| Phase | Tasks | Description |
|-------|-------|-------------|
| Setup | 7 | Project structure, Docker, dependencies |
| Foundational | 15 | Config, DB, auth, rate limiting, middleware |
| US1 (P1) | 16 | Basic emission with async queue |
| US2 (P1) | 7 | Certificate signing (XMLDSig) |
| US3 (P2) | 6 | Pre-signed XML submission |
| US4 (P2) | 5 | Taker information support |
| US5 (P3) | 4 | Discounts and deductions |
| Polish | 7 | Metrics, logging, documentation |
| **Total** | **67** | 23 parallelizable |

---

## Implementation Readiness Checklist

- [x] Technical context defined (Go, Gin, MongoDB, Redis)
- [x] Research complete (all technology decisions documented)
- [x] Data model designed (3 MongoDB collections)
- [x] API contracts defined (OpenAPI 3.1 spec)
- [x] Developer quickstart guide created
- [x] Agent context updated (CLAUDE.md)
- [x] Task breakdown generated (`/speckit.tasks`)
- [x] Implementation complete (67/67 tasks)
