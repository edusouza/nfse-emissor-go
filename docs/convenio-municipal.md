# Como saber se o convênio do município está ativo

Um município só emite NFS-e pelo Sistema Nacional depois de assinar o convênio
de adesão e parametrizar as suas regras no Painel Administrativo Municipal. Até
lá, a Sefin Nacional não gera nota para aquele município — e, mesmo depois, as
parametrizações dele (alíquotas por subitem, regimes especiais, retenções)
mudam o que a sua DPS pode declarar.

Duas regras de alíquota do ISS dependem disso: E0635 e E0640, que valem para um
ME/EPP do Simples que apura o ISSQN fora do Simples (`regime_apuracao`
`iss-municipio` ou `fora-do-sn`). O `nfse emitir` não opina nesses dois casos
justamente porque a resposta não está na máquina — está no convênio. Veja
[ADR 0006](decisoes/0006-validacao-de-aliquota-do-iss.md).

## 1. A consulta oficial (API de Parâmetros Municipais)

O serviço saiu da Sefin e foi para o ADN. A rota antiga
(`GET /ParametrosMunicipais` na Sefin) responde **501** com o endereço novo:

```
https://adn.producaorestrita.nfse.gov.br/parametrizacao/docs/index.html
```

O contrato está versionado em
[`docs/api/adn-parametrizacao-swagger.json`](api/adn-parametrizacao-swagger.json),
baixado em 30/09/2026 de `/parametrizacao/swagger/v1/swagger.json`, que é o
arquivo que a página acima carrega. Todas as rotas ficam debaixo de
`https://adn.producaorestrita.nfse.gov.br/parametrizacao`:

| Método | Para que serve |
|---|---|
| `GET /{codigoMunicipio}/convenio` | parâmetros do convênio do município |
| `GET /{codigoMunicipio}/{codigoServico}/{competencia}/aliquota` | alíquota do ISSQN do serviço na competência |
| `GET /{codigoMunicipio}/{codigoServico}/historicoaliquotas` | histórico de alíquotas do serviço |
| `GET /{codigoMunicipio}/{codigoServico}/{competencia}/regimes_especiais` | regimes especiais de tributação do serviço |
| `GET /{codigoMunicipio}/{competencia}/retencoes` | retenções do ISSQN definidas pelo município |
| `GET /{codigoMunicipio}/{numeroBeneficio}/{competencia}/beneficio` | parâmetros de um benefício municipal, pelo número |

O mesmo swagger traz três rotas `POST .../{idManut}`. Elas servem para o
município manter os próprios parâmetros, e o contribuinte não as usa.

> **O manual em PDF está desatualizado.** A seção 1.2.1 do manual do
> contribuinte (`docs/markdown/03-api-manual-contribuintes-emissor-publico.md`)
> ainda lista `/parametros_municipais/{codigoMunicipio}/...` e uma consulta de
> retenções e benefícios por CPF/CNPJ. Nenhuma das duas existe na API: não há
> segmento `parametros_municipais`, e retenções e benefícios são consultados
> por município e competência, não por contribuinte. Vale o swagger.

Curitiba é o código IBGE **4106902**. A consulta exige o mesmo certificado A1
que você usa para emitir, em TLS mútuo. Até a página da documentação exige:
sem certificado, o servidor recusa a conexão ainda no *handshake*.

```bash
# Converta o A1 para o par PEM que o curl aceita.
# Os arquivos gerados contêm a sua chave privada — apague depois.
openssl pkcs12 -in certificado.pfx -clcerts -nokeys -out cert.pem
openssl pkcs12 -in certificado.pfx -nocerts -nodes  -out key.pem

curl --cert cert.pem --key key.pem   https://adn.producaorestrita.nfse.gov.br/parametrizacao/4106902/convenio

rm -f cert.pem key.pem
```

Troque `producaorestrita` por produção quando for valer. Se a primeira conexão
falhar no *handshake*, tente de novo: com certificado válido, foi o que
aconteceu com frequência ao baixar o swagger, e a segunda tentativa passou.

Em PowerShell, o `Invoke-RestMethod` lê o `.pfx` direto, sem converter:

```powershell
$cert = Get-PfxCertificate -FilePath .\certificado.pfx
Invoke-RestMethod -Certificate $cert `
  -Uri "https://adn.producaorestrita.nfse.gov.br/parametrizacao/4106902/convenio" |
  ConvertTo-Json -Depth 10
```

Se o certificado já estiver instalado no Windows, use
`Get-Item Cert:\CurrentUser\My\<thumbprint>` no lugar do `Get-PfxCertificate`.
A senha não é pedida.

### O que o swagger deixa em aberto

O serviço já mudou de endereço uma vez. Além disso, estes pontos não se
resolvem lendo o contrato. Confira com uma resposta real antes de depender
deles:

- **`codigoServico`** é só `string`. O swagger não diz se é o `cTribNac` de
  6 dígitos da DPS ou outra forma do subitem.
- **`competencia`** é `date-time`, mas vai no caminho da URL. O swagger não
  mostra como serializá-la (só a data ou com horário).
- **`aliquotas`** e **`regimesEspeciais`** são mapas, e o swagger não diz o que
  é a chave.
- **`Aliq`** é um `double` sem unidade declarada, então pode vir `2.5` ou
  `0.025`. O `iss_aliquota` do `nfse.yaml` é em percentual.
- **Os enums do convênio são inteiros sem nome.** `TipoSimNao` aceita `0`, `1`
  e `-1`, sem dizer qual é qual.
- **A grafia dos campos varia.** `mensagem` e `aliquotas` vêm em camelCase, mas
  os campos de cada alíquota (`Incidencia`, `Aliq`, `DtIni`, `DtFim`) vêm em
  PascalCase. O convênio traz uma propriedade chamada
  `tipoConvenioDeserializationSetter`.

## 2. Os atalhos, quando você só quer a resposta

- **Lista pública de municípios aderentes** — <https://www.nfse.gov.br>, em
  "Municípios Aderentes". Diz se Curitiba assinou o convênio e desde quando,
  mas não detalha as parametrizações.
- **A própria emissão em produção restrita** — mande uma DPS de teste. Se o
  município não estiver ativo, a rejeição diz isso com todas as letras, e em
  produção restrita a nota não tem valor fiscal. É o teste mais direto que
  existe: o que vale é o que a Sefin aceita.
- **A prefeitura** — a Secretaria de Finanças de Curitiba publica a adesão e a
  data de início da obrigatoriedade por grupo de contribuinte. O município pode
  fazer **adoção faseada**, por grupos, então "o convênio está ativo" e "vale
  para o meu CNPJ hoje" são perguntas diferentes.

## O que fazer com a resposta

Se o convênio estiver ativo e você for ME/EPP apurando o ISSQN pelo município,
consulte a rota `.../aliquota` com o seu serviço e a competência da nota. Pegue
o campo `Aliq` da alíquota cujo período (`DtIni`–`DtFim`) cubra a competência e
coloque-o em `padroes.valores.iss_aliquota`, em percentual. Se você apura tudo pelo Simples
(`regime_apuracao: sn`, o padrão), nada disso muda a sua nota: a alíquota
continua fora da DPS, porque o ISS sai no DAS.
