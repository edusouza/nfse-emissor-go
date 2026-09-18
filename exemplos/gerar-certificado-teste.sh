#!/usr/bin/env bash
#
# Gera um certificado descartavel para experimentar o emissor localmente.
#
# ATENCAO: este certificado e autoassinado. Ele serve para exercitar o fluxo
# completo na sua maquina — montar, validar e assinar a DPS. A Sefin Nacional
# NAO o aceita: para enviar de verdade voce precisa de um A1 emitido por uma
# Autoridade Certificadora da ICP-Brasil.

set -euo pipefail

ARQUIVO="${1:-certificado-teste.pfx}"
SENHA="${NFSE_CERT_SENHA:-senha-de-teste}"
CNPJ="12345678000195"
NOME="EMPRESA EXEMPLO LTDA"

temporario="$(mktemp -d)"
trap 'rm -rf "$temporario"' EXIT

# keyUsage digitalSignature e o que o emissor exige para assinar uma DPS.
cat > "$temporario/openssl.cnf" <<'CNF'
[req]
distinguished_name = dn
x509_extensions    = ext
prompt             = no

[dn]
CN = EMPRESA EXEMPLO LTDA:12345678000195
O  = ICP-Brasil
C  = BR

[ext]
keyUsage         = critical, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth, emailProtection
basicConstraints = critical, CA:FALSE
CNF

openssl req -x509 -newkey rsa:2048 -sha256 -days 365 -nodes \
    -config "$temporario/openssl.cnf" \
    -keyout "$temporario/chave.pem" \
    -out "$temporario/cert.pem" 2>/dev/null

openssl pkcs12 -export \
    -inkey "$temporario/chave.pem" \
    -in "$temporario/cert.pem" \
    -out "$ARQUIVO" \
    -passout "pass:$SENHA" 2>/dev/null

chmod 600 "$ARQUIVO"

echo "Certificado de teste criado em $ARQUIVO"
echo "  Titular  $NOME:$CNPJ"
echo "  Senha    $SENHA"
echo
echo "Confira com:"
echo "  NFSE_CERT_SENHA='$SENHA' nfse cert info --arquivo $ARQUIVO"
