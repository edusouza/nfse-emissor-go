# Tasks: NFS-e Query and Retrieval API

**Input**: Design documents from `/specs/002-nfse-query/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Not explicitly requested in feature specification. Tests omitted but can be added on request.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Single project**: `src/` at repository root (Go project)
- Paths follow existing 001-nfse-emission-core structure

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Core utilities and types shared across all user stories

- [X] T001 [P] Create DPS identifier package with parsing and validation in src/pkg/dpsid/parser.go
- [X] T002 [P] Create access key validation utility in src/internal/domain/query/accesskey.go
- [X] T003 [P] Create query-specific error codes and messages in src/internal/domain/query/errors.go
- [X] T004 [P] Extend error translation to map government query error codes to integrator-friendly messages in src/internal/domain/query/errors.go
- [X] T005 Create query response DTOs (NFSeQueryResponse, DPSLookupResponse, EventsQueryResponse) in src/internal/domain/query/response.go

**Checkpoint**: Core utilities ready - user story implementation can now begin

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: SEFIN client extension and infrastructure that ALL user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T006 Extend SefinClient interface with query methods (QueryNFSe, LookupDPS, CheckDPSExists, QueryEvents) in src/internal/infrastructure/sefin/client.go
- [X] T007 Implement QueryNFSe method in ProductionClient in src/internal/infrastructure/sefin/client.go
- [X] T008 Implement LookupDPS method in ProductionClient in src/internal/infrastructure/sefin/client.go
- [X] T009 Implement CheckDPSExists method in ProductionClient in src/internal/infrastructure/sefin/client.go
- [X] T010 Implement QueryEvents method in ProductionClient in src/internal/infrastructure/sefin/client.go
- [X] T011 Add mock implementations for all query methods in MockClient in src/internal/infrastructure/sefin/client.go
- [X] T012 [P] Update rate limiter configuration for query endpoints (200 req/min default) in src/internal/api/middleware/ratelimit.go

**Checkpoint**: Foundation ready - SEFIN client can now query government API

---

## Phase 3: User Story 1 - Query NFS-e by Access Key (Priority: P1) 🎯 MVP

**Goal**: Integrators can retrieve complete NFS-e documents using the 50-character access key

**Independent Test**: GET /v1/nfse/{chaveAcesso} returns complete NFS-e with provider, taker, service, values, and XML

### Implementation for User Story 1

- [X] T013 [US1] Create NFS-e XML parser to extract structured data in src/internal/domain/query/nfse.go
- [X] T014 [US1] Create query handler for GET /nfse/{chaveAcesso} in src/internal/api/handlers/query.go
- [X] T015 [US1] Add access key validation in query handler (50 chars, alphanumeric, starts with "NFSe") in src/internal/api/handlers/query.go
- [X] T016 [US1] Implement response mapping from SEFIN response to NFSeQueryResponse DTO in src/internal/api/handlers/query.go
- [X] T017 [US1] Add error handling for 404 (not found), 400 (invalid format), 503 (gov unavailable) in src/internal/api/handlers/query.go
- [X] T018 [US1] Register GET /v1/nfse/:chaveAcesso route in src/internal/api/routes.go
- [X] T019 [US1] Add request logging for NFS-e query operations in src/internal/api/handlers/query.go

**Checkpoint**: User Story 1 complete - integrators can query NFS-e by access key

---

## Phase 4: User Story 2 - Lookup Access Key by DPS Identifier (Priority: P1)

**Goal**: Integrators can recover NFS-e access key using DPS identifier when they don't have the access key

**Independent Test**: GET /v1/dps/{id} with valid certificate returns access key for authorized actors

### Implementation for User Story 2

- [X] T020 [US2] Create DPS lookup handler for GET /dps/{id} in src/internal/api/handlers/dps.go
- [X] T021 [US2] Add DPS ID validation (42 chars, all numeric, valid components) in src/internal/api/handlers/dps.go
- [X] T022 [US2] Implement certificate extraction from multipart form request in src/internal/api/handlers/dps.go
- [X] T023 [US2] Add certificate validation (valid PFX, not expired) in src/internal/api/handlers/dps.go
- [X] T024 [US2] Implement response mapping to DPSLookupResponse DTO in src/internal/api/handlers/dps.go
- [X] T025 [US2] Add error handling for 403 (forbidden), 404 (not found), 400 (invalid format/cert) in src/internal/api/handlers/dps.go
- [X] T026 [US2] Register GET /v1/dps/:id route in src/internal/api/routes.go
- [X] T027 [US2] Add request logging for DPS lookup operations in src/internal/api/handlers/dps.go

**Checkpoint**: User Story 2 complete - integrators can recover access keys via DPS ID

---

## Phase 5: User Story 3 - Check NFS-e Existence by DPS (Priority: P2)

**Goal**: Integrators can check if NFS-e exists for a DPS without actor restriction

**Independent Test**: HEAD /v1/dps/{id} returns 200 if exists, 404 if not (any valid certificate)

### Implementation for User Story 3

- [X] T028 [US3] Create HEAD handler for /dps/{id} in src/internal/api/handlers/dps.go
- [X] T029 [US3] Implement existence check without actor authorization in src/internal/api/handlers/dps.go
- [X] T030 [US3] Return HTTP 200 (no body) when NFS-e exists in src/internal/api/handlers/dps.go
- [X] T031 [US3] Return HTTP 404 when NFS-e does not exist in src/internal/api/handlers/dps.go
- [X] T032 [US3] Register HEAD /v1/dps/:id route in src/internal/api/routes.go

**Checkpoint**: User Story 3 complete - integrators can check NFS-e existence

---

## Phase 6: User Story 4 - Query Request Status (Priority: P2)

**Goal**: Integrators can check status of emission requests and retrieve results

**Independent Test**: GET /v1/nfse/status/{requestId} returns current status with details

### Implementation for User Story 4

- [X] T033 [US4] Extend existing status handler to include NFS-e details on success in src/internal/api/handlers/status.go
- [X] T034 [US4] Add nfse_xml_url field to successful status response in src/internal/api/handlers/status.go
- [X] T035 [US4] Ensure status endpoint uses correct route /v1/nfse/status/:requestId in src/internal/api/routes.go
- [X] T036 [US4] Add logging for status query operations in src/internal/api/handlers/status.go

**Checkpoint**: User Story 4 complete - integrators can track emission request status

---

## Phase 7: User Story 5 - Query Events by Access Key (Priority: P3)

**Goal**: Integrators can retrieve events linked to an NFS-e (cancellations, manifestations)

**Independent Test**: GET /v1/nfse/{chaveAcesso}/eventos returns list of events (may be empty)

### Implementation for User Story 5

- [X] T037 [US5] Create events query handler for GET /nfse/{chaveAcesso}/eventos in src/internal/api/handlers/query.go
- [X] T038 [US5] Add optional query parameter filtering by event type (?tipo=) in src/internal/api/handlers/query.go
- [X] T039 [US5] Implement response mapping to EventsQueryResponse DTO in src/internal/api/handlers/query.go
- [X] T040 [US5] Return empty list when no events exist (not 404) in src/internal/api/handlers/query.go
- [X] T041 [US5] Register GET /v1/nfse/:chaveAcesso/eventos route in src/internal/api/routes.go
- [X] T042 [US5] Add request logging for events query operations in src/internal/api/handlers/query.go

**Checkpoint**: User Story 5 complete - integrators can query NFS-e events

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Final integration, documentation, and cleanup

- [X] T043 [P] Verify all routes are registered correctly with proper HTTP methods in src/internal/api/routes.go
- [X] T044 [P] Ensure 10-second timeout is configured for all government API calls in src/internal/infrastructure/sefin/client.go
- [X] T045 Run quickstart.md validation - test all documented examples work as expected
- [X] T046 Code review and cleanup across all new files

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-7)**: All depend on Foundational phase completion
  - US1 and US2 are both P1 - can proceed in parallel
  - US3 and US4 are both P2 - can proceed in parallel after P1
  - US5 is P3 - can start after Foundational
- **Polish (Phase 8)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational - No dependencies on other stories
- **User Story 2 (P1)**: Can start after Foundational - No dependencies on other stories
- **User Story 3 (P2)**: Can start after Foundational - Shares dps.go with US2 but different handlers
- **User Story 4 (P2)**: Can start after Foundational - Extends existing status.go
- **User Story 5 (P3)**: Can start after Foundational - Extends query.go from US1

### Within Each User Story

- Validation before handlers
- Handlers before routes
- Error handling integrated with implementation
- Logging added at end of each story

### Parallel Opportunities

- T001, T002, T003, T004 can run in parallel (different files)
- T012 can run in parallel with T006-T011
- US1 (T013-T019) and US2 (T020-T027) can run in parallel after Phase 2
- US3 (T028-T032) and US4 (T033-T036) can run in parallel
- T043, T044 can run in parallel

---

## Parallel Example: Phase 1 (Setup)

```bash
# Launch all setup tasks together:
Task: "Create DPS identifier package in src/pkg/dpsid/parser.go"
Task: "Create access key validation in src/internal/domain/query/accesskey.go"
Task: "Create query error codes in src/internal/domain/query/errors.go"
```

## Parallel Example: User Stories 1 & 2 (Both P1)

```bash
# After Phase 2, these can run in parallel by different developers:
# Developer A: User Story 1
Task: "Create NFS-e XML parser in src/internal/domain/query/nfse.go"
Task: "Create query handler for GET /nfse in src/internal/api/handlers/query.go"

# Developer B: User Story 2
Task: "Create DPS lookup handler in src/internal/api/handlers/dps.go"
Task: "Implement certificate extraction in src/internal/api/handlers/dps.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1 (Query NFS-e by Access Key)
4. **STOP and VALIDATE**: Test GET /v1/nfse/{chaveAcesso} independently
5. Deploy/demo if ready - integrators can query NFS-e

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy (MVP - NFS-e query)
3. Add User Story 2 → Test independently → Deploy (DPS lookup)
4. Add User Story 3 → Test independently → Deploy (Existence check)
5. Add User Story 4 → Test independently → Deploy (Status query)
6. Add User Story 5 → Test independently → Deploy (Events query)

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (query.go)
   - Developer B: User Story 2 (dps.go)
3. After P1 stories complete:
   - Developer A: User Story 5 (extends query.go)
   - Developer B: User Story 3 (extends dps.go)
   - Developer C: User Story 4 (extends status.go)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- US1 is the MVP - delivers core NFS-e query capability
- US2-US5 add recovery, verification, and lifecycle features
