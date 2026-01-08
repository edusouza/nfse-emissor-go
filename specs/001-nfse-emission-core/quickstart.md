# Quickstart: NFS-e Emission API

## Prerequisites

- Go 1.21+
- Docker & Docker Compose (for MongoDB + Redis)
- Make (optional, for convenience commands)

## Local Development Setup

### 1. Clone and navigate to source

```bash
cd nfse-nacional/src
```

### 2. Start infrastructure

```bash
# Start MongoDB and Redis
docker compose up -d mongodb redis
```

Or manually:
```bash
docker run -d --name mongodb -p 27017:27017 -e MONGO_INITDB_ROOT_USERNAME=admin -e MONGO_INITDB_ROOT_PASSWORD=admin123 mongo:7
docker run -d --name redis -p 6379:6379 redis:7-alpine
```

### 3. Configure environment

```bash
cp .env.example .env
# Edit .env with your settings
```

Required variables:
```env
# Server
PORT=8080
ENV=development

# MongoDB
MONGO_URI=mongodb://admin:admin123@localhost:27017
MONGO_DATABASE=nfse

# Redis
REDIS_URL=redis://localhost:6379

# Government API (homologation)
SEFIN_API_URL=https://hom.nfse.gov.br/api
SEFIN_ENVIRONMENT=homologacao

# Logging
LOG_LEVEL=debug
LOG_FORMAT=json
```

### 4. Run the services

```bash
# Terminal 1: API server (from src/ directory)
go run ./cmd/api

# Terminal 2: Worker (from src/ directory)
go run ./cmd/worker
```

## API Usage

### Create an API key (development)

```bash
# Using MongoDB shell
mongosh nfse --eval '
db.api_keys.insertOne({
  key_hash: "dev-key-hash",
  key_prefix: "nfse_dev_",
  integrator_name: "Development",
  webhook_url: "https://webhook.site/your-id",
  webhook_secret: "your-secret",
  environment: "homologation",
  rate_limit: { requests_per_minute: 100, burst: 10 },
  active: true,
  created_at: new Date(),
  updated_at: new Date()
})'
```

### Submit an emission request

```bash
curl -X POST http://localhost:8080/v1/nfse \
  -H "Content-Type: application/json" \
  -H "X-API-Key: nfse_dev_your-key" \
  -d '{
    "provider": {
      "cnpj": "12345678000199",
      "tax_regime": "mei",
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
      "password": "your-password"
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

### Check request status

```bash
curl http://localhost:8080/v1/nfse/status/550e8400-e29b-41d4-a716-446655440000 \
  -H "X-API-Key: nfse_dev_your-key"
```

### Webhook payload example

Your webhook endpoint will receive:

```json
{
  "event": "emission.success",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2026-01-08T14:30:00Z",
  "data": {
    "nfse_access_key": "NFSe35503080000000012345678000199000010000000001202601123456789012345",
    "nfse_number": "1",
    "nfse_xml": "<?xml version=\"1.0\"?>..."
  }
}
```

Verify signature:
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

## Running Tests

```bash
# Unit tests
go test ./...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Integration tests (requires running infra)
go test -tags=integration ./tests/integration/...
```

## Project Structure

```
src/
├── cmd/
│   ├── api/main.go          # HTTP server entry point
│   └── worker/main.go       # Job worker entry point
├── internal/
│   ├── api/                 # HTTP layer (handlers, middleware, routes)
│   ├── config/              # Configuration loading
│   ├── domain/              # Business logic (emission, validation)
│   ├── infrastructure/      # External services (mongodb, redis, sefin, webhook)
│   └── jobs/                # Async job handlers (emission, webhook)
├── pkg/                     # Shared utilities (cnpjcpf, xmlbuilder)
├── docker-compose.yml       # Infrastructure stack
├── .env.example             # Environment template
└── README.md                # Full documentation
```

## Common Issues

### Certificate errors

- Ensure PFX is valid A1 certificate
- Check password is correct
- Verify certificate has NFS-e signing permissions

### Government API timeout

- Default timeout is 30 seconds
- Check SEFIN_API_URL is correct for your environment
- Worker will retry 3 times with exponential backoff

### Rate limit exceeded

- Default: 100 requests/minute per API key
- Response includes `Retry-After` header
- Contact support to increase limits

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check with component status |
| GET | `/health/live` | Kubernetes liveness probe |
| GET | `/health/ready` | Kubernetes readiness probe |
| GET | `/metrics` | Prometheus metrics |
| POST | `/v1/nfse` | Submit emission request |
| POST | `/v1/nfse/xml` | Submit pre-signed XML |
| GET | `/v1/nfse/status/:requestId` | Query emission status |
| GET | `/v1/nfse/status` | List emission statuses |

## Next Steps

1. Set up production MongoDB and Redis
2. Configure real government API credentials
3. Set up monitoring with Prometheus + Grafana (scrape /metrics endpoint)
4. Configure proper TLS certificates
5. Set up CI/CD pipeline
