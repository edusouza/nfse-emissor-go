# Implementation Plan: NFS-e Query and Retrieval API

**Branch**: `002-nfse-query` | **Date**: 2026-01-08 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/002-nfse-query/spec.md`

## Summary

Backend REST API endpoints for querying and retrieving NFS-e (Brazilian electronic service invoices) from the Sistema Nacional NFS-e. The service extends the existing emission API (001-nfse-emission-core) with read operations:
- Query NFS-e by access key (chaveAcesso)
- Lookup access key by DPS identifier
- Check NFS-e existence by DPS (HEAD)
- Query emission request status
- Query events linked to NFS-e

Focus: Providing integrators with comprehensive query capabilities while respecting fiscal secrecy rules for DPS-based lookups.

## Technical Context

**Language/Version**: Go 1.21+ with Gin web framework (existing stack)
**Primary Dependencies**: Gin (HTTP), mongo-driver, go-redis (existing)
**Storage**: MongoDB (emission_requests - existing), Redis (rate limiting - existing)
**Testing**: Go testing package + testify, httptest for API tests
**Target Platform**: Linux server (Docker container)
**Project Type**: Single backend service (extends existing API server)
**Performance Goals**: 200 requests/minute per API key, 3-second response time under normal conditions
**Constraints**: Government API timeout 10 seconds, stateless design, fiscal secrecy compliance
**Scale/Scope**: Same as emission API (~10k requests/day capacity)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

The constitution template is not yet configured for this project. Proceeding with standard best practices:

| Principle | Status | Notes |
|-----------|--------|-------|
| Single Responsibility | PASS | Query endpoints extend existing API service |
| Test Coverage | PASS | Contract + integration + unit tests planned |
| API-First | PASS | REST API with OpenAPI spec extension |
| Observability | PASS | Uses existing logging infrastructure |
| Simplicity | PASS | Reuses existing SEFIN client, adds query methods |

## Project Structure

### Documentation (this feature)

```text
specs/002-nfse-query/
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
│   └── api/             # HTTP API server (existing)
├── internal/
│   ├── api/
│   │   ├── handlers/
│   │   │   ├── query.go       # NEW: NFS-e query handlers
│   │   │   ├── dps.go         # NEW: DPS lookup handlers
│   │   │   └── status.go      # EXISTING (extends)
│   │   └── routes.go          # UPDATE: Add query routes
│   ├── domain/
│   │   ├── query/             # NEW: Query domain logic
│   │   │   ├── nfse.go        # NFS-e document parsing
│   │   │   ├── dps.go         # DPS ID validation
│   │   │   ├── accesskey.go   # Access key validation
│   │   │   ├── errors.go      # Query error codes + translation
│   │   │   └── response.go    # Response DTOs
│   │   └── validation/        # EXISTING (reuse)
│   └── infrastructure/
│       ├── sefin/
│       │   └── client.go      # UPDATE: Add query methods
│       └── mongodb/
│           └── emission_repo.go  # EXISTING (reuse for status)
├── pkg/
│   └── dpsid/                 # NEW: DPS identifier utilities
│       └── parser.go          # DPS ID parsing/validation
└── tests/
    ├── contract/
    │   └── query_test.go      # NEW: Query API contracts
    └── integration/
        └── query_flow_test.go # NEW: Query flow tests
```

**Structure Decision**: Extends existing single project. New handlers for query operations, new domain package for query logic, extends SEFIN client with query methods.

## Complexity Tracking

No violations requiring justification. Design reuses existing infrastructure and follows established patterns.

---

## Planning Phases

### Phase 0: Research ✓

**Status**: Complete

**Output**: [research.md](./research.md)

Key decisions made:
| Decision | Choice | Rationale |
|----------|--------|-----------|
| SEFIN Query Method | REST GET (JSON) | Government API uses REST for queries, SOAP for submissions |
| DPS ID Format | 42-char composite | Matches government spec: MunCode(7)+RegType(1)+FedReg(14)+Series(5)+Number(15) |
| Certificate Auth | Per-request (PFX) | Consistent with emission API, stateless design |
| Response Caching | None (deferred) | Out of scope per spec; government data authoritative |
| Error Translation | Reuse existing | Extend existing error code translation infrastructure |

### Phase 1: Design & Contracts ✓

**Status**: Complete

**Outputs**:
- [data-model.md](./data-model.md) - Response DTOs, DPS identifier, NFS-e document structure
- [contracts/openapi.yaml](./contracts/openapi.yaml) - OpenAPI 3.1 specification extension
- [quickstart.md](./quickstart.md) - Developer usage guide for query endpoints

**API Endpoints**:
| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/nfse/{chaveAcesso}` | Retrieve NFS-e by access key |
| GET | `/v1/nfse/{chaveAcesso}/eventos` | Retrieve events for NFS-e |
| GET | `/v1/dps/{id}` | Lookup access key by DPS ID (cert required) |
| HEAD | `/v1/dps/{id}` | Check NFS-e existence by DPS ID (cert required) |
| GET | `/v1/nfse/status/{requestId}` | Query request status (existing) |

### Phase 2: Task Breakdown ✓

**Status**: Complete

**Output**: [tasks.md](./tasks.md) - 46 tasks across 8 phases

---

## Implementation Readiness Checklist

- [x] Technical context defined (Go, Gin, MongoDB - reuse existing)
- [x] Research complete (query patterns, DPS format documented)
- [x] Data model designed (DTOs, response structures)
- [x] API contracts defined (OpenAPI 3.1 extension)
- [x] Developer quickstart guide created
- [x] Agent context updated (CLAUDE.md)
- [x] Task breakdown generated (`/speckit.tasks`)
- [x] Implementation complete
