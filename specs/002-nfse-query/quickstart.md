# Quickstart: NFS-e Query API

**Feature**: 002-nfse-query
**Date**: 2026-01-08

## Overview

The NFS-e Query API allows you to retrieve electronic service invoices (NFS-e) from the Sistema Nacional NFS-e. This API extends the emission API with read operations.

## Prerequisites

- API Key (obtain from admin dashboard)
- For DPS lookup: Digital certificate (PFX/P12 format)

## Authentication

All endpoints require an API key in the `X-API-Key` header:

```bash
curl -H "X-API-Key: your-api-key" https://api.example.com/v1/nfse/{chaveAcesso}
```

## Endpoints

### 1. Query NFS-e by Access Key

**The primary way to retrieve an NFS-e document.**

```bash
GET /v1/nfse/{chaveAcesso}
```

#### Example Request

```bash
curl -X GET \
  -H "X-API-Key: your-api-key" \
  "https://api.example.com/v1/nfse/NFSe3550308202601081123456789012300000000000012310"
```

#### Example Response (200 OK)

```json
{
  "chave_acesso": "NFSe3550308202601081123456789012300000000000012310",
  "numero": "000000123",
  "data_emissao": "2026-01-08T10:30:00-03:00",
  "status": "100",
  "prestador": {
    "documento": "12345678000199",
    "nome": "Empresa Exemplo LTDA",
    "municipio": "São Paulo"
  },
  "tomador": {
    "documento": "98765432000188",
    "nome": "Cliente Exemplo S.A."
  },
  "servico": {
    "codigo_nacional": "010101",
    "descricao": "Serviços de consultoria em tecnologia da informação",
    "local_prestacao": "São Paulo - SP"
  },
  "valores": {
    "valor_servico": 1000.00,
    "base_calculo": 1000.00,
    "aliquota": 5.00,
    "valor_issqn": 50.00,
    "valor_liquido": 950.00
  },
  "xml": "<?xml version=\"1.0\"?>..."
}
```

### 2. Lookup Access Key by DPS ID

**Recover the access key when you only have the DPS identifier.**

> **Note**: Requires digital certificate. Only authorized actors (Provider, Taker, or Intermediary) can access this endpoint.

```bash
GET /v1/dps/{id}
```

#### DPS ID Format

The DPS identifier is a 42-character numeric string:

| Component | Length | Description |
|-----------|--------|-------------|
| Municipality Code | 7 | IBGE code |
| Registration Type | 1 | 1=CNPJ, 2=CPF |
| Federal Registration | 14 | CNPJ or CPF (padded) |
| Series | 5 | DPS series |
| Number | 15 | DPS number |

Example: `3550308112345678000199000010000000000000001`
- Municipality: 3550308 (São Paulo)
- Type: 1 (CNPJ)
- CNPJ: 12345678000199
- Series: 00001
- Number: 000000000000001

#### Example Request

```bash
curl -X GET \
  -H "X-API-Key: your-api-key" \
  -F "certificate=@/path/to/certificate.pfx" \
  -F "certificate_password=your-password" \
  "https://api.example.com/v1/dps/3550308112345678000199000010000000000000001"
```

#### Example Response (200 OK)

```json
{
  "dps_id": "3550308112345678000199000010000000000000001",
  "chave_acesso": "NFSe3550308202601081123456789012300000000000012310",
  "nfse_url": "/v1/nfse/NFSe3550308202601081123456789012300000000000012310"
}
```

### 3. Check NFS-e Existence

**Check if an NFS-e exists without retrieving the access key.**

> **Note**: Requires certificate but no actor restriction.

```bash
HEAD /v1/dps/{id}
```

#### Example Request

```bash
curl -I \
  -H "X-API-Key: your-api-key" \
  -F "certificate=@/path/to/certificate.pfx" \
  -F "certificate_password=your-password" \
  "https://api.example.com/v1/dps/3550308112345678000199000010000000000000001"
```

#### Responses

- `200 OK` - NFS-e exists
- `404 Not Found` - No NFS-e for this DPS

### 4. Query Events

**Retrieve events linked to an NFS-e (cancellations, manifestations, etc.).**

```bash
GET /v1/nfse/{chaveAcesso}/eventos
GET /v1/nfse/{chaveAcesso}/eventos?tipo={tipoEvento}
```

#### Example Request

```bash
curl -X GET \
  -H "X-API-Key: your-api-key" \
  "https://api.example.com/v1/nfse/NFSe3550308202601081123456789012300000000000012310/eventos"
```

#### Example Response (200 OK)

```json
{
  "chave_acesso": "NFSe3550308202601081123456789012300000000000012310",
  "total": 1,
  "eventos": [
    {
      "tipo": "e101101",
      "descricao": "Cancelamento de NFS-e",
      "sequencia": 1,
      "data": "2026-01-08T15:00:00-03:00",
      "xml": "<?xml version=\"1.0\"?>..."
    }
  ]
}
```

### 5. Query Request Status

**Check the status of an emission request.**

```bash
GET /v1/nfse/status/{requestId}
```

#### Example Request

```bash
curl -X GET \
  -H "X-API-Key: your-api-key" \
  "https://api.example.com/v1/nfse/status/550e8400-e29b-41d4-a716-446655440000"
```

#### Example Response - Success

```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "success",
  "created_at": "2026-01-08T10:00:00-03:00",
  "updated_at": "2026-01-08T10:00:05-03:00",
  "result": {
    "nfse_access_key": "NFSe3550308202601081123456789012300000000000012310",
    "nfse_number": "000000123",
    "nfse_xml_url": "/v1/nfse/NFSe3550308202601081123456789012300000000000012310"
  }
}
```

#### Example Response - Failed

```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "failed",
  "created_at": "2026-01-08T10:00:00-03:00",
  "updated_at": "2026-01-08T10:00:05-03:00",
  "error": {
    "code": "E001",
    "message": "CNPJ do prestador não encontrado no cadastro",
    "details": "Provider CNPJ not registered in municipal system"
  }
}
```

## Error Handling

### Error Response Format

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable message",
    "details": "Additional context (optional)",
    "field": "field_name (for validation errors)"
  }
}
```

### Common Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_ACCESS_KEY` | 400 | Access key format invalid |
| `INVALID_DPS_ID` | 400 | DPS identifier format invalid |
| `CERTIFICATE_REQUIRED` | 400 | Certificate required for operation |
| `CERTIFICATE_INVALID` | 400 | Certificate invalid or expired |
| `FORBIDDEN_ACCESS` | 403 | Not authorized actor (fiscal secrecy) |
| `NFSE_NOT_FOUND` | 404 | NFS-e not found |
| `DPS_NOT_FOUND` | 404 | No NFS-e for this DPS |
| `RATE_LIMIT_EXCEEDED` | 429 | Too many requests |
| `GOVERNMENT_UNAVAILABLE` | 503 | Government API unavailable |
| `GOVERNMENT_TIMEOUT` | 504 | Government API timeout |

### Rate Limiting

- Default: 200 requests/minute per API key
- When exceeded: HTTP 429 with `Retry-After` header

```bash
HTTP/1.1 429 Too Many Requests
Retry-After: 30
```

## Code Examples

### Python

```python
import requests

API_KEY = "your-api-key"
BASE_URL = "https://api.example.com/v1"

def query_nfse(chave_acesso):
    response = requests.get(
        f"{BASE_URL}/nfse/{chave_acesso}",
        headers={"X-API-Key": API_KEY}
    )
    response.raise_for_status()
    return response.json()

def lookup_dps(dps_id, cert_path, cert_password):
    with open(cert_path, 'rb') as cert_file:
        response = requests.get(
            f"{BASE_URL}/dps/{dps_id}",
            headers={"X-API-Key": API_KEY},
            files={
                'certificate': cert_file,
                'certificate_password': (None, cert_password)
            }
        )
    response.raise_for_status()
    return response.json()

# Usage
nfse = query_nfse("NFSe3550308202601081123456789012300000000000012310")
print(f"NFS-e #{nfse['numero']}: R$ {nfse['valores']['valor_servico']}")
```

### JavaScript (Node.js)

```javascript
const axios = require('axios');
const FormData = require('form-data');
const fs = require('fs');

const API_KEY = 'your-api-key';
const BASE_URL = 'https://api.example.com/v1';

async function queryNFSe(chaveAcesso) {
  const response = await axios.get(`${BASE_URL}/nfse/${chaveAcesso}`, {
    headers: { 'X-API-Key': API_KEY }
  });
  return response.data;
}

async function lookupDPS(dpsId, certPath, certPassword) {
  const form = new FormData();
  form.append('certificate', fs.createReadStream(certPath));
  form.append('certificate_password', certPassword);

  const response = await axios.get(`${BASE_URL}/dps/${dpsId}`, {
    headers: {
      'X-API-Key': API_KEY,
      ...form.getHeaders()
    },
    data: form
  });
  return response.data;
}

// Usage
queryNFSe('NFSe3550308202601081123456789012300000000000012310')
  .then(nfse => console.log(`NFS-e #${nfse.numero}: R$ ${nfse.valores.valor_servico}`));
```

### Go

```go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
)

const (
    apiKey  = "your-api-key"
    baseURL = "https://api.example.com/v1"
)

type NFSeResponse struct {
    ChaveAcesso string `json:"chave_acesso"`
    Numero      string `json:"numero"`
    Valores     struct {
        ValorServico float64 `json:"valor_servico"`
    } `json:"valores"`
}

func queryNFSe(chaveAcesso string) (*NFSeResponse, error) {
    req, err := http.NewRequest("GET", baseURL+"/nfse/"+chaveAcesso, nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("X-API-Key", apiKey)

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var nfse NFSeResponse
    if err := json.NewDecoder(resp.Body).Decode(&nfse); err != nil {
        return nil, err
    }
    return &nfse, nil
}

func main() {
    nfse, err := queryNFSe("NFSe3550308202601081123456789012300000000000012310")
    if err != nil {
        panic(err)
    }
    fmt.Printf("NFS-e #%s: R$ %.2f\n", nfse.Numero, nfse.Valores.ValorServico)
}
```

## Best Practices

1. **Store access keys**: Always store the `chave_acesso` returned from emissions. DPS lookup is for recovery only.

2. **Handle rate limits gracefully**: Implement exponential backoff when receiving 429 responses.

3. **Cache locally if needed**: The API doesn't cache, so implement client-side caching if you query the same NFS-e frequently.

4. **Validate before calling**: Validate access key format (50 chars, starts with "NFSe") and DPS ID format (42 numeric chars) before making API calls.

5. **Check status for pending emissions**: If you didn't receive the webhook callback, poll the status endpoint with exponential backoff.
