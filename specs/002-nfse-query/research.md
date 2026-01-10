# Research: NFS-e Query and Retrieval API

**Date**: 2026-01-08
**Feature**: 002-nfse-query

## Research Tasks

### 1. Government API Query Methods

**Question**: How does the SEFIN Nacional API support NFS-e queries?

**Findings**:
- **GET /nfse/{chaveAcesso}**: REST endpoint returning NFS-e by access key
- **GET /dps/{id}**: REST endpoint returning access key from DPS identifier
- **HEAD /dps/{id}**: Check existence only (no access key returned)
- **GET /nfse/{chaveAcesso}/eventos**: Retrieve events linked to NFS-e

**Decision**: REST GET (JSON responses)
**Rationale**: Government API uses REST for queries (simpler than SOAP emission). JSON responses are easier to parse and more developer-friendly.
**Alternatives Considered**:
- SOAP wrapper: Rejected - unnecessary complexity for read operations
- GraphQL adapter: Rejected - adds abstraction layer for simple queries

### 2. DPS Identifier Format

**Question**: What is the exact format of DPS identifiers for lookup?

**Findings**:
Per government documentation (03-api-manual-contribuintes-emissor-publico.md):
- **Format**: MunCode(7) + RegType(1) + FedReg(14) + Series(5) + Number(15) = **42 characters**
- **Components**:
  - Municipality IBGE code (7 digits)
  - Registration type (1 digit): 1=CNPJ, 2=CPF
  - Federal registration (14 digits): CNPJ or CPF padded with zeros
  - DPS Series (5 digits)
  - DPS Number (15 digits)

**Decision**: 42-character composite string
**Rationale**: Matches government spec exactly. All components are numeric.
**Alternatives Considered**:
- JSON object with separate fields: Rejected - government expects single path parameter
- URL-encoded segments: Rejected - adds complexity, doesn't match government API

### 3. Certificate Authentication for DPS Lookup

**Question**: How should certificates be provided for DPS operations?

**Findings**:
- Government requires mTLS (client certificate) for DPS lookups
- Fiscal secrecy rules restrict access to authorized actors only
- Existing emission API uses per-request certificate (PFX + password)

**Decision**: Per-request certificate (PFX file + password)
**Rationale**: Consistent with emission API pattern. Maintains stateless design. Different integrators can use different certificates.
**Alternatives Considered**:
- Pre-registered certificates: Rejected - reduces flexibility, adds state management
- OAuth tokens: Rejected - government doesn't support, certificate required for fiscal secrecy

### 4. Response Caching Strategy

**Question**: Should query responses be cached?

**Findings**:
- NFS-e documents are immutable once issued
- Events can be added at any time (cancellation, manifestation)
- Request status changes over time
- Government API is authoritative source

**Decision**: No caching (deferred to future feature)
**Rationale**: Out of scope per spec. Government data is authoritative. Adding caching requires invalidation strategy for events.
**Alternatives Considered**:
- In-memory TTL cache: Rejected - may return stale event data
- Redis cache: Rejected - adds complexity for unclear benefit
- CDN layer: Rejected - better suited for high-volume scenarios (future)

### 5. Error Code Translation

**Question**: How should government error codes be translated?

**Findings**:
- Existing emission API has error translation infrastructure
- Query errors are simpler (404, 403, 400) than emission errors
- Government returns structured error codes

**Decision**: Reuse existing error translation infrastructure
**Rationale**: Consistent error handling across API. Simpler maintenance.
**Alternatives Considered**:
- Separate error handler: Rejected - code duplication
- Pass-through government errors: Rejected - poor developer experience

### 6. NFS-e XML Parsing

**Question**: How to handle NFS-e XML responses?

**Findings**:
- Government returns complete signed XML per XSD schema (NFSe_v1.00.xsd)
- XML contains nested structures (infNFSe, emit, valores, DPS)
- Integrators need both structured data and raw XML

**Decision**: Parse key fields for JSON response, include raw XML
**Rationale**: Provides convenience (parsed fields) and completeness (raw XML). Integrators can choose which to use.
**Alternatives Considered**:
- XML-only response: Rejected - poor developer experience for JSON API consumers
- Full XML-to-JSON conversion: Rejected - lossy for complex nested structures

### 7. Rate Limiting Configuration

**Question**: What rate limits for query operations?

**Findings**:
- Emission API uses 100 requests/minute default
- Queries are lighter operations than emissions
- Government API has its own rate limits

**Decision**: 200 requests/minute default for queries
**Rationale**: Higher than emission (read vs write operations). Still respects government limits.
**Alternatives Considered**:
- Same as emission (100/min): Rejected - unnecessarily restrictive for reads
- Unlimited: Rejected - could overwhelm government API

## Technology Decisions Summary

| Component | Decision | Rationale |
|-----------|----------|-----------|
| Query Transport | REST/JSON | Government API uses REST for queries |
| DPS ID Format | 42-char string | Exact government specification |
| Certificate | Per-request PFX | Consistent with emission, stateless |
| Caching | None | Out of scope, government authoritative |
| Error Handling | Reuse existing | Consistency, simplicity |
| XML Handling | Parse + preserve raw | Balance of convenience and completeness |
| Rate Limit | 200 req/min | Higher for read operations |

## Implementation Notes

### SEFIN Client Extension

The existing `SefinClient` interface needs extension:

```go
type SefinClient interface {
    // Existing
    SubmitDPS(ctx context.Context, dpsXML string, env string) (*SefinResponse, error)

    // New query methods
    QueryNFSe(ctx context.Context, chaveAcesso string, cert *tls.Certificate) (*NFSeQueryResponse, error)
    LookupDPS(ctx context.Context, dpsID string, cert *tls.Certificate) (*DPSLookupResponse, error)
    CheckDPSExists(ctx context.Context, dpsID string, cert *tls.Certificate) (bool, error)
    QueryEvents(ctx context.Context, chaveAcesso string, cert *tls.Certificate) (*EventsResponse, error)
}
```

### DPS ID Validation

New package for DPS identifier handling:

```go
package dpsid

type DPSIdentifier struct {
    MunicipalityCode string // 7 digits (IBGE)
    RegistrationType string // 1 digit (1=CNPJ, 2=CPF)
    FederalReg       string // 14 digits (padded)
    Series           string // 5 digits
    Number           string // 15 digits
}

func Parse(id string) (*DPSIdentifier, error)
func (d *DPSIdentifier) String() string
func (d *DPSIdentifier) Validate() error
```

### Government API Endpoints

Based on documentation research:

| Operation | URL Pattern | Auth |
|-----------|-------------|------|
| Query NFS-e | `GET /nfse/{chaveAcesso}` | API Key |
| Lookup DPS | `GET /dps/{id}` | mTLS Certificate |
| Check DPS | `HEAD /dps/{id}` | mTLS Certificate |
| Query Events | `GET /nfse/{chaveAcesso}/eventos` | API Key |
