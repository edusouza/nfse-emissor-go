# Data Model: NFS-e Emission Core API

**Date**: 2026-01-08
**Feature**: 001-nfse-emission-core

## Entities Overview

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│    APIKey       │────▶│ EmissionRequest  │────▶│  WebhookDelivery│
└─────────────────┘     └──────────────────┘     └─────────────────┘
                               │
                               ▼
                        ┌──────────────────┐
                        │   DPS (XML)      │
                        └──────────────────┘
                               │
                               ▼
                        ┌──────────────────┐
                        │  NFS-e (XML)     │
                        └──────────────────┘
```

## MongoDB Collections

### 1. api_keys

Stores integrator API credentials and configuration.

```typescript
{
  _id: ObjectId,
  key_hash: string,           // SHA-256 hash of API key
  key_prefix: string,         // First 8 chars for identification (e.g., "nfse_live_")
  integrator_name: string,    // Display name
  webhook_url: string,        // Default callback URL
  webhook_secret: string,     // HMAC secret for webhook signatures
  environment: "production" | "homologation",
  rate_limit: {
    requests_per_minute: number,  // Default: 100
    burst: number                 // Default: 10
  },
  active: boolean,
  created_at: Date,
  updated_at: Date
}
```

**Indexes**:
- `key_hash`: unique
- `key_prefix`: for lookup/display

### 2. emission_requests

Tracks emission request lifecycle.

```typescript
{
  _id: ObjectId,
  request_id: string,         // UUID v4, returned to integrator
  api_key_id: ObjectId,       // Reference to api_keys

  // Status tracking
  status: "pending" | "processing" | "success" | "failed",
  created_at: Date,
  updated_at: Date,
  processed_at: Date | null,

  // Request payload (sanitized - no certificate data)
  environment: "production" | "homologation",
  provider: {
    cnpj: string,             // 14 digits
    tax_regime: 2 | 3,        // 2=MEI, 3=ME/EPP
    municipal_registration: string | null,
    name: string,
    address: Address | null
  },
  taker: {
    type: "cnpj" | "cpf" | "nif" | "none",
    document: string | null,
    name: string | null,
    address: Address | null
  } | null,
  service: {
    national_code: string,    // cTribNac (6 digits)
    municipal_code: string | null,
    description: string,      // Up to 2000 chars
    location: {
      type: "municipality" | "foreign",
      code: string            // IBGE code or ISO country
    }
  },
  values: {
    service_value: Decimal128,
    unconditional_discount: Decimal128 | null,
    conditional_discount: Decimal128 | null,
    deductions: Decimal128 | null
  },
  dps: {
    series: string,           // 5 digits
    number: string            // Up to 15 digits
  },

  // Processing details
  retry_count: number,
  last_error: string | null,

  // Results (populated on success)
  result: {
    nfse_access_key: string,  // chaveAcesso (50 chars)
    nfse_number: string,
    nfse_xml: string,         // Complete XML response
    processed_at: Date
  } | null,

  // Government rejection (populated on failure)
  rejection: {
    code: string,
    message: string,
    details: string | null
  } | null
}
```

**Indexes**:
- `request_id`: unique
- `api_key_id, created_at`: for listing requests
- `status, created_at`: for queue processing
- `provider.cnpj, dps.series, dps.number`: for duplicate detection
- TTL index on `created_at`: Auto-delete after 90 days

### 3. webhook_deliveries

Tracks webhook delivery attempts.

```typescript
{
  _id: ObjectId,
  request_id: string,         // Reference to emission_request
  api_key_id: ObjectId,

  webhook_url: string,
  attempt: number,            // 1, 2, 3

  status: "pending" | "success" | "failed",

  // Request details
  payload_hash: string,       // SHA-256 of payload for verification

  // Response details
  response_status: number | null,
  response_body: string | null,  // Truncated to 1000 chars
  error: string | null,

  created_at: Date,
  delivered_at: Date | null
}
```

**Indexes**:
- `request_id, attempt`: unique
- `status, created_at`: for retry processing
- TTL index on `created_at`: Auto-delete after 30 days

## Embedded Types

### Address

```typescript
{
  street: string,
  number: string,
  complement: string | null,
  neighborhood: string,
  municipality_code: string,  // IBGE 7 digits
  postal_code: string,        // CEP 8 digits
  country_code: string | null // ISO 2-letter for foreign
}
```

## Request/Response DTOs

### EmissionRequest (API Input)

```typescript
{
  // Required
  provider: {
    cnpj: string,             // 14 digits, no formatting
    tax_regime: "mei" | "me_epp",
    name: string,
    municipal_registration?: string
  },
  service: {
    national_code: string,    // 6 digits
    description: string,
    municipality_code: string // IBGE 7 digits (or country_code for export)
  },
  values: {
    service_value: number     // Decimal, e.g., 1500.00
  },
  dps: {
    series: string,           // 5 digits
    number: string            // Up to 15 digits
  },

  // Optional
  taker?: {
    cnpj?: string,
    cpf?: string,
    nif?: string,             // Foreign tax ID
    name: string,
    address?: Address
  },
  discounts?: {
    unconditional?: number,
    conditional?: number
  },
  deductions?: number,

  // Certificate (for service-signed mode)
  certificate?: {
    pfx_base64: string,       // Base64-encoded PFX file
    password: string
  },

  // Webhook override
  webhook_url?: string        // Override default from API key
}
```

### EmissionResponse (API Output - 202 Accepted)

```typescript
{
  request_id: string,
  status: "pending",
  message: "Request queued for processing",
  status_url: string          // GET /v1/nfse/status/{request_id}
}
```

### StatusResponse (GET /v1/nfse/status/{request_id})

```typescript
{
  request_id: string,
  status: "pending" | "processing" | "success" | "failed",
  created_at: string,         // ISO 8601
  updated_at: string,

  // Only on success
  result?: {
    nfse_access_key: string,
    nfse_number: string,
    nfse_xml_url: string      // Temporary signed URL to download XML
  },

  // Only on failure
  error?: {
    code: string,
    message: string,
    details?: string
  }
}
```

### WebhookPayload (POST to integrator)

```typescript
{
  event: "emission.success" | "emission.failed",
  request_id: string,
  timestamp: string,          // ISO 8601

  // On success
  data?: {
    nfse_access_key: string,
    nfse_number: string,
    nfse_xml: string          // Complete XML
  },

  // On failure
  error?: {
    code: string,
    message: string,
    government_code?: string,
    details?: string
  }
}

// Headers
X-Webhook-Signature: sha256=<HMAC-SHA256 of body with webhook_secret>
X-Request-ID: <request_id>
```

## State Transitions

### EmissionRequest Status

```
                    ┌───────────────────┐
                    │     pending       │
                    └─────────┬─────────┘
                              │ Worker picks up
                              ▼
                    ┌───────────────────┐
           ┌───────│    processing     │───────┐
           │       └───────────────────┘       │
           │ Success                     Fail  │
           ▼                                   ▼
┌───────────────────┐               ┌───────────────────┐
│     success       │               │  (retry if < 3)   │
└───────────────────┘               └─────────┬─────────┘
                                              │ Max retries
                                              ▼
                                    ┌───────────────────┐
                                    │      failed       │
                                    └───────────────────┘
```

## Validation Rules

| Field | Rule | Error Code |
|-------|------|------------|
| provider.cnpj | Valid CNPJ (14 digits + check digit) | INVALID_CNPJ |
| provider.tax_regime | Must be "mei" or "me_epp" | INVALID_TAX_REGIME |
| taker.cpf | Valid CPF (11 digits + check digit) | INVALID_CPF |
| service.national_code | 6 digits, valid LC 116/2003 code | INVALID_SERVICE_CODE |
| service.municipality_code | 7 digits, valid IBGE code | INVALID_MUNICIPALITY |
| values.service_value | > 0, max 2 decimal places | INVALID_VALUE |
| dps.series | 5 digits | INVALID_DPS_SERIES |
| dps.number | 1-15 digits | INVALID_DPS_NUMBER |
| certificate.pfx_base64 | Valid base64, valid PFX format | INVALID_CERTIFICATE |
