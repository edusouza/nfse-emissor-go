# Research: NFS-e Emission Core API

**Date**: 2026-01-08
**Feature**: 001-nfse-emission-core

## Technology Decisions

### 1. Web Framework: Gin

**Decision**: Use Gin as the HTTP framework

**Rationale**:
- High performance with minimal overhead
- Excellent middleware support (auth, rate limiting, logging)
- Strong community and documentation
- JSON binding/validation built-in
- Easy to test with httptest

**Alternatives Considered**:
- Echo: Similar performance, less community adoption
- Chi: More minimal, would need more boilerplate
- Standard library: Too low-level for this project scope

### 2. Job Queue: Asynq (Redis-based)

**Decision**: Use Asynq for async job processing

**Rationale**:
- Native Go library (not a port)
- Redis-backed (already needed for rate limiting)
- Built-in retry with exponential backoff
- Task scheduling and prioritization
- Web UI for monitoring (asynqmon)
- Simple API, well-documented

**Alternatives Considered**:
- Machinery: More complex setup, less active maintenance
- Go-workers: Older, fewer features
- Custom implementation: Unnecessary complexity

### 3. XML Digital Signature: go-xmlsec or signxml approach

**Decision**: Use etree + crypto/x509 for XML construction and signing

**Rationale**:
- Pure Go implementation (no CGO dependency on libxmlsec)
- Full control over XMLDSig envelope construction
- Brazilian fiscal systems use specific canonicalization (C14N)
- PFX/P12 parsing with crypto/x509

**Implementation Notes**:
- Use `github.com/beevik/etree` for XML manipulation
- Implement XMLDSig manually following Brazilian NFS-e specs
- Sign using RSA-SHA1 or RSA-SHA256 per certificate type
- Apply exclusive canonicalization (exc-c14n)

**Alternatives Considered**:
- go-xmlsec: CGO dependency, complex deployment
- russellhaering/goxmldsig: Limited, SAML-focused

### 4. Database: MongoDB

**Decision**: MongoDB for request status and API key storage

**Rationale**:
- Flexible schema for varying request payloads
- Good for document storage (XML responses)
- Native Go driver with good performance
- Easy horizontal scaling if needed
- TTL indexes for automatic cleanup of old requests

**Collections**:
- `api_keys`: Integrator credentials and rate limit config
- `emission_requests`: Request status, timestamps, results
- `webhook_deliveries`: Webhook attempt logs

**Alternatives Considered**:
- PostgreSQL: More rigid schema, JSONB support adequate but less native
- DynamoDB: Vendor lock-in

### 5. Rate Limiting: Redis + go-redis/redis_rate

**Decision**: Token bucket rate limiting with Redis

**Rationale**:
- Distributed rate limiting (works with multiple API instances)
- Precise per-API-key limits
- Redis already in stack for Asynq
- `go-redis/redis_rate` implements GCRA algorithm

**Configuration**:
- Default: 100 requests/minute per API key
- Configurable per integrator in `api_keys` collection
- Return `Retry-After` header with reset time

### 6. Government API Integration

**Decision**: HTTP client with retry logic and circuit breaker

**Rationale**:
- Government API may have variable latency
- Need graceful degradation on outages
- Asynq handles retries for async jobs
- Circuit breaker prevents cascade failures

**Implementation**:
- Use `net/http` with custom transport
- Timeout: 30 seconds per request
- Asynq retry: 3 attempts with exponential backoff (2s, 4s, 8s)
- Log all government interactions for debugging

### 7. XSD Schema Validation

**Decision**: Validate XML against XSD before submission

**Rationale**:
- Catch errors early before government submission
- Better error messages for integrators
- Government rejects invalid XML anyway

**Implementation**:
- Use `github.com/xeipuuv/gojsonschema` pattern but for XML
- Consider `github.com/lestrrat-go/libxml2` (CGO) for strict validation
- Alternative: Build validation rules in Go from XSD (no CGO)

**Recommendation**: Start with Go-native validation of critical fields, add full XSD validation later if needed.

## Best Practices

### API Design

1. **Versioning**: Include `/v1/` prefix for future compatibility
2. **Idempotency**: Accept `X-Idempotency-Key` header to prevent duplicates
3. **Request ID**: Return unique request ID for all async operations
4. **Error Format**: RFC 7807 Problem Details for errors

### Security

1. **API Key**: `X-API-Key` header, SHA-256 hashed in database
2. **Certificate Handling**: Never persist certificates, use in-memory only
3. **Webhook Signature**: Sign webhook payloads with HMAC-SHA256
4. **TLS**: Require HTTPS in production

### Observability

1. **Structured Logging**: JSON format with request_id correlation
2. **Metrics**: Prometheus format (request count, latency, queue depth)
3. **Health Check**: `/health` endpoint for orchestration
4. **Request Tracing**: Propagate trace IDs through async jobs

### Testing Strategy

1. **Unit Tests**: Domain logic, validation, XML generation
2. **Integration Tests**: Full API flow with mocked government API
3. **Contract Tests**: OpenAPI spec validation
4. **Load Tests**: Verify rate limiting and queue behavior

## Open Questions Resolved

| Question | Resolution |
|----------|------------|
| Go XML signing library | Pure Go with etree + crypto/x509 (no CGO) |
| Rate limiter algorithm | Token bucket via go-redis/redis_rate |
| Schema validation approach | Go-native field validation initially |
| Webhook retry strategy | Separate from emission retries, 3 attempts with backoff |

## Dependencies Summary

```go
// go.mod dependencies
github.com/gin-gonic/gin           // HTTP framework
github.com/hibiken/asynq           // Job queue
github.com/go-redis/redis/v9       // Redis client
github.com/go-redis/redis_rate/v10 // Rate limiting
go.mongodb.org/mongo-driver        // MongoDB driver
github.com/beevik/etree            // XML manipulation
github.com/stretchr/testify        // Testing assertions
```

## References

- [Gin Documentation](https://gin-gonic.com/docs/)
- [Asynq Wiki](https://github.com/hibiken/asynq/wiki)
- [Brazilian NFS-e XSD Schemas](../../../docs/schemas/)
- [XMLDSig Specification](https://www.w3.org/TR/xmldsig-core/)
- [RFC 7807 - Problem Details](https://tools.ietf.org/html/rfc7807)
