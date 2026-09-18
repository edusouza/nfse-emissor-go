<#
.SYNOPSIS
    Gera um certificado descartavel para experimentar o emissor localmente.

.DESCRIPTION
    ATENCAO: este certificado e autoassinado. Ele serve para exercitar o fluxo
    completo na sua maquina — montar, validar e assinar a DPS. A Sefin Nacional
    NAO o aceita: para enviar de verdade voce precisa de um A1 emitido por uma
    Autoridade Certificadora da ICP-Brasil.

    Usa os cmdlets nativos do Windows, entao nao e preciso instalar o OpenSSL.

.PARAMETER Arquivo
    Caminho do .pfx a criar. Padrao: certificado-teste.pfx

.PARAMETER Senha
    Senha do arquivo. Padrao: o valor de $env:NFSE_CERT_SENHA, ou
    "senha-de-teste".

.EXAMPLE
    $env:NFSE_CERT_SENHA = 'senha-de-teste'
    .\gerar-certificado-teste.ps1
#>

[CmdletBinding()]
param(
    [string] $Arquivo = 'certificado-teste.pfx',
    [string] $Senha = $(if ($env:NFSE_CERT_SENHA) { $env:NFSE_CERT_SENHA } else { 'senha-de-teste' })
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# New-SelfSignedCertificate e Export-PfxCertificate vivem no modulo PKI, que so
# existe no Windows. Em Linux ou macOS use o script .sh ao lado.
#
# A ordem do teste importa: $IsWindows so foi introduzido no PowerShell 6, e o
# Windows PowerShell 5.1 — o que vem instalado no Windows — nao o define. Sob
# Set-StrictMode, le-lo ali lancaria excecao. O -and curto-circuita, entao a
# variavel so e consultada nas versoes em que ela existe.
if ($PSVersionTable.PSVersion.Major -ge 6 -and -not $IsWindows) {
    Write-Error @'
Este script depende de cmdlets exclusivos do Windows (New-SelfSignedCertificate).
Em Linux ou macOS use o equivalente em shell:

    ./gerar-certificado-teste.sh
'@
    exit 1
}

$cnpj = '12345678000195'
$nome = 'EMPRESA EXEMPLO LTDA'

# O titular segue o padrao ICP-Brasil: NOME:CNPJ.
# keyUsage digitalSignature e o que o emissor exige para assinar uma DPS;
# a extensao de texto adiciona o EKU clientAuth (1.3.6.1.5.5.7.3.2), usado no
# TLS mutuo com a Sefin.
$certificado = New-SelfSignedCertificate `
    -Subject "CN=$nome`:$cnpj, O=ICP-Brasil, C=BR" `
    -KeyAlgorithm RSA `
    -KeyLength 2048 `
    -HashAlgorithm SHA256 `
    -KeyUsage DigitalSignature, KeyEncipherment `
    -KeyExportPolicy Exportable `
    -TextExtension @('2.5.29.37={text}1.3.6.1.5.5.7.3.2') `
    -NotAfter (Get-Date).AddYears(1) `
    -CertStoreLocation 'Cert:\CurrentUser\My'

try {
    $senhaSegura = ConvertTo-SecureString -String $Senha -Force -AsPlainText
    Export-PfxCertificate -Cert $certificado -FilePath $Arquivo -Password $senhaSegura | Out-Null
}
finally {
    # New-SelfSignedCertificate deixa o certificado no seu repositorio pessoal.
    # So queriamos o arquivo, entao limpamos para nao acumular lixo de teste.
    Remove-Item -Path "Cert:\CurrentUser\My\$($certificado.Thumbprint)" -Force -ErrorAction SilentlyContinue
}

Write-Host "Certificado de teste criado em $Arquivo"
Write-Host "  Titular  $nome`:$cnpj"
Write-Host "  Senha    $Senha"
Write-Host ''
Write-Host 'Confira com:'
Write-Host "  `$env:NFSE_CERT_SENHA = '$Senha'"
Write-Host "  nfse cert info --arquivo $Arquivo"
