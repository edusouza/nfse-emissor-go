# Tasks: NFS-e Emission Core API

**Input**: Design documents from `/specs/001-nfse-emission-core/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/openapi.yaml

**Tests**: Not explicitly requested - test tasks excluded. Add integration tests in Polish phase.

**Organization**: Tasks grouped by user story to enable independent implementation.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, etc.)
- Paths follow plan.md structure: `src/` at repository root

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and Go module setup

- [x] T001 Create Go project structure per plan.md in src/
- [x] T002 Initialize Go module with `go mod init` and add dependencies (gin, asynq, mongo-driver, redis, etree, testify)
- [x] T003 [P] Copy XSD schemas from docs/schemas/ to src/docs/schemas/
- [x] T004 [P] Create Dockerfile for API server in src/cmd/api/Dockerfile
- [x] T005 [P] Create Dockerfile for worker in src/cmd/worker/Dockerfile
- [x] T006 [P] Create docker-compose.yml with MongoDB, Redis, API, Worker services
- [x] T007 Create .env.example with all required environment variables

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T008 Implement configuration loader in src/internal/config/config.go (env vars, defaults)
- [x] T009 [P] Implement MongoDB connection pool in src/internal/infrastructure/mongodb/client.go
- [x] T010 [P] Implement Redis connection in src/internal/infrastructure/redis/client.go
- [x] T011 Implement API key repository in src/internal/infrastructure/mongodb/apikey_repo.go
- [x] T012 Implement authentication middleware in src/internal/api/middleware/auth.go (X-API-Key header)
- [x] T013 Implement rate limiting middleware in src/internal/api/middleware/ratelimit.go (go-redis/redis_rate)
- [x] T014 [P] Implement structured logging middleware in src/internal/api/middleware/logging.go
- [x] T015 [P] Implement request ID middleware in src/internal/api/middleware/requestid.go
- [x] T016 Implement error response helper in src/internal/api/handlers/errors.go (RFC 7807 format)
- [x] T017 [P] Implement CNPJ validation in src/pkg/cnpjcpf/cnpj.go (digit verification algorithm)
- [x] T018 [P] Implement CPF validation in src/pkg/cnpjcpf/cpf.go (digit verification algorithm)
- [x] T019 Implement base domain entities in src/internal/domain/entities.go (Provider, Taker, Service, Values, Address)
- [x] T020 Setup Gin router with middleware chain in src/internal/api/routes.go
- [x] T021 Implement health check endpoint in src/internal/api/handlers/health.go (GET /health)
- [x] T022 Create API server entry point in src/cmd/api/main.go

**Checkpoint**: Foundation ready - run `go build ./cmd/api` and verify health endpoint works

---

## Phase 3: User Story 1 - Basic NFS-e Emission (Priority: P1) 🎯 MVP

**Goal**: Submit emission request with minimal fields, receive NFS-e via webhook

**Independent Test**: POST valid JSON to `/v1/nfse`, get HTTP 202 with request_id, receive webhook callback

### Implementation for User Story 1

- [x] T023 [P] [US1] Implement EmissionRequest DTO in src/internal/domain/emission/request.go
- [x] T024 [P] [US1] Implement EmissionResponse DTO in src/internal/domain/emission/response.go
- [x] T025 [US1] Implement request validation in src/internal/domain/validation/emission.go (required fields, CNPJ, tax regime)
- [x] T026 [US1] Implement emission_requests MongoDB repository in src/internal/infrastructure/mongodb/emission_repo.go
- [x] T027 [US1] Implement DPS XML builder (basic fields) in src/pkg/xmlbuilder/dps.go
- [x] T028 [US1] Implement DPS ID generator in src/pkg/xmlbuilder/dps_id.go (format: DPS + MunCode + RegType + FedReg + Series + Number)
- [x] T029 [US1] Implement Asynq job client setup in src/internal/infrastructure/redis/queue.go
- [x] T030 [US1] Implement emission job definition in src/internal/jobs/emission.go (task type, payload)
- [x] T031 [US1] Implement POST /v1/nfse handler in src/internal/api/handlers/emission.go (validate, save, queue, return 202)
- [x] T032 [US1] Implement GET /v1/nfse/status/{requestId} handler in src/internal/api/handlers/status.go
- [x] T033 [US1] Implement Sefin Nacional API client stub in src/internal/infrastructure/sefin/client.go (interface + mock for dev)
- [x] T034 [US1] Implement webhook delivery service in src/internal/infrastructure/webhook/sender.go (HMAC signature)
- [x] T035 [US1] Implement webhook_deliveries repository in src/internal/infrastructure/mongodb/webhook_repo.go
- [x] T036 [US1] Implement emission job processor in src/internal/jobs/processor.go (build XML, call Sefin, save result, trigger webhook)
- [x] T037 [US1] Create worker entry point in src/cmd/worker/main.go (Asynq server with emission handler)
- [x] T038 [US1] Add emission routes to router in src/internal/api/routes.go

**Checkpoint**: User Story 1 complete - can submit emission request, track status, receive webhook

---

## Phase 4: User Story 2 - Service-Signed XML (Priority: P1)

**Goal**: Accept PFX certificate in request, service handles XML signing

**Independent Test**: POST with certificate.pfx_base64 + password, verify signed XML in webhook

### Implementation for User Story 2

- [x] T039 [P] [US2] Implement PFX certificate parser in src/internal/infrastructure/xmlsigner/certificate.go (crypto/x509)
- [x] T040 [P] [US2] Implement certificate validation in src/internal/infrastructure/xmlsigner/validate.go (expiry, format)
- [x] T041 [US2] Implement XMLDSig signer in src/internal/infrastructure/xmlsigner/signer.go (etree + crypto)
- [x] T042 [US2] Implement canonical XML (C14N) in src/internal/infrastructure/xmlsigner/canonicalize.go
- [x] T043 [US2] Extend emission request validation for certificate fields in src/internal/domain/validation/certificate.go
- [x] T044 [US2] Integrate signing into emission job processor in src/internal/jobs/processor.go
- [x] T045 [US2] Add certificate error responses (400) in src/internal/api/handlers/emission.go

**Checkpoint**: User Story 2 complete - certificates accepted and XML signed correctly

---

## Phase 5: User Story 3 - Pre-Signed XML Submission (Priority: P2)

**Goal**: Accept pre-signed DPS XML directly, validate and forward to government

**Independent Test**: POST valid signed XML to `/v1/nfse/xml`, verify it's submitted without re-signing

### Implementation for User Story 3

- [x] T046 [P] [US3] Implement XML signature validator in src/internal/infrastructure/xmlsigner/verifier.go
- [x] T047 [P] [US3] Implement XSD schema validator in src/internal/domain/validation/xsd.go
- [x] T048 [US3] Implement PreSignedRequest DTO in src/internal/domain/emission/presigned.go
- [x] T049 [US3] Implement POST /v1/nfse/xml handler in src/internal/api/handlers/emission_xml.go
- [x] T050 [US3] Extend emission job to handle pre-signed flow in src/internal/jobs/processor.go
- [x] T051 [US3] Add pre-signed routes to router in src/internal/api/routes.go

**Checkpoint**: User Story 3 complete - pre-signed XML accepted and processed

---

## Phase 6: User Story 4 - NFS-e with Taker Information (Priority: P2)

**Goal**: Include taker (tomador) details in NFS-e emission

**Independent Test**: POST with taker CNPJ/CPF/NIF, verify taker info in resulting NFS-e XML

### Implementation for User Story 4

- [x] T052 [P] [US4] Extend Taker entity with NIF support in src/internal/domain/entities.go
- [x] T053 [US4] Implement taker validation (CNPJ/CPF/NIF options) in src/internal/domain/validation/taker.go
- [x] T054 [US4] Extend DPS XML builder with taker section in src/pkg/xmlbuilder/dps.go
- [x] T055 [US4] Add taker address handling (national + foreign) in src/pkg/xmlbuilder/address.go
- [x] T056 [US4] Update emission request validation to include optional taker in src/internal/domain/validation/emission.go

**Checkpoint**: User Story 4 complete - NFS-e includes taker information when provided

---

## Phase 7: User Story 5 - NFS-e with Discounts and Deductions (Priority: P3)

**Goal**: Apply discounts and deductions to service value for correct tax calculation

**Independent Test**: POST with discounts, verify tax base = service_value - unconditional_discount

### Implementation for User Story 5

- [x] T057 [P] [US5] Implement discount/deduction calculation in src/internal/domain/emission/calculator.go
- [x] T058 [US5] Extend DPS XML builder with discount fields in src/pkg/xmlbuilder/dps.go
- [x] T059 [US5] Add deduction validation rules in src/internal/domain/validation/values.go
- [x] T060 [US5] Update emission request to include discount/deduction fields in src/internal/domain/emission/request.go

**Checkpoint**: User Story 5 complete - discounts and deductions correctly applied

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Improvements affecting multiple user stories

- [x] T061 [P] Implement government rejection code translator in src/internal/domain/emission/errors.go
- [x] T062 [P] Add request/response logging for debugging in src/internal/api/middleware/logging.go
- [x] T063 Implement Sefin Nacional production client in src/internal/infrastructure/sefin/client.go (replace stub)
- [x] T064 [P] Add Prometheus metrics endpoint in src/internal/api/handlers/metrics.go
- [x] T065 [P] Create README.md with setup instructions in src/README.md
- [x] T066 Run quickstart.md validation (manual smoke test)
- [x] T067 Implement graceful shutdown for API and worker

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - start immediately
- **Foundational (Phase 2)**: Depends on Setup - BLOCKS all user stories
- **User Stories (Phase 3-7)**: All depend on Foundational completion
  - US1 and US2 are both P1 - US2 depends on US1 core (signing extends emission)
  - US3 and US4 are P2 - can start after US1/US2
  - US5 is P3 - lowest priority
- **Polish (Phase 8)**: After all desired user stories complete

### User Story Dependencies

| Story | Depends On | Can Start After |
|-------|------------|-----------------|
| US1 (Basic Emission) | Foundational | Phase 2 complete |
| US2 (Service-Signed) | US1 core handlers | T031 complete |
| US3 (Pre-Signed XML) | US1 core handlers | T031 complete |
| US4 (Taker Info) | US1 XML builder | T027 complete |
| US5 (Discounts) | US1 XML builder | T027 complete |

### Parallel Opportunities per Phase

**Phase 1 (Setup)**: T003, T004, T005, T006 can run in parallel

**Phase 2 (Foundational)**: T009+T010, T014+T015, T017+T018 can run in parallel

**Phase 3 (US1)**: T023+T024 can run in parallel

**Phase 4 (US2)**: T039+T040 can run in parallel

**Phase 5 (US3)**: T046+T047 can run in parallel

---

## Parallel Example: Phase 2 Foundational

```bash
# These can run simultaneously (different files, no dependencies):
Task: "Implement MongoDB connection pool in src/internal/infrastructure/mongodb/client.go"
Task: "Implement Redis connection in src/internal/infrastructure/redis/client.go"
Task: "Implement structured logging middleware in src/internal/api/middleware/logging.go"
Task: "Implement CNPJ validation in src/pkg/cnpjcpf/cnpj.go"
Task: "Implement CPF validation in src/pkg/cnpjcpf/cpf.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 + 2 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL)
3. Complete Phase 3: User Story 1 (Basic Emission)
4. Complete Phase 4: User Story 2 (Service-Signed) - completes signing capability
5. **STOP and VALIDATE**: Test emission with certificate
6. Deploy MVP to homologation environment

### Incremental Delivery

1. **MVP**: Setup + Foundational + US1 + US2 → Core emission with signing
2. **+US3**: Add pre-signed XML support → Serves integrators with existing signing
3. **+US4**: Add taker info support → Complete B2B invoicing
4. **+US5**: Add discounts/deductions → Full tax calculation support
5. Each increment is independently deployable

---

## Task Summary

| Phase | Tasks | Parallel Tasks |
|-------|-------|----------------|
| Phase 1: Setup | 7 | 4 |
| Phase 2: Foundational | 15 | 7 |
| Phase 3: US1 (P1) | 16 | 2 |
| Phase 4: US2 (P1) | 7 | 2 |
| Phase 5: US3 (P2) | 6 | 2 |
| Phase 6: US4 (P2) | 5 | 1 |
| Phase 7: US5 (P3) | 4 | 1 |
| Phase 8: Polish | 7 | 4 |
| **Total** | **67** | **23** |

---

## Notes

- [P] tasks = different files, no dependencies within same phase
- Each user story is independently testable after completion
- Commit after each task or logical group
- Stop at any checkpoint to validate
- US1 + US2 = MVP (core emission with signing)
