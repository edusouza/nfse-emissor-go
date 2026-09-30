# Experimentar sem certificado

Um passo a passo completo para ver o emissor funcionando na sua máquina, sem
precisar de certificado real nem de acesso à Sefin. Os comandos estão em duas
versões: escolha a aba do seu sistema uma vez e as outras acompanham.

Os arquivos de [`exemplos/`](https://github.com/edusouza/nfse-emissor-go/tree/master/exemplos)
descrevem um **MEI de desenvolvimento de software em Curitiba**. Os dados são
fictícios mas consistentes: o CNPJ tem dígitos verificadores válidos, `4106902`
é o código de Curitiba na tabela do IBGE e `010101` é "Análise e
desenvolvimento de sistemas" na lista nacional de serviços —
`nfse servico ver 010101` confirma.

## Antes de começar

=== "Linux e macOS"

    ```bash
    git clone https://github.com/edusouza/nfse-emissor-go.git
    cd nfse-emissor-go
    go build -o nfse ./cmd/nfse
    cd exemplos
    export PATH="$PWD/..:$PATH"
    ```

=== "Windows (PowerShell)"

    ```powershell
    git clone https://github.com/edusouza/nfse-emissor-go.git
    Set-Location nfse-emissor-go
    go build -o nfse.exe .\cmd\nfse
    Set-Location exemplos
    $env:PATH = "$PWD\..;$env:PATH"
    ```

    Funciona tanto no **Windows PowerShell 5.1**, o que já vem no Windows,
    quanto no **PowerShell 7+**. Se o PowerShell recusar rodar o `.ps1` com uma
    mensagem sobre *execution policy*, libere apenas para esta janela:

    ```powershell
    Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
    ```

    Isso vale só para o processo atual e não muda a configuração da máquina.

## 1. Gerar um certificado descartável

Assinar uma DPS exige um certificado. Para experimentar localmente, o script
cria um autoassinado:

=== "Linux e macOS"

    ```bash
    export NFSE_CERT_SENHA='senha-de-teste'
    ./gerar-certificado-teste.sh
    ```

=== "Windows (PowerShell)"

    ```powershell
    $env:NFSE_CERT_SENHA = 'senha-de-teste'
    .\gerar-certificado-teste.ps1
    ```

    O script usa os cmdlets nativos do Windows (`New-SelfSignedCertificate` e
    `Export-PfxCertificate`), então **não é preciso instalar o OpenSSL**. Ele
    também remove o certificado do seu repositório pessoal depois de exportar o
    arquivo, para não acumular lixo de teste em `Cert:\CurrentUser\My`.

!!! warning "Este certificado não serve para emitir de verdade"
    Ele é autoassinado; a Sefin Nacional só aceita um A1 emitido por uma
    Autoridade Certificadora da ICP-Brasil. Para o fluxo local — montar, validar
    e assinar — ele basta.

## 2. Conferir o certificado

```bash
nfse cert info --arquivo certificado-teste.pfx
```

```
Titular                 EMPRESA EXEMPLO LTDA:12345678000195
Emissor                 EMPRESA EXEMPLO LTDA:12345678000195
Valido ate              18/09/2027 13:12
Dias restantes          364
Tamanho da chave        2048 bits

Certificado apto a assinar uma DPS.
```

O comando sai com código diferente de zero se o certificado não puder assinar —
útil para usar em script:

=== "Linux e macOS"

    ```bash
    nfse cert info --arquivo certificado-teste.pfx || exit 1
    ```

=== "Windows (PowerShell)"

    ```powershell
    nfse cert info --arquivo certificado-teste.pfx
    if ($LASTEXITCODE -ne 0) { throw 'certificado inapto' }
    ```

## 3. Conferir a configuração

```bash
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

```bash
nfse emitir --numero 1 --valor 1500 --descricao "Consultoria - agosto/2026"
```

```
DPS DPS410690221234567800019500001000000000000001
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

```bash
nfse emitir --yaml nota-consultoria.yaml
```

```
DPS DPS410690221234567800019500001000000000000002
  Valor          R$ 8500.00
  Servico        Consultoria tecnica em arquitetura de software - agosto/2026
  Assinatura     aplicada
```

Repare que `nota-consultoria.yaml` **não** informa o código do serviço: ele vem
dos `padroes` da configuração. As três camadas se combinam nesta ordem —
`padroes` → arquivo `--yaml` → flags —, cada uma sobrescrevendo a anterior.

## 6. O que acontece se você repetir um número

```bash
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

Sem `--numero`, o `nfse` usa o próximo número da série; veja
[a numeração](../guias/numeracao.md).

## 7. Inspecionar o XML gerado

=== "Linux e macOS"

    ```bash
    xmllint --format notas/*-dps.xml | less    # ou apenas cat
    ```

=== "Windows (PowerShell)"

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

    !!! warning "Só para ver"
        `[xml]` reserializa o documento, mudando espaços em branco e a
        declaração de encoding. XML canônico é sensível a isso: gravar o
        resultado por cima do arquivo assinado **invalida a assinatura**, porque
        os digests foram calculados sobre os bytes originais. Se precisar
        guardar, guarde em outro nome.

Vale olhar a estrutura: `serv/cServ/cTribNac`, `valores/vServPrest/vServ`, e o
bloco `<Signature>` no final. Para gerar sem assinar e comparar:

```bash
nfse emitir --numero 99 --valor 100 --descricao "Sem assinatura" --sem-assinar
```

## 8. Enviar de verdade

Com um A1 da ICP-Brasil, aponte `certificado.arquivo` para ele e acrescente
`--enviar`:

```bash
nfse emitir --numero 1 --valor 1500 --descricao "Consultoria" --enviar
```

Se preferir conferir antes de transmitir, emita sem `--enviar` e mande o
arquivo depois — é o mesmo documento, byte a byte:

```bash
nfse emitir --valor 1500 --descricao "Consultoria"
nfse enviar notas/DPS4106902...-dps.xml
```

Em `producao-restrita` a nota é processada pelo governo mas **não tem valor
fiscal** — é o lugar certo para o primeiro teste. Para emitir com valor fiscal,
mude `ambiente` para `producao` no `nfse.yaml`; o comando vai pedir confirmação
no terminal antes de transmitir.

=== "Linux e macOS"

    O certificado precisa estar em arquivo `.pfx` ou `.p12`.

=== "Windows (PowerShell)"

    Se o seu certificado está instalado no Windows em vez de estar em arquivo,
    exporte-o para `.pfx` primeiro:

    ```powershell
    $cert = Get-ChildItem Cert:\CurrentUser\My |
        Where-Object { $_.Subject -like '*SEU CNPJ*' }

    $senha = Read-Host -AsSecureString -Prompt 'Senha para o arquivo'
    Export-PfxCertificate -Cert $cert -FilePath .\meu-certificado.pfx -Password $senha
    ```

## Limpando

=== "Linux e macOS"

    ```bash
    rm -rf notas certificado-teste.pfx
    ```

=== "Windows (PowerShell)"

    ```powershell
    Remove-Item -Recurse -Force .\notas, .\certificado-teste.pfx
    ```
