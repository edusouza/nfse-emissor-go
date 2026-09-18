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
[ADR 0005](decisoes/0005-validacao-de-aliquota-do-iss.md).

## 1. A consulta oficial (API de Parâmetros Municipais)

O serviço saiu da Sefin e foi para o ADN. A rota antiga
(`GET /ParametrosMunicipais` na Sefin) responde **501** com o endereço novo:

```
https://adn.producaorestrita.nfse.gov.br/parametrizacao/docs/index.html
```

Os métodos, conforme o manual do contribuinte
(`docs/markdown/03-api-manual-contribuintes-emissor-publico.md`, seção 1.2.1):

| Método | Para que serve |
|---|---|
| `GET /parametros_municipais/{codigoMunicipio}/convenio` | parâmetros do convênio do município |
| `GET /parametros_municipais/{codigoMunicipio}/{codigoServico}` | alíquotas, regimes especiais e deduções por subitem |
| `GET /parametros_municipais/{codigoMunicipio}/{CNPJ}` | retenções e benefícios do contribuinte naquele município |

Curitiba é o código IBGE **4106902**. A consulta exige o mesmo certificado A1
que você usa para emitir, em TLS mútuo:

```bash
# Converta o A1 para o par PEM que o curl aceita.
# Os arquivos gerados contêm a sua chave privada — apague depois.
openssl pkcs12 -in certificado.pfx -clcerts -nokeys -out cert.pem
openssl pkcs12 -in certificado.pfx -nocerts -nodes  -out key.pem

curl --cert cert.pem --key key.pem   https://adn.producaorestrita.nfse.gov.br/parametrizacao/parametros_municipais/4106902/convenio

rm -f cert.pem key.pem
```

Troque `producaorestrita` por produção quando for valer. Abra o swagger no
endereço acima para conferir o caminho exato e os campos da resposta antes de
depender deles: o serviço mudou de endereço uma vez e nada garante que o
contrato tenha ficado parado.

Em PowerShell, o `Invoke-RestMethod` lê o `.pfx` direto, sem converter:

```powershell
$cert = Get-PfxCertificate -FilePath .\certificado.pfx
Invoke-RestMethod -Certificate $cert `
  -Uri "https://adn.producaorestrita.nfse.gov.br/parametrizacao/parametros_municipais/4106902/convenio" |
  ConvertTo-Json -Depth 10
```

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
pegue a alíquota do seu subitem na consulta por `codigoServico` e coloque em
`padroes.valores.iss_aliquota`. Se você apura tudo pelo Simples
(`regime_apuracao: sn`, o padrão), nada disso muda a sua nota: a alíquota
continua fora da DPS, porque o ISS sai no DAS.
