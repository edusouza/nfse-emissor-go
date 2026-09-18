# Exemplo de emissão local — Windows / PowerShell

O mesmo passo a passo do [README.md](README.md), com os comandos em PowerShell.
Se você usa Linux ou macOS, siga aquele.

Os arquivos aqui descrevem um **MEI de desenvolvimento de software em
Curitiba**. Os dados são fictícios mas consistentes: o CNPJ tem dígitos
verificadores válidos, `4106902` é o código de Curitiba na tabela do IBGE
(`docs\anexos\ANEXO_A-...`) e `010101` é "Análise e desenvolvimento de
sistemas" na lista nacional de serviços (`docs\anexos\ANEXO_B-...`).

Funciona tanto no **Windows PowerShell 5.1** (o que já vem no Windows) quanto
no **PowerShell 7+**.

## Antes de começar

```powershell
# a partir da raiz do repositório
go build -o nfse.exe .\cmd\nfse
Set-Location exemplos
$env:PATH = "$PWD\..;$env:PATH"
```

> Se o PowerShell recusar rodar o `.ps1` com uma mensagem sobre *execution
> policy*, libere apenas para esta janela:
>
> ```powershell
> Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
> ```
>
> Isso vale só para o processo atual e não muda a configuração da máquina.

## 1. Gerar um certificado descartável

Assinar uma DPS exige um certificado. Para experimentar localmente:

```powershell
$env:NFSE_CERT_SENHA = 'senha-de-teste'
.\gerar-certificado-teste.ps1
```

O script usa os cmdlets nativos do Windows (`New-SelfSignedCertificate` e
`Export-PfxCertificate`), então **não é preciso instalar o OpenSSL**. Ele também
remove o certificado do seu repositório pessoal depois de exportar o arquivo,
para não acumular lixo de teste em `Cert:\CurrentUser\My`.

> **Este certificado não serve para emitir de verdade.** Ele é autoassinado; a
> Sefin Nacional só aceita um A1 emitido por uma Autoridade Certificadora da
> ICP-Brasil. Para o fluxo local — montar, validar e assinar — ele basta.

## 2. Conferir o certificado

```powershell
nfse cert info --arquivo certificado-teste.pfx
```

```
Titular                 EMPRESA EXEMPLO LTDA:12345678000195
Emissor                 EMPRESA EXEMPLO LTDA:12345678000195
Valido ate              18/09/2027 13:23
Dias restantes          364
Tamanho da chave        2048 bits

Certificado apto a assinar uma DPS.
```

O comando sai com código diferente de zero se o certificado não puder assinar.
Em script, verifique com `$LASTEXITCODE`:

```powershell
nfse cert info --arquivo certificado-teste.pfx
if ($LASTEXITCODE -ne 0) { throw 'certificado inapto' }
```

## 3. Conferir a configuração

```powershell
nfse config check
```

```
nfse.yaml esta valido.

  Ambiente    producao-restrita
  Prestador   EMPRESA EXEMPLO LTDA (12345678000195)
  Regime      mei
  Municipio   4106902
  Serie       00001
```

Se faltar algo, ele lista **tudo** que falta de uma vez, em vez de parar no
primeiro problema.

## 4. Emitir com o mínimo de digitação

Como o `nfse.yaml` já traz código do serviço, alíquota e "tomador não
identificado" na seção `padroes`, uma nota para o público em geral precisa só
de número, valor e descrição:

```powershell
nfse emitir --numero 1 --valor 1500 --descricao "Consultoria - agosto/2026"
```

```
DPS DPS410690211234567800019500001000000000000001
  Ambiente       producao-restrita
  Valor          R$ 1500.00
  Servico        Consultoria - agosto/2026
  Assinatura     aplicada
  Arquivo        notas/DPS4106902...001-dps.xml

Ambiente de producao restrita: esta DPS nao tem valor fiscal.

A DPS foi assinada mas nao enviada. Use --enviar para transmiti-la.
```

## 5. Emitir uma nota com cliente identificado

Para o caso B2B, um arquivo por nota é mais confortável — e versionável:

```powershell
nfse emitir --yaml nota-consultoria.yaml
```

```
DPS DPS410690211234567800019500001000000000000002
  Valor          R$ 8500.00
  Servico        Consultoria tecnica em arquitetura de software - agosto/2026
  Assinatura     aplicada
```

Repare que `nota-consultoria.yaml` **não** informa o código do serviço: ele vem
dos `padroes` da configuração. As três camadas se combinam nesta ordem —
`padroes` → arquivo `--yaml` → flags —, cada uma sobrescrevendo a anterior.

## 6. O que acontece se você repetir um número

```powershell
nfse emitir --numero 1 --valor 99 --descricao "Repetida"
```

```
erro: "notas/DPS4106902...001-dps.xml" ja existe: o numero da DPS
provavelmente ja foi usado.
Use outro --numero, ou --sobrescrever se for mesmo para substituir o arquivo
```

O nome do arquivo vem do identificador da DPS, que se repete quando série e
número se repetem. Sobrescrever em silêncio destruiria uma declaração assinada
— ou, depois do envio, a única cópia local de uma nota que existe no governo.

A numeração automática está na
[issue #8](https://github.com/edusouza/nfse-emissor-go/issues/8).

## 7. Inspecionar o XML gerado

O PowerShell formata XML sem precisar de ferramenta externa:

```powershell
$xml = [xml](Get-Content .\notas\*001-dps.xml -Raw)
$xml.Save([Console]::Out)
```

Ou, para olhar campos específicos:

```powershell
$xml.DPS.infDPS.serv.cServ.cTribNac
$xml.DPS.infDPS.valores.vServPrest.vServ
```

> **Só para ver.** `[xml]` reserializa o documento, mudando espaços em branco e
> a declaração de encoding. XML canônico é sensível a isso: gravar o resultado
> por cima do arquivo assinado **invalida a assinatura**, porque os digests
> foram calculados sobre os bytes originais. Se precisar guardar, guarde em
> outro nome.

Vale conferir também o bloco `<Signature>` no fim do documento. Para gerar sem
assinar e comparar:

```powershell
nfse emitir --numero 99 --valor 100 --descricao "Sem assinatura" --sem-assinar
```

## 8. Enviar de verdade

Com um A1 da ICP-Brasil, aponte `certificado.arquivo` para ele e acrescente
`--enviar`:

```powershell
nfse emitir --numero 1 --valor 1500 --descricao "Consultoria" --enviar
```

Em `producao-restrita` a nota é processada pelo governo mas **não tem valor
fiscal** — é o lugar certo para o primeiro teste. Para emitir com valor fiscal,
mude `ambiente` para `producao` no `nfse.yaml`; o comando vai pedir confirmação
no terminal antes de transmitir.

Se o seu certificado já está instalado no Windows em vez de estar em arquivo,
exporte-o para `.pfx` primeiro:

```powershell
$cert = Get-ChildItem Cert:\CurrentUser\My |
    Where-Object { $_.Subject -like '*SEU CNPJ*' }

$senha = Read-Host -AsSecureString -Prompt 'Senha para o arquivo'
Export-PfxCertificate -Cert $cert -FilePath .\meu-certificado.pfx -Password $senha
```

## Limpando

```powershell
Remove-Item -Recurse -Force .\notas, .\certificado-teste.pfx
```
