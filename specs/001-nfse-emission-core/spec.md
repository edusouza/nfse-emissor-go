# Feature Specification: NFS-e Emission Core API

**Feature Branch**: `001-nfse-emission-core`
**Created**: 2026-01-08
**Status**: Draft
**Input**: User description: "Backend REST API for NFS-e emission focused on SIMPLES NACIONAL service providers. Handles DPS submission, XML generation, digital signature, and NFS-e creation via government API."

## Context & Scope

This specification covers the **core NFS-e emission functionality** for a B2B backend service targeting integrators. The service acts as a middleware between integrators and Brazil's Sistema Nacional NFS-e (Sefin Nacional).

**Target Users**: Software integrators building NFS-e emission capabilities into their systems (ERPs, accounting software, SaaS platforms).

**Tax Regime Focus**: SIMPLES NACIONAL contributors only (MEI and ME/EPP) providing services (not commerce or industry).

**Key Constraint**: The system does NOT store company data. All required information for NFS-e emission must be provided in each request.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Basic NFS-e Emission (Priority: P1)

As an integrator, I want to submit a service invoice request with minimal required fields and receive a valid NFS-e so that my SIMPLES NACIONAL clients can issue compliant electronic invoices.

**Why this priority**: This is the core value proposition. Without basic emission, there's no product.

**Independent Test**: Can be tested by submitting a valid JSON payload with provider (CNPJ, tax regime), taker (CNPJ/CPF, name), service (code, description, location), and value. System returns NFS-e with access key and XML.

**Acceptance Scenarios**:

1. **Given** a valid request with all required fields for a SIMPLES NACIONAL provider (opSimpNac=2 or 3), **When** the integrator POSTs to `/nfse`, **Then** the system returns HTTP 202 Accepted with a request ID for tracking.

2. **Given** a queued request completes successfully, **When** the government returns the NFS-e, **Then** the system calls the integrator's webhook with NFS-e access key (chaveAcesso), NFS-e number, and signed XML.

3. **Given** a request missing required fields, **When** the integrator POSTs to `/nfse`, **Then** the system returns HTTP 400 with clear error messages indicating which fields are missing (synchronous validation, not queued).

4. **Given** a request with invalid CNPJ format, **When** the integrator POSTs to `/nfse`, **Then** the system returns HTTP 400 with validation error specifying the invalid field (synchronous validation, not queued).

5. **Given** a queued request fails after all retry attempts, **When** retries are exhausted, **Then** the system calls the integrator's webhook with error details and government rejection info.

---

### User Story 2 - Service-Signed XML (Priority: P1)

As an integrator, I want to provide my client's digital certificate and have the service handle XML signing so that I don't need to implement complex XML digital signature logic.

**Why this priority**: Digital signature is complex and error-prone. Handling it internally removes a major integration barrier.

**Independent Test**: Submit request with certificate file (PFX/P12) and password. System generates signed XML that passes government schema validation.

**Acceptance Scenarios**:

1. **Given** a valid request with certificate file and password, **When** the integrator POSTs to `/nfse`, **Then** the system signs the DPS XML using the provided certificate before submitting to government.

2. **Given** an invalid or expired certificate, **When** the integrator POSTs to `/nfse`, **Then** the system returns HTTP 400 with certificate error details (expired, invalid password, wrong format).

3. **Given** certificate without NFS-e signing permission, **When** the integrator POSTs to `/nfse`, **Then** the system returns clear error about certificate permissions.

---

### User Story 3 - Pre-Signed XML Submission (Priority: P2)

As an integrator who already has XML signing capabilities, I want to submit a pre-signed DPS XML directly so that I maintain full control over the signing process.

**Why this priority**: Some integrators have existing signing infrastructure. Supporting this mode expands market reach.

**Independent Test**: Submit pre-signed XML. System validates signature and forwards to government without re-signing.

**Acceptance Scenarios**:

1. **Given** a valid pre-signed DPS XML, **When** the integrator POSTs to `/nfse/xml`, **Then** the system validates the signature and submits to government.

2. **Given** XML with invalid signature, **When** the integrator POSTs to `/nfse/xml`, **Then** the system returns HTTP 400 with signature validation error.

3. **Given** XML that doesn't conform to government XSD schema, **When** the integrator POSTs to `/nfse/xml`, **Then** the system returns HTTP 400 with schema validation errors.

---

### User Story 4 - NFS-e with Taker Information (Priority: P2)

As an integrator, I want to include service taker (tomador) information when issuing an NFS-e so that invoices identify who received the service.

**Why this priority**: Most invoices require taker identification. Essential for B2B services.

**Independent Test**: Submit request with taker CNPJ/CPF, name, and address. NFS-e includes complete taker information.

**Acceptance Scenarios**:

1. **Given** a request with taker CNPJ and name, **When** the integrator POSTs to `/nfse`, **Then** the NFS-e includes taker information in the XML.

2. **Given** a request with foreign taker (NIF), **When** the integrator POSTs to `/nfse`, **Then** the system correctly handles foreign tax identification.

3. **Given** a request without taker information (consumer sale), **When** the integrator POSTs to `/nfse`, **Then** the system allows emission without taker for applicable service types.

---

### User Story 5 - NFS-e with Discounts and Deductions (Priority: P3)

As an integrator, I want to apply discounts and deductions to the service value so that the tax base is correctly calculated.

**Why this priority**: Common business need, but not required for MVP.

**Independent Test**: Submit request with unconditional discount. Tax base is calculated as service value minus discount.

**Acceptance Scenarios**:

1. **Given** a request with unconditional discount (vDescIncond), **When** calculating tax base, **Then** discount is subtracted from service value before tax calculation.

2. **Given** a request with conditional discount (vDescCond), **When** calculating tax base, **Then** conditional discount does NOT reduce tax base.

3. **Given** deduction documents (vDedRed), **When** calculating tax base, **Then** deductions are applied according to government rules.

---

### Edge Cases

- What happens when the government returns business rule rejection (not schema error)?
  - System forwards rejection via webhook with government rejection code and message translated to clear guidance.

- How does system handle duplicate DPS number for same provider?
  - Government API rejects duplicate DPS; system forwards rejection via webhook with clear error about duplicate DPS number (integrator responsibility to manage sequences).

- What happens when service value is zero?
  - System validates that service value must be greater than zero.

- How does system handle service location outside Brazil (export)?
  - System accepts foreign country code and applies service export rules (tribISSQN=3).

- What happens when integrator exceeds rate limit?
  - System returns HTTP 429 Too Many Requests with Retry-After header indicating seconds until limit resets.

## Requirements *(mandatory)*

### Functional Requirements

**Authentication & Rate Limiting**:
- **FR-000**: System MUST authenticate integrators via API Key passed in request header
- **FR-000a**: System MUST enforce per-API-key rate limits (configurable, default 100 requests/minute)
- **FR-000b**: System MUST return HTTP 429 Too Many Requests with Retry-After header when rate limit exceeded

**Input/Request Handling**:
- **FR-001**: System MUST accept JSON requests with provider, taker, service, and value information
- **FR-002**: System MUST validate CNPJ/CPF using official digit verification algorithm
- **FR-003**: System MUST validate that provider's tax regime (opSimpNac) is 2 (MEI) or 3 (ME/EPP)
- **FR-004**: System MUST reject requests with fields not applicable to SIMPLES NACIONAL services
- **FR-005**: System MUST accept national service code (cTribNac) as 6-digit code per LC 116/2003

**XML Generation**:
- **FR-006**: System MUST generate DPS XML conforming to government XSD schema (DPS_v1.00.xsd)
- **FR-007**: System MUST generate unique DPS ID following format: DPS + MunCode(7) + RegType(1) + FedReg(14) + Series(5) + Number(15)
- **FR-008**: System MUST require integrator to provide DPS series (5 digits) and DPS number (up to 15 digits) in every request
- **FR-009**: System MUST include UTC timestamp (dhEmi) in ISO 8601 format

**Digital Signature**:
- **FR-010**: System MUST sign DPS XML when certificate file and password are provided
- **FR-011**: System MUST validate pre-signed XML signatures when submitted via XML endpoint
- **FR-012**: System MUST use XMLDSig standard for digital signatures
- **FR-013**: System MUST support PFX/P12 certificate formats

**Async Processing & Queue**:
- **FR-014**: System MUST queue valid requests immediately and return HTTP 202 with request ID
- **FR-015**: System MUST process queued requests asynchronously with exponential backoff retries (up to 3 attempts)
- **FR-016**: System MUST call integrator's registered webhook URL on success or final failure
- **FR-017**: System MUST allow integrators to query request status by request ID (GET /nfse/status/{requestId})

**Government Integration**:
- **FR-018**: System MUST submit signed DPS to Sefin Nacional API (POST /nfse)
- **FR-019**: System MUST support both production (tpAmb=1) and homologation (tpAmb=2) environments
- **FR-020**: System MUST return NFS-e access key (chaveAcesso) from successful emissions via webhook
- **FR-021**: System MUST preserve complete NFS-e XML response from government

**Error Handling**:
- **FR-022**: System MUST return structured error responses with field-level validation errors
- **FR-023**: System MUST translate government rejection codes to human-readable messages
- **FR-024**: System MUST distinguish between client errors (4xx) and government/system errors (5xx)

**SIMPLES NACIONAL Specific**:
- **FR-025**: System MUST set opSimpNac field (2=MEI, 3=ME/EPP) based on provider tax regime
- **FR-026**: System MUST handle regApTribSN (apportionment regime) when provided
- **FR-027**: System MUST support pTotTribSN (unified SN tax rate) for tax transparency

### Key Entities

- **EmissionRequest**: The integrator's JSON payload containing all data needed to emit an NFS-e. Includes provider info, taker info (optional), service details, values, certificate (optional), and webhook URL for async callback.

- **RequestStatus**: Tracks the state of a queued emission request. States: pending, processing, success, failed. Includes request ID, timestamps, and result details.

- **DPS (Declaração de Prestação de Serviço)**: The XML document submitted to government. Generated from EmissionRequest. Contains provider tax registration, service classification, values, and digital signature.

- **NFS-e (Nota Fiscal de Serviço Eletrônica)**: The official electronic invoice returned by government. Contains unique access key, sequential number, validation timestamp, and all DPS data.

- **Provider (Prestador)**: Service provider information including CNPJ/CPF, tax regime (SIMPLES NACIONAL), municipal registration, and contact details.

- **Taker (Tomador)**: Service recipient information including identification (CNPJ/CPF/NIF), name, and address.

- **Service (Serviço)**: Service classification including national tax code (cTribNac), description, location (municipality or country for exports).

- **Values (Valores)**: Financial information including service value, discounts (conditional/unconditional), deductions, and calculated taxes.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Integrators can emit a valid NFS-e with under 10 required fields for basic SIMPLES NACIONAL services
- **SC-002**: 95% of valid requests receive webhook callback within 60 seconds (including retries and government processing)
- **SC-003**: 100% of emitted NFS-e are valid and queryable in the government system
- **SC-004**: Error messages allow integrators to fix issues without consulting external documentation in 90% of cases
- **SC-005**: Integration time for new customers is under 1 day for basic emission flow
- **SC-006**: System correctly rejects 100% of requests with invalid CNPJ/CPF or non-SIMPLES NACIONAL providers

## Assumptions

1. **Government API Availability**: Sefin Nacional API is available and responds within expected timeframes.
2. **Certificate Validity**: Integrators provide valid A1 digital certificates with NFS-e emission permissions.
3. **Municipality Participation**: Target municipalities are already integrated with Sistema Nacional NFS-e.
4. **SIMPLES NACIONAL Registration**: Providers are properly registered in the Simples Nacional system (MEI or ME/EPP).
5. **Service Codes**: National service codes (cTribNac) follow LC 116/2003 structure.

## Clarifications

### Session 2026-01-08

- Q: How do integrators authenticate to the API? → A: API Key in header (simple, stateless, standard for B2B)
- Q: How should the service handle government API failures? → A: Queue immediately, process async with exponential backoff retries, callback when complete
- Q: How should DPS number sequencing be managed (stateless system)? → A: Require integrator to always provide DPS series and number (no auto-generation)
- Q: How should request volume be controlled? → A: Per-API-key rate limit (e.g., 100 requests/minute per integrator)

## Out of Scope (Future Features)

- NFS-e query/retrieval (separate spec: 002-nfse-query)
- Event management - cancellation, manifestation (separate spec: 003-nfse-events)
- Municipal parameters retrieval (separate spec: 004-municipal-parameters)
- Batch emission (multiple NFS-e in single request)
- Dashboard or web interface
- Company data persistence
- User/role management within integrator systems (integrators authenticate via API Key)
