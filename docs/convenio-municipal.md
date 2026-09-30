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

O jeito mais curto é o próprio emissor, que usa o certificado e o ambiente do
`nfse.yaml`:

```bash
nfse parametros 4106902 010701 --competencia 2026-09-01
```

O resto desta seção mostra a API por baixo dele, para quem quiser consultá-la
à mão ou conferir o que o comando faz.

O serviço saiu da Sefin e foi para o ADN. A rota antiga
(`GET /ParametrosMunicipais` na Sefin) responde **501** com o endereço novo:

```
https://adn.producaorestrita.nfse.gov.br/parametrizacao/docs/index.html
```

O contrato está versionado em
[`docs/api/adn-parametrizacao-swagger.json`](api/adn-parametrizacao-swagger.json),
baixado em 30/09/2026 de `/parametrizacao/swagger/v1/swagger.json`, que é o
arquivo que a página acima carrega. O de produção (`adn.nfse.gov.br`) é idêntico
byte a byte. Todas as rotas ficam debaixo de
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
> por município e competência, não por contribuinte. O manual dos municípios
> (`docs/markdown/06-api-manual-municipios-emissor-publico.md`, seção 1.1.2)
> traz uma terceira versão dos caminhos, também sem a competência e com as
> descrições de retenções e regimes especiais trocadas. Vale o swagger.

Curitiba é o código IBGE **4106902**. A consulta exige o mesmo certificado A1
que você usa para emitir, em TLS mútuo. Até a página da documentação exige:
sem certificado, o servidor recusa a conexão ainda no *handshake*.

```bash
# Converta o A1 para o par PEM que o curl aceita.
# Os arquivos gerados contêm a sua chave privada — apague depois.
openssl pkcs12 -in certificado.pfx -clcerts -nokeys -out cert.pem
openssl pkcs12 -in certificado.pfx -nocerts -nodes  -out key.pem

curl --cert cert.pem --key key.pem   https://adn.producaorestrita.nfse.gov.br/parametrizacao/4106902/convenio

# Alíquota do serviço 01.07.01 em Curitiba, na competência de setembro/2026.
curl --cert cert.pem --key key.pem   https://adn.producaorestrita.nfse.gov.br/parametrizacao/4106902/01.07.01.000/2026-09-01/aliquota

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

### O que o swagger não diz, conferido no serviço real

O swagger declara os tipos, mas não os formatos. O que segue foi conferido em
30/09/2026, em produção restrita, com consultas `GET` e um e-CNPJ A1.

**O código do serviço é o código completo, com pontos:** `II.SS.DD.CCC`. São
os 6 dígitos do `cTribNac` da DPS mais os 3 do complemento municipal. O
complemento é `000`, a não ser que o município tenha criado um código de
tributação próprio abaixo do seu serviço. Nesse caso ele é diferente de `000`
(guia do Painel Municipal, p. 49). Qualquer outra grafia recebe 400, inclusive
os mesmos 9 dígitos sem pontos:

```
GET /4106902/010701/2026-09-01/aliquota        → 400
GET /4106902/010701000/2026-09-01/aliquota     → 400
    {"aliquotas":null,"mensagem":"Chamada mal formada. O código do serviço deve ser composto por nove dígitos."}
GET /4106902/01.07.01.000/2026-09-01/aliquota  → 200
GET /4106902/01.07.01.001/2026-09-01/aliquota  → 404 (formato aceito, sem dados)
```

As retenções confirmam o nome: cada serviço vem como `"codigoCompleto": "10.01.01.000"`.

**A competência vai como `AAAA-MM-DD`.** `2026-09-01T00:00:00` e `09-01-2026`
também passam, mas o servidor lê datas ambíguas com o **mês primeiro**:
`12-31-2022` caiu na vigência de 31/12/2022, e `15-09-2026` recebeu 400. Uma
data escrita dia-mês-ano com dia até 12 seria aceita e lida errado, sem aviso.
Use só o formato ISO.

**A alíquota vem em percentual,** em um mapa cuja chave é o código completo:

```json
{"aliquotas":{"01.07.01.000":[{"Incidencia":"SIM","Aliq":5.00,"DtIni":"2023-01-02T00:00:00","DtFim":null}]},
 "mensagem":"Alíquotas recuperadas com sucesso."}
```

- `.../aliquota` devolve a alíquota vigente na competência, e
  `.../historicoaliquotas` devolve todas.
- `DtIni` e `DtFim` vêm com horário e sem fuso. `DtFim` é `null` enquanto a
  vigência está aberta.
- Só os campos de cada alíquota vêm em PascalCase. O resto da API, inclusive
  as retenções (`dataInicioVigencia`, `tiposRetencao`), vem em camelCase.

**O convênio real não traz `tipoConvenioDeserializationSetter`,** que o swagger
declara:

```json
{"parametrosConvenio":{"aderenteAmbienteNacional":1,"aderenteEmissorNacional":1,
  "situacaoEmissaoPadraoContribuintesRFB":1,"aderenteMAN":0,"permiteAproveitametoDeCreditos":true},
 "mensagem":"Parâmetros do convênio recuperados com sucesso."}
```

Em Curitiba e em São Paulo, os dois ativos, as adesões vêm `1` e o MAN vem
`0`. Isso é coerente com 1 = Sim e 0 = Não. Um município sem convênio ativo
recebe 404, com `parametrosConvenio: null` e o motivo em `mensagem` ("O
convênio do município <São Caetano do Sul/SP> ainda não está ativo…").

**Há dois formatos de erro.** Recusas do serviço (400 e 404) vêm com o campo da
resposta `null` e o motivo em `mensagem`, como nos exemplos acima. Um valor que
nem chega a ser lido, como uma competência inválida, recebe o 400 padrão do
ASP.NET:

```json
{"type":"https://tools.ietf.org/html/rfc9110#section-15.5.1","title":"One or more validation errors occurred.",
 "status":400,"errors":{"competencia":["The value '15-09-2026' is not valid."]},"traceId":"00-…"}
```

**Continua em aberto:**
- As chaves de `regimesEspeciais`. Todas as consultas deram 404: sete códigos em
  Curitiba e São Paulo, entre eles contabilidade, advocacia, medicina e
  construção.
- O significado de `-1` nos enums do convênio, que não apareceu.
- Os códigos de `tiposRetencao` (`[2]`, `[2, 3]`).

**Os dados de produção restrita são de teste.** O histórico de Curitiba para
`01.07.01.000` tem 5% até 31/12/2022, 2% só no dia 01/01/2023 e 5% a partir
de 02/01/2023. Confie no formato, não nos valores: a alíquota que vale é a de
produção (`https://adn.nfse.gov.br/parametrizacao`).

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

Para um ME/EPP do Simples que apura o ISSQN pela alíquota do município
(`regime_apuracao` `iss-municipio` ou `fora-do-sn`), o convênio decide o que a
DPS pode levar. As regras são E0635 e E0640 do ANEXO I, aba "RN DPS_NFS-e":

| Convênio na competência da nota | `iss_aliquota` | Regra |
|---|---|---|
| ativo | **não informe** (`0`): a Sefin aplica a alíquota parametrizada | E0635 |
| inativo | **informe**: sem ela a DPS é rejeitada | E0640 |

Com o convênio ativo, a alíquota que o ADN devolve é a que a Sefin vai aplicar
na nota. Consultá-la serve para saber quanto de ISS a nota vai ter, não para
copiá-la para a configuração. Com o convênio inativo, o ADN não tem alíquota
para o município, e quem diz qual vale é a prefeitura.

Se você apura tudo pelo Simples (`regime_apuracao: sn`, o padrão), o convênio
não muda a sua nota: sem retenção, a alíquota fica fora da DPS, porque o ISS
sai no DAS. Com retenção, você informa a alíquota do seu anexo do Simples, no
mínimo 1,8% (E0621 e E0628).
