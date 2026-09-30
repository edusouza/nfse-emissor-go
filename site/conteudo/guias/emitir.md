# Emitir uma nota

Com o `nfse.yaml` no lugar (veja [configurar](../comecar/configurar.md)),
confira o que ficou faltando:

```bash
$EDITOR nfse.yaml
nfse config check    # confere se está completo
```

No `nfse.yaml`, a seção `padroes` guarda tudo que se repete — código do serviço,
município, alíquota, e o tomador não identificado quando você vende ao público
em geral. Com ela preenchida, emitir é uma linha:

```bash
nfse emitir --numero 42 --valor 1500 --descricao "Consultoria - agosto/2026"
```

```
DPS DPS410690221234567800019500001000000000000042
  Ambiente       producao-restrita
  Valor          R$ 1500.00
  Servico        Consultoria - agosto/2026
  Assinatura     aplicada
  Arquivo        notas/DPS4106902...042-dps.xml
```

`--numero` é opcional; sem ele, vale o próximo número da série — veja
[a numeração](numeracao.md).

## Identificar o cliente

Para identificar o tomador, use as flags:

```bash
nfse emitir --numero 43 --valor 2400 --descricao "Manutencao mensal" \
  --tomador-cnpj 98765432000198 --tomador-nome "CLIENTE EXEMPLO SA"
```

## A nota num arquivo

Para notas com muitos campos, ou para versionar a nota junto do projeto, use um
arquivo:

```yaml
# nota.yaml
numero: "44"
competencia: "2026-08-01"
servico:
  descricao: Desenvolvimento de API de pagamentos
valores:
  valor_servico: 8500.00
  desconto_incondicionado: 500.00
tomador:
  cnpj: "98765432000198"
  nome: CLIENTE EXEMPLO SA
  email: financeiro@exemplo.com.br
```

```bash
nfse emitir --yaml nota.yaml
```

## Como as três camadas se combinam

Nesta ordem: `padroes` do `nfse.yaml`, depois o arquivo de `--yaml`, depois as
flags. Cada uma sobrescreve a anterior, então `--valor` na linha de comando
vence o que estiver no arquivo.

## Antes de gastar o certificado

`--sem-assinar` gera o XML sem assinatura, para inspecionar. O `emitir` também
valida a DPS contra o schema oficial e confere as regras de alíquota antes de
assinar, então boa parte dos erros aparece no terminal em vez de voltar como
rejeição — veja [o que a validação local cobre](../referencia/validacao.md).

## Próximo passo

Sem `--enviar`, o `emitir` para na assinatura e grava a DPS em `notas/`. Para
transmitir, veja [enviar à Sefin Nacional](enviar.md).
