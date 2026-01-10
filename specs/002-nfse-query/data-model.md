# Data Model: NFS-e Query and Retrieval API

**Date**: 2026-01-08
**Feature**: 002-nfse-query

## Entities Overview

```
                    ┌─────────────────────┐
                    │   NFS-e Document    │
                    │   (from gov API)    │
                    └──────────┬──────────┘
                               │
           ┌───────────────────┼───────────────────┐
           │                   │                   │
           ▼                   ▼                   ▼
┌─────────────────┐   ┌─────────────────┐   ┌─────────────────┐
│  Access Key     │   │  DPS Identifier │   │   NFS-e Events  │
│  (chaveAcesso)  │   │  (42-char ID)   │   │   (cancellation,│
│                 │   │                 │   │    manifestation)│
└─────────────────┘   └─────────────────┘   └─────────────────┘
                               │
                               ▼
                    ┌─────────────────────┐
                    │  EmissionRequest    │
                    │  (internal status)  │
                    └─────────────────────┘
```

## Core Types

### AccessKey (chaveAcesso)

50-character unique identifier for an NFS-e assigned by the government system.

```typescript
type AccessKey = string  // 50 characters, alphanumeric

// Format: code(2) + MunCode(7) + emission(8) + RegType(1) + FedReg(14) + NNFSe(13) + TpEmis(1) + verifyCode(4)
// Example: "NFSe3550308202601081123456789012300000000000012310"
```

**Validation Rules**:
- Exactly 50 characters
- Alphanumeric only
- MUST start with "NFSe" prefix

### DPSIdentifier

42-character composite identifier for looking up NFS-e by DPS.

```typescript
interface DPSIdentifier {
  // Parsed components
  municipalityCode: string   // 7 digits - IBGE code
  registrationType: "1" | "2"  // 1=CNPJ, 2=CPF
  federalRegistration: string  // 14 digits (CPF padded with 000)
  series: string              // 5 digits
  number: string              // 15 digits
}

// Serialized: MunCode(7) + RegType(1) + FedReg(14) + Series(5) + Number(15) = 42 chars
// Example: "3550308112345678000199000010000000000000001"
```

**Validation Rules**:
- Exactly 42 characters
- All numeric
- Municipality code must be valid IBGE code
- Registration type must be 1 or 2
- If type=2 (CPF), first 3 chars of federalReg must be "000"

### NFSeDocument

Complete NFS-e document retrieved from government API.

```typescript
interface NFSeDocument {
  // Identifiers
  chaveAcesso: string           // 50-char access key
  nNFSe: string                 // NFS-e number (up to 15 digits)

  // Timestamps
  dhProc: string                // Processing date/time (ISO 8601)
  dhEmi: string                 // Emission date/time (ISO 8601)

  // Status
  cStat: string                 // Status code
  ambGer: 1 | 2                 // Environment: 1=prod, 2=homolog

  // Location
  xLocEmi: string               // Emission municipality name
  cLocEmi: string               // Emission municipality IBGE code
  xLocPrestacao: string         // Service location description
  cLocIncid?: string            // Tax incidence municipality
  xLocIncid?: string            // Tax incidence municipality name

  // Provider (Emitter)
  emit: {
    cnpj?: string               // 14 digits
    cpf?: string                // 11 digits
    im?: string                 // Municipal registration
    xNome: string               // Name/Corporate name
    xFant?: string              // Trade name
    enderNac: Address
    fone?: string
    email?: string
  }

  // Service Info (from DPS)
  xTribNac: string              // National tax code description
  xTribMun?: string             // Municipal tax code description

  // Values
  valores: {
    vCalcDR?: number            // Deduction/reduction calc value
    tpBM?: number               // Municipal benefit type
    vCalcBM?: number            // Municipal benefit calc value
    vBC?: number                // Tax base
    pAliqAplic?: number         // Applied rate
    vISSQN?: number             // ISSQN value
    vTotalRet?: number          // Total withholdings
    vLiq: number                // Net value
  }

  // Original DPS (embedded)
  dps: DPSInfo

  // Raw XML
  xmlNFSe: string               // Complete signed XML
}
```

### DPSInfo

DPS information embedded in NFS-e response.

```typescript
interface DPSInfo {
  // Identifiers
  id: string                    // DPS ID (42 chars)
  serie: string                 // 5 digits
  nDPS: string                  // Up to 15 digits

  // Environment and timestamps
  tpAmb: 1 | 2                  // 1=prod, 2=homolog
  dhEmi: string                 // Emission date/time
  dCompet: string               // Competence date (YYYYMMDD)

  // Emitter type
  tpEmit: 1 | 2 | 3             // 1=Provider, 2=Taker, 3=Intermediary
  cLocEmi: string               // Emission municipality

  // Provider
  prest: ProviderInfo

  // Taker (optional)
  toma?: PersonInfo

  // Intermediary (optional)
  interm?: PersonInfo

  // Service
  serv: ServiceInfo

  // Values
  valores: ValuesInfo
}
```

### NFSeEvent

Event linked to an NFS-e (cancellation, manifestation, etc.).

```typescript
interface NFSeEvent {
  tipoEvento: string            // Event type code
  descEvento: string            // Event description
  numSeqEvento: number          // Sequential number (usually 1)
  dhEvento: string              // Event date/time (ISO 8601)

  // Event-specific data
  xMotivo?: string              // Reason (for cancellation)
  xDescManif?: string           // Manifestation description

  // Raw XML
  xmlEvento: string             // Complete signed event XML
}
```

## Request/Response DTOs

### NFSeQueryRequest

No body - access key in path parameter.

```
GET /v1/nfse/{chaveAcesso}
Headers:
  X-API-Key: <api_key>
```

### NFSeQueryResponse

```typescript
interface NFSeQueryResponse {
  // Summary (parsed from XML)
  chave_acesso: string
  numero: string
  data_emissao: string          // ISO 8601
  status: string

  // Provider summary
  prestador: {
    documento: string           // CNPJ or CPF
    nome: string
    municipio: string
  }

  // Taker summary (if present)
  tomador?: {
    documento?: string
    nome: string
  }

  // Service summary
  servico: {
    codigo_nacional: string     // cTribNac
    descricao: string
    local_prestacao: string
  }

  // Values summary
  valores: {
    valor_servico: number
    base_calculo: number
    aliquota?: number
    valor_issqn?: number
    valor_liquido: number
  }

  // Full XML
  xml: string                   // Complete NFS-e XML
}
```

### DPSLookupRequest

Certificate required - DPS ID in path parameter.

```
GET /v1/dps/{id}
Headers:
  X-API-Key: <api_key>
Body (multipart/form-data):
  certificate: <PFX file>
  certificate_password: <password>
```

### DPSLookupResponse

```typescript
interface DPSLookupResponse {
  dps_id: string                // Echo back the DPS ID
  chave_acesso: string          // The NFS-e access key
  nfse_url: string              // URL to query full NFS-e
}
```

### DPSExistsResponse (HEAD)

No body - HTTP status indicates result.

```
HEAD /v1/dps/{id}
Headers:
  X-API-Key: <api_key>
Body (multipart/form-data):
  certificate: <PFX file>
  certificate_password: <password>

Response:
  200 OK - NFS-e exists
  404 Not Found - No NFS-e for this DPS
```

### EventsQueryRequest

No body - access key in path parameter.

```
GET /v1/nfse/{chaveAcesso}/eventos
GET /v1/nfse/{chaveAcesso}/eventos?tipo={tipoEvento}
Headers:
  X-API-Key: <api_key>
```

### EventsQueryResponse

```typescript
interface EventsQueryResponse {
  chave_acesso: string
  total: number
  eventos: Array<{
    tipo: string                // Event type code
    descricao: string           // Event description
    sequencia: number
    data: string                // ISO 8601
    xml: string                 // Event XML
  }>
}
```

### StatusResponse (existing, reused)

```typescript
interface StatusResponse {
  request_id: string
  status: "pending" | "processing" | "success" | "failed"
  created_at: string            // ISO 8601
  updated_at: string

  // Only on success
  result?: {
    nfse_access_key: string
    nfse_number: string
    nfse_xml_url: string
  }

  // Only on failure
  error?: {
    code: string
    message: string
    details?: string
  }
}
```

## Embedded Types

### Address

```typescript
interface Address {
  xLgr: string                  // Street
  nro: string                   // Number
  xCpl?: string                 // Complement
  xBairro: string               // Neighborhood
  cMun: string                  // Municipality IBGE code
  uf?: string                   // State
  cep: string                   // Postal code (8 digits)
}
```

### ProviderInfo

```typescript
interface ProviderInfo {
  cnpj?: string
  cpf?: string
  im?: string                   // Municipal registration
  xNome?: string
  end?: Address
  fone?: string
  email?: string
  regTrib: {
    opSimpNac: 1 | 2 | 3        // 1=Non-optant, 2=MEI, 3=ME/EPP
    regApTribSN?: 1 | 2 | 3     // Apportionment regime
    regEspTrib: number          // Special regime (0-6)
  }
}
```

### PersonInfo

```typescript
interface PersonInfo {
  cnpj?: string
  cpf?: string
  nif?: string                  // Foreign tax ID
  cNaoNIF?: 0 | 1 | 2           // Reason for no NIF
  im?: string
  xNome: string
  end?: Address
  fone?: string
  email?: string
}
```

### ServiceInfo

```typescript
interface ServiceInfo {
  locPrest: {
    cLocPrestacao?: string      // Municipality IBGE code
    cPaisPrestacao?: string     // Country ISO code (for exports)
  }
  cServ: {
    cTribNac: string            // 6-digit national code
    cTribMun?: string           // Municipal code
    xDescServ: string           // Description (up to 2000 chars)
    cNBS?: string               // NBS code
  }
}
```

### ValuesInfo

```typescript
interface ValuesInfo {
  vServPrest: {
    vReceb?: number             // Intermediary received value
    vServ: number               // Service value
  }
  vDescCondIncond?: {
    vDescIncond?: number        // Unconditional discount
    vDescCond?: number          // Conditional discount
  }
  vDedRed?: DeductionInfo
  trib: TributationInfo
}
```

## Error Response

```typescript
interface ErrorResponse {
  error: {
    code: string                // e.g., "INVALID_ACCESS_KEY", "NFSE_NOT_FOUND"
    message: string             // Human-readable message
    details?: string            // Additional context
    field?: string              // For validation errors
  }
}
```

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_ACCESS_KEY` | 400 | Access key format invalid |
| `INVALID_DPS_ID` | 400 | DPS identifier format invalid |
| `NFSE_NOT_FOUND` | 404 | NFS-e not found for given key |
| `DPS_NOT_FOUND` | 404 | No NFS-e exists for DPS ID |
| `FORBIDDEN_ACCESS` | 403 | Requester not authorized actor |
| `CERTIFICATE_REQUIRED` | 400 | Certificate required for operation |
| `CERTIFICATE_INVALID` | 400 | Certificate invalid or expired |
| `RATE_LIMIT_EXCEEDED` | 429 | Too many requests |
| `GOVERNMENT_UNAVAILABLE` | 503 | Government API unavailable |
| `GOVERNMENT_TIMEOUT` | 504 | Government API timeout |

## Validation Rules

| Field | Rule | Error Code |
|-------|------|------------|
| chaveAcesso | 50 chars, alphanumeric, starts with "NFSe" | INVALID_ACCESS_KEY |
| dpsId | 42 chars, all numeric | INVALID_DPS_ID |
| dpsId.municipalityCode | 7 digits, valid IBGE | INVALID_DPS_ID |
| dpsId.registrationType | "1" or "2" | INVALID_DPS_ID |
| certificate | Valid PFX, not expired | CERTIFICATE_INVALID |
| tipoEvento (filter) | Valid event type code | INVALID_EVENT_TYPE |
