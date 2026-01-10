# Feature Specification: NFS-e Query and Retrieval API

**Feature Branch**: `002-nfse-query`
**Created**: 2026-01-08
**Status**: Draft
**Input**: User description: "Consulta e recuperação de NFS-e emitidas"

## Context & Scope

This specification covers the **NFS-e query and retrieval functionality** for a B2B backend service. The service allows integrators to retrieve NFS-e documents previously emitted through the Sistema Nacional NFS-e.

**Target Users**: Software integrators who have emitted NFS-e through this service or need to retrieve NFS-e from the national system.

**Relationship to Emission**: This feature complements 001-nfse-emission-core, providing read operations for invoices created via the emission API.

**Key Constraint**: Access to NFS-e data is governed by fiscal secrecy rules. Only authorized actors (Provider, Taker, or Intermediary named in the NFS-e) can access full document details via certificate-based authentication with the government API.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Query NFS-e by Access Key (Priority: P1)

As an integrator, I want to retrieve a complete NFS-e document using its access key so that I can obtain the official XML, verify invoice details, or re-download a previously emitted invoice.

**Why this priority**: This is the primary query method. Access keys are returned after successful emission and are the canonical identifier for NFS-e documents.

**Independent Test**: Can be tested by providing a valid 50-character access key (chaveAcesso). System returns the complete NFS-e XML document with all invoice details.

**Acceptance Scenarios**:

1. **Given** a valid access key for an existing NFS-e, **When** the integrator GETs `/nfse/{chaveAcesso}`, **Then** the system returns HTTP 200 with the complete NFS-e XML including provider, taker, service, and values information.

2. **Given** an access key for a non-existent NFS-e, **When** the integrator GETs `/nfse/{chaveAcesso}`, **Then** the system returns HTTP 404 with a clear error message.

3. **Given** an access key with invalid format (not 50 characters or invalid characters), **When** the integrator GETs `/nfse/{chaveAcesso}`, **Then** the system returns HTTP 400 with validation error details.

4. **Given** a valid access key, **When** the integrator requests the NFS-e, **Then** the response includes NFS-e number, emission date/time, provider details, taker details (if present), service information, tax values, and the complete signed XML.

---

### User Story 2 - Lookup Access Key by DPS Identifier (Priority: P1)

As an integrator, I want to recover the NFS-e access key using the DPS identifier so that I can retrieve invoices even when I've lost or don't have the access key stored locally.

**Why this priority**: Critical for disaster recovery, system migrations, and reconciliation. The DPS ID (composed of municipality code, registration type, federal registration, series, and number) is often known when the access key is not.

**Independent Test**: Can be tested by providing a valid DPS ID. System returns the corresponding NFS-e access key if the requester is an authorized actor (Provider, Taker, or Intermediary).

**Acceptance Scenarios**:

1. **Given** a valid DPS identifier and the requester is the Provider (certificate matches provider CNPJ/CPF), **When** the integrator GETs `/dps/{id}`, **Then** the system returns HTTP 200 with the corresponding NFS-e access key.

2. **Given** a valid DPS identifier and the requester is the Taker named in the NFS-e, **When** the integrator GETs `/dps/{id}`, **Then** the system returns HTTP 200 with the corresponding NFS-e access key.

3. **Given** a valid DPS identifier but the requester is NOT an actor in the NFS-e, **When** the integrator GETs `/dps/{id}`, **Then** the system returns HTTP 403 Forbidden (fiscal secrecy protection).

4. **Given** a DPS identifier that doesn't match any NFS-e, **When** the integrator GETs `/dps/{id}`, **Then** the system returns HTTP 404 with error message.

5. **Given** a DPS identifier with invalid format, **When** the integrator GETs `/dps/{id}`, **Then** the system returns HTTP 400 with validation error details specifying the format requirements.

---

### User Story 3 - Check NFS-e Existence by DPS (Priority: P2)

As an integrator, I want to check if an NFS-e was generated for a given DPS without needing to be an authorized actor so that I can verify emission status before attempting full retrieval.

**Why this priority**: Useful for status checking, duplicate prevention, and pre-validation. Unlike GET, HEAD doesn't require actor authorization, only a valid certificate.

**Independent Test**: Can be tested by providing a valid DPS ID. System returns HTTP 200 if NFS-e exists, HTTP 404 if not.

**Acceptance Scenarios**:

1. **Given** a valid DPS identifier for an existing NFS-e, **When** the integrator sends HEAD `/dps/{id}`, **Then** the system returns HTTP 200 with no body (confirming NFS-e exists).

2. **Given** a valid DPS identifier for a non-existent NFS-e, **When** the integrator sends HEAD `/dps/{id}`, **Then** the system returns HTTP 404.

3. **Given** any DPS identifier and any valid certificate (not necessarily an actor), **When** the integrator sends HEAD `/dps/{id}`, **Then** the request succeeds without fiscal secrecy restrictions.

---

### User Story 4 - Query Request Status (Priority: P2)

As an integrator, I want to check the status of a previously submitted emission request so that I can track progress of pending requests and retrieve results of completed ones.

**Why this priority**: Essential for async flow management. Integrators need to poll status when webhooks fail or for reconciliation.

**Independent Test**: Can be tested by providing a request ID returned from the emission API. System returns current status and results if available.

**Acceptance Scenarios**:

1. **Given** a valid request ID for a pending emission, **When** the integrator GETs `/nfse/status/{requestId}`, **Then** the system returns HTTP 200 with status "pending" or "processing".

2. **Given** a valid request ID for a successful emission, **When** the integrator GETs `/nfse/status/{requestId}`, **Then** the system returns HTTP 200 with status "success", NFS-e access key, NFS-e number, and XML.

3. **Given** a valid request ID for a failed emission, **When** the integrator GETs `/nfse/status/{requestId}`, **Then** the system returns HTTP 200 with status "failed", error code, and error message.

4. **Given** an unknown request ID, **When** the integrator GETs `/nfse/status/{requestId}`, **Then** the system returns HTTP 404.

---

### User Story 5 - Query Events by Access Key (Priority: P3)

As an integrator, I want to retrieve all events linked to an NFS-e so that I can check cancellation status, manifestations, or other events affecting the invoice.

**Why this priority**: Important for invoice lifecycle management, but not required for basic query operations. Events feature depends on separate spec (003-nfse-events).

**Independent Test**: Can be tested by providing an access key for an NFS-e that has linked events. System returns all events.

**Acceptance Scenarios**:

1. **Given** an access key for an NFS-e with linked events, **When** the integrator GETs `/nfse/{chaveAcesso}/eventos`, **Then** the system returns HTTP 200 with a list of all events including type, date, and event XML.

2. **Given** an access key for an NFS-e with no events, **When** the integrator GETs `/nfse/{chaveAcesso}/eventos`, **Then** the system returns HTTP 200 with an empty list.

3. **Given** an access key for a non-existent NFS-e, **When** the integrator GETs `/nfse/{chaveAcesso}/eventos`, **Then** the system returns HTTP 404.

---

### Edge Cases

- What happens when the government API is temporarily unavailable?
  - System returns HTTP 503 Service Unavailable with Retry-After header suggesting when to retry.

- How does the system handle access key queries for NFS-e from non-participating municipalities?
  - System returns the NFS-e if it exists in the national system. If the NFS-e was emitted through a municipal system not integrated with Sistema Nacional, it won't be found (HTTP 404).

- What happens when an integrator queries for a cancelled NFS-e?
  - System returns the NFS-e XML (cancellation doesn't delete the document). The cancellation event can be queried separately via the events endpoint.

- How does the system handle rate limiting on queries?
  - System enforces per-API-key rate limits. Returns HTTP 429 Too Many Requests with Retry-After header when exceeded.

- What if the DPS ID components contain invalid characters?
  - System validates format before calling government API. Returns HTTP 400 with specific field errors.

## Requirements *(mandatory)*

### Functional Requirements

**Authentication & Rate Limiting**:
- **FR-001**: System MUST authenticate integrators via API Key passed in request header
- **FR-002**: System MUST enforce per-API-key rate limits for query operations (configurable, default 200 requests/minute)
- **FR-003**: System MUST return HTTP 429 Too Many Requests with Retry-After header when rate limit exceeded

**Query by Access Key (GET /nfse/{chaveAcesso})**:
- **FR-004**: System MUST accept 50-character access key as path parameter
- **FR-005**: System MUST validate access key format (50 characters, alphanumeric, starts with "NFSe" prefix)
- **FR-006**: System MUST forward request to Sefin Nacional API with appropriate authentication
- **FR-007**: System MUST return complete NFS-e XML preserving government schema and signature
- **FR-008**: System MUST return structured JSON response containing NFS-e number, emission date, status, provider info, taker info, service details, values, and the raw XML

**Lookup by DPS ID (GET /dps/{id})**:
- **FR-009**: System MUST accept DPS identifier as path parameter in format: MunCode(7) + RegType(1) + FedReg(14) + Series(5) + Number(15) = 42 characters
- **FR-010**: System MUST validate DPS ID format and components
- **FR-011**: System MUST require digital certificate for this operation, provided per-request (certificate file + password), consistent with emission API
- **FR-012**: System MUST return NFS-e access key when requester is an authorized actor
- **FR-013**: System MUST return HTTP 403 when requester is not an actor in the NFS-e

**Check Existence (HEAD /dps/{id})**:
- **FR-014**: System MUST accept DPS identifier as path parameter
- **FR-015**: System MUST return HTTP 200 with no body when NFS-e exists
- **FR-016**: System MUST return HTTP 404 when NFS-e does not exist
- **FR-017**: System MUST accept any valid certificate provided per-request (no actor restriction)

**Request Status (GET /nfse/status/{requestId})**:
- **FR-018**: System MUST return current status of emission request
- **FR-019**: System MUST include NFS-e details when status is "success"
- **FR-020**: System MUST include error details when status is "failed"
- **FR-021**: System MUST retain request status for configurable period (default 30 days)

**Events Query (GET /nfse/{chaveAcesso}/eventos)**:
- **FR-022**: System MUST return all events linked to the NFS-e access key
- **FR-023**: System MUST include event type code, description, date, sequence number, and event XML for each event
- **FR-024**: System MUST support filtering by event type via optional query parameter

**Timeouts & Resilience**:
- **FR-025**: System MUST timeout government API calls after 10 seconds
- **FR-026**: System MUST return HTTP 503 with Retry-After header when government API times out or is unavailable

**Error Handling**:
- **FR-027**: System MUST return structured error responses with error code and human-readable message
- **FR-028**: System MUST translate government error codes to integrator-friendly messages
- **FR-029**: System MUST log all query operations for audit purposes

**Environments**:
- **FR-030**: System MUST support both production (tpAmb=1) and homologation (tpAmb=2) environments
- **FR-031**: System MUST determine environment based on configuration, not per-request parameter

### Key Entities

- **NFSeDocument**: The complete NFS-e document retrieved from the national system. Contains access key (chaveAcesso), NFS-e number (nNFSe), emission date/time (dhProc), provider (emit), taker (toma), service details (serv), values (valores), status code (cStat), and the complete signed XML.

- **DPSIdentifier**: The unique identifier for a DPS document. Composed of municipality code (7 digits), registration type (1 digit), federal registration (14 digits, CPF padded with zeros), series (5 digits), and number (15 digits).

- **AccessKey (chaveAcesso)**: 50-character unique identifier for an NFS-e. Format: code(2) + MunCode(7) + emission(8) + RegType(1) + FedReg(14) + NNFSe(13) + TpEmis(1) + verifyCode(4).

- **RequestStatus**: Tracks emission request state. States: pending, processing, success, failed. Includes request ID, timestamps, NFS-e details (on success), and error details (on failure).

- **NFSeEvent**: An event linked to an NFS-e. Contains event type code (tipoEvento), sequence number (numSeqEvento), event date, and the signed event XML.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Integrators can retrieve any NFS-e by access key within 3 seconds under normal conditions
- **SC-002**: 99% of valid queries return correct results (matching government system data)
- **SC-003**: System correctly enforces fiscal secrecy rules - 100% of unauthorized DPS lookups are rejected
- **SC-004**: Error messages allow integrators to diagnose and fix issues without external documentation in 90% of cases
- **SC-005**: Request status remains queryable for at least 30 days after emission
- **SC-006**: System handles government API unavailability gracefully, returning appropriate error within 30 seconds

## Clarifications

### Session 2026-01-08

- Q: How should integrators provide the digital certificate for DPS operations? → A: Per-request (certificate file + password in request, same as emission)
- Q: What timeout should be used for government API query calls? → A: 10 seconds

## Assumptions

1. **Government API Availability**: Sefin Nacional API is available and responds within expected timeframes for query operations.
2. **Certificate Validity**: When certificate-based authentication is required (DPS lookup), integrators provide valid A1 digital certificates.
3. **Access Key Persistence**: Integrators typically store access keys from emission responses but may need recovery via DPS ID.
4. **Emission via This Service**: Request status queries only work for emissions made through this service (tracked in MongoDB).
5. **Events Dependency**: Event queries depend on 003-nfse-events being implemented for full functionality; basic query structure is included here for completeness.

## Out of Scope (Future Features)

- Batch queries (multiple NFS-e in single request)
- Date range queries (list all NFS-e emitted in a period)
- Provider/Taker-based queries (list all NFS-e for a given CNPJ)
- Full-text search on invoice content
- PDF/DANFSE generation from NFS-e XML
- Caching layer for frequently accessed NFS-e
- Event registration (separate spec: 003-nfse-events)
