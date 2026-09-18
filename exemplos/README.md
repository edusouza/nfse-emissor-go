# Exemplo de emissão local

> Usa Windows? Veja [README-windows.md](README-windows.md), com os mesmos
> passos em PowerShell.

Um passo a passo completo para você ver o emissor funcionando na sua máquina,
sem precisar de certificado real nem de acesso à Sefin.

Os arquivos aqui descrevem um **MEI de desenvolvimento de software em
Curitiba**. Os dados são fictícios mas consistentes: o CNPJ tem dígitos
verificadores válidos, `4106902` é o código de Curitiba na tabela do IBGE
(`docs/anexos/ANEXO_A-...`) e `010101` é "Análise e desenvolvimento de
sistemas" na lista nacional de serviços (`docs/anexos/ANEXO_B-...`).

## Antes de começar

```bash
go build -o nfse ./cmd/nfse    # a partir da raiz do repositório
cd exemplos
export PATH="$PWD/..:$PATH"
```

## 1. Gerar um certificado descartável

Assinar uma DPS exige um certificado. Para experimentar localmente, este script
cria um autoassinado:

```bash
export NFSE_CERT_SENHA='senha-de-teste'
./gerar-certificado-teste.sh
```

> **Este certificado não serve para emitir de verdade.** Ele é autoassinado; a
> Sefin Nacional só aceita um A1 emitido por uma Autoridade Certificadora da
> ICP-Brasil. Para o fluxo local — montar, validar e assinar — ele basta.

## 2. Conferir o certificado

```console
$ nfse cert info --arquivo certificado-teste.pfx
Titular                 EMPRESA EXEMPLO LTDA:12345678000195
Emissor                 EMPRESA EXEMPLO LTDA:12345678000195
Valido ate              18/09/2027 13:12
Dias restantes          364
Tamanho da chave        2048 bits

Certificado apto a assinar uma DPS.
```

O comando sai com código diferente de zero se o certificado não puder assinar —
útil para usar em script.

## 3. Conferir a configuração

```console
$ nfse config check
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

```console
$ nfse emitir --numero 1 --valor 1500 --descricao "Consultoria - agosto/2026"
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

```console
$ nfse emitir --yaml nota-consultoria.yaml
DPS DPS410690211234567800019500001000000000000002
  Valor          R$ 8500.00
  Servico        Consultoria tecnica em arquitetura de software - agosto/2026
  Assinatura     aplicada
```

Repare que `nota-consultoria.yaml` **não** informa o código do serviço: ele vem
dos `padroes` da configuração. As três camadas se combinam nesta ordem —
`padroes` → arquivo `--yaml` → flags —, cada uma sobrescrevendo a anterior.

## 6. O que acontece se você repetir um número

```console
$ nfse emitir --numero 1 --valor 99 --descricao "Repetida"
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

```bash
xmllint --format notas/*-dps.xml | less    # ou apenas cat
```

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

## Limpando

```bash
rm -rf notas certificado-teste.pfx
```
