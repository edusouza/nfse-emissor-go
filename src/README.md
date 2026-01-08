# NFS-e Emission API

Backend REST API for NFS-e (Brazilian electronic service invoice) emission using the Sistema Nacional NFS-e.

## Overview

This API provides a streamlined interface for emitting electronic service invoices (NFS-e) through Brazil's national system. It handles:

- DPS (Declaracao Prestacao Servicos) submission
- Pre-signed XML submission for integrators with their own signing
- Asynchronous processing with webhook notifications
- Status tracking and query
- Government error translation

## Quick Start

### Prerequisites

- Go 1.21+
- Docker and Docker Compose
- MongoDB 7+
- Redis 7+

### Development Setup

1. **Clone and navigate to the source directory**

```bash
cd nfse-nacional/src
```

2. **Copy environment configuration**

```bash
cp .env.example .env
```

3. **Start infrastructure services**

```bash
docker compose up -d mongodb redis
```

4. **Run the API server**

```bash
go run ./cmd/api
```

5. **Run the worker (separate terminal)**

```bash
go run ./cmd/worker
```

The API will be available at `http://localhost:8080`.

### Docker Deployment

To run the entire stack with Docker:

```bash
docker compose up -d
```

This starts:
- MongoDB on port 27017
- Redis on port 6379
- API server on port 8080
- Worker process

## API Endpoints

### Public Endpoints (No Authentication)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Full health check with component status |
| GET | `/health/live` | Liveness probe (Kubernetes) |
| GET | `/health/ready` | Readiness probe (Kubernetes) |
| GET | `/metrics` | Prometheus metrics |

### Protected Endpoints (Require X-API-Key header)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/v1/nfse` | Submit emission request (JSON) |
| POST | `/v1/nfse/xml` | Submit pre-signed XML |
| GET | `/v1/nfse/status/:requestId` | Query emission status |
| GET | `/v1/nfse/status` | List emission statuses |

## Authentication

All `/v1/*` endpoints require an API key in the `X-API-Key` header:

```bash
curl -H "X-API-Key: your-api-key" http://localhost:8080/v1/nfse/status
```

### Creating an API Key (Development)

```bash
mongosh nfse --eval '
db.api_keys.insertOne({
  key_hash: "your-sha256-hash",
  key_prefix: "nfse_dev_",
  integrator_name: "Development",
  webhook_url: "https://webhook.site/your-id",
  webhook_secret: "your-webhook-secret",
  environment: "homologation",
  rate_limit: { requests_per_minute: 100, burst: 20 },
  active: true,
  created_at: new Date(),
  updated_at: new Date()
})'
```

## Usage Examples

### Submit Emission Request

```bash
curl -X POST http://localhost:8080/v1/nfse \
  -H "Content-Type: application/json" \
  -H "X-API-Key: your-api-key" \
  -d '{
    "provider": {
      "cnpj": "12345678000199",
      "tax_regime": "simples_nacional",
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
      "pfx_base64": "<base64-encoded-pfx>",
      "password": "certificate-password"
    }
  }'
```

Response:
```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending",
  "message": "Request queued for processing",
  "status_url": "http://localhost:8080/v1/nfse/status/550e8400-e29b-41d4-a716-446655440000"
}
```

### Check Status

```bash
curl http://localhost:8080/v1/nfse/status/550e8400-e29b-41d4-a716-446655440000 \
  -H "X-API-Key: your-api-key"
```

### Submit Pre-Signed XML

```bash
curl -X POST http://localhost:8080/v1/nfse/xml \
  -H "Content-Type: application/xml" \
  -H "X-API-Key: your-api-key" \
  -d '<?xml version="1.0" encoding="UTF-8"?>
<DPS xmlns="http://www.sped.fazenda.gov.br/nfse">
  <!-- Your signed DPS XML -->
</DPS>'
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | API server port |
| `ENV` | `development` | Environment (development/staging/production) |
| `MONGODB_URI` | `mongodb://localhost:27017` | MongoDB connection URI |
| `MONGODB_DATABASE` | `nfse` | MongoDB database name |
| `REDIS_URL` | `redis://localhost:6379` | Redis connection URL |
| `SEFIN_API_URL` | `https://hom.nfse.gov.br/api` | Government API URL |
| `SEFIN_ENVIRONMENT` | `homologacao` | Government environment (producao/homologacao) |
| `SEFIN_TIMEOUT` | `30` | Government API timeout (seconds) |
| `LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |
| `LOG_FORMAT` | `json` | Log format (json/text) |
| `WORKER_CONCURRENCY` | `10` | Number of concurrent worker jobs |
| `WORKER_MAX_RETRIES` | `3` | Maximum job retry attempts |
| `RATE_LIMIT_DEFAULT_RPM` | `100` | Default requests per minute |
| `RATE_LIMIT_BURST` | `20` | Rate limit burst size |
| `CERT_PATH` | - | Path to certificate file (optional) |
| `CERT_PASSWORD` | - | Certificate password (optional) |
| `CORS_ORIGINS` | `http://localhost:3000,http://localhost:8080` | Allowed CORS origins |

## Architecture

```
src/
├── cmd/
│   ├── api/main.go          # HTTP server entry point
│   └── worker/main.go       # Job worker entry point
├── internal/
│   ├── api/                  # HTTP layer
│   │   ├── handlers/         # Request handlers
│   │   ├── middleware/       # Auth, logging, rate limiting
│   │   └── routes.go         # Route definitions
│   ├── config/               # Configuration loading
│   ├── domain/               # Business logic
│   │   ├── emission/         # Emission domain
│   │   └── validation/       # Input validation
│   ├── infrastructure/       # External services
│   │   ├── mongodb/          # MongoDB repositories
│   │   ├── redis/            # Redis client & queue
│   │   ├── sefin/            # Government API client
│   │   ├── webhook/          # Webhook delivery
│   │   └── xmlsigner/        # XML digital signature
│   └── jobs/                 # Async job handlers
└── pkg/                      # Shared utilities
    ├── cnpjcpf/              # CNPJ/CPF validation
    └── xmlbuilder/           # XML construction
```

## Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific package tests
go test ./internal/domain/...

# Run with verbose output
go test -v ./...
```

## Monitoring

### Prometheus Metrics

The `/metrics` endpoint exposes Prometheus-format metrics:

- `nfse_api_requests_total` - Total API requests by method/endpoint/status
- `nfse_api_request_duration_seconds` - Request latency histogram
- `nfse_emission_total` - Total emissions by status/environment
- `nfse_emission_duration_seconds` - Emission processing time
- `nfse_queue_depth` - Pending jobs in queue
- `nfse_sefin_requests_total` - Government API requests
- `nfse_sefin_latency_seconds` - Government API latency
- `nfse_webhook_deliveries_total` - Webhook delivery attempts
- `nfse_api_rate_limit_hits_total` - Rate limit hits

### Health Checks

```bash
# Full health check
curl http://localhost:8080/health

# Kubernetes liveness probe
curl http://localhost:8080/health/live

# Kubernetes readiness probe
curl http://localhost:8080/health/ready
```

## Error Handling

The API follows RFC 7807 Problem Details format for errors:

```json
{
  "type": "https://api.nfse.gov.br/problems/validation-error",
  "title": "Validation Error",
  "status": 400,
  "detail": "Invalid CNPJ format",
  "instance": "/v1/nfse"
}
```

### Government Error Codes

The API translates government rejection codes to user-friendly messages. Common codes:

| Code | Description | Action |
|------|-------------|--------|
| E001 | CNPJ not found | Verify provider registration |
| E002 | Invalid CPF | Check CPF format |
| E003 | Duplicate DPS | Use unique series/number |
| E004 | Invalid certificate | Check certificate validity |
| E005 | Malformed XML | Validate against XSD schema |
| E006 | Invalid signature | Regenerate XMLDSig |
| E007 | Invalid service code | Use valid cTribNac code |

## Webhook Notifications

Successful emissions trigger webhook delivery:

```json
{
  "event": "emission.success",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2026-01-08T14:30:00Z",
  "data": {
    "nfse_access_key": "NFSe35...",
    "nfse_number": "1",
    "nfse_xml": "<?xml version=\"1.0\"?>..."
  }
}
```

Verify webhook signatures using HMAC-SHA256:

```python
import hmac
import hashlib

def verify_webhook(body: bytes, signature: str, secret: str) -> bool:
    expected = "sha256=" + hmac.new(
        secret.encode(),
        body,
        hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(signature, expected)
```

## Security Considerations

1. **API Keys**: Always use HTTPS in production. API keys are hashed with SHA-256.
2. **Certificates**: PFX certificates are never persisted; used only in memory.
3. **Rate Limiting**: Default 100 req/min per API key to prevent abuse.
4. **Input Validation**: All inputs validated against schemas before processing.
5. **Audit Logging**: All requests logged with correlation IDs.

## Common Issues

### Certificate Errors
- Ensure PFX is a valid A1 certificate
- Check certificate password
- Verify certificate has NFS-e signing permissions
- Certificate must match provider CNPJ

### Government API Timeout
- Default timeout is 30 seconds
- Worker retries 3 times with exponential backoff
- Check SEFIN_API_URL matches your environment

### Rate Limit Exceeded
- Response includes `Retry-After` header
- Configure per-integrator limits in `api_keys` collection
- Contact support for limit increases

## Development

### Building

```bash
# Build API
go build -o bin/api ./cmd/api

# Build worker
go build -o bin/worker ./cmd/worker
```

### Code Quality

```bash
# Format code
go fmt ./...

# Run linter
golangci-lint run

# Check for vulnerabilities
govulncheck ./...
```

## License

Copyright (c) 2026. All rights reserved.

## Support

For issues and feature requests, please use the issue tracker.
