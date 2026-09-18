# 0002 — Troca do parser de PKCS#12

**Status:** Aceita
**Data:** 2026-09-18

## Contexto

`ParsePFX` usava `golang.org/x/crypto/pkcs12` para ler o certificado A1. Esse
pacote só decodifica arquivos PKCS#12 protegidos com os algoritmos antigos —
3DES para o conteúdo e SHA-1 no MAC.

Desde o OpenSSL 3.0, o padrão ao exportar um `.pfx` é AES-256-CBC com MAC
SHA-256. Certificados A1 emitidos recentemente vêm nesse formato.

O teste que expôs o problema:

```
modern.pfx  FALHOU: invalid PFX format or incorrect password:
            pkcs12: unknown digest algorithm: 2.16.840.1.101.3.4.2.1
legacy.pfx  OK
```

O OID `2.16.840.1.101.3.4.2.1` é SHA-256.

Duas agravantes. Primeira: a mensagem de erro diz "formato inválido ou senha
incorreta", então o usuário com um certificado perfeitamente válido é levado a
procurar o problema na senha. Segunda: isso inviabilizaria a v0.1 inteira, cujo
único propósito é assinar com o certificado do usuário.

## Decisão

Migrar para `software.sslmate.com/src/go-pkcs12`, e usar `DecodeChain` em vez
de `Decode`.

## Consequências

- Certificados modernos e antigos passam a ser lidos. Há um teste de regressão
  em `internal/infrastructure/xmlsigner/certificate_format_test.go` que gera os
  dois formatos em memória e valida ambos.
- A cadeia de certificados intermediários passa a ser extraída. O código antigo
  tinha um `Chain: nil` com o comentário "chain certificates not extracted",
  o que é um problema para arquivos ICP-Brasil, que trazem a cadeia.
- Uma dependência nova, sem `cgo`. O binário continua único e cross-compilável.

## Aprendizado

`golang.org/x/crypto/pkcs12` é congelado e explicitamente incompleto — a
documentação do próprio pacote avisa que só cobre o subconjunto que os
navegadores geravam na época. Vale desconfiar de qualquer biblioteca `x/` que
não recebe alterações há anos quando o formato que ela lê continua evoluindo.

O problema também mostra o limite de testar só com material que você mesmo
gerou. Os testes existentes criavam o certificado em memória e nunca passavam
por um arquivo `.pfx` de verdade, então o caminho que quebrava para o usuário
real nunca era exercitado.
