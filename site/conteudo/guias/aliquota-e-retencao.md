# Alíquota do ISS e retenção

Quem pode declarar alíquota de ISS na nota depende do regime do prestador, e a
Sefin rejeita a DPS quando a combinação está errada. O `emitir` verifica isso
antes de assinar, então a recusa chega no terminal em vez de voltar como
rejeição:

| Situação | Alíquota |
|---|---|
| MEI | não pode informar ([E0600](../referencia/rejeicoes.md#e0600)) — o ISS sai no DAS |
| ME/EPP no Simples, sem retenção | não pode informar ([E0625](../referencia/rejeicoes.md#e0625)) |
| ME/EPP no Simples, com retenção | **precisa** informar, no mínimo 1,8% ([E0621](../referencia/rejeicoes.md#e0621)) |
| Qualquer um | nunca acima de 5% ([E0595](../referencia/rejeicoes.md#e0595)) |

## Retenção

A retenção vem de `--retencao`, ou de `padroes.valores.retencao_issqn` no
`nfse.yaml`:

```bash
nfse emitir --numero 45 --valor 3000 --descricao "Consultoria" \
  --tomador-cnpj 98765432000198 --tomador-nome "CLIENTE EXEMPLO SA" \
  --retencao tomador --iss-aliquota 2.5
```

Os valores são `nao` (padrão), `tomador` e `intermediario`.

## ISS apurado fora do Simples

Um ME/EPP que apura o ISSQN fora do Simples declara isso em
`prestador.regime_apuracao` (`sn`, `iss-municipio` ou `fora-do-sn`). Nesses dois
últimos casos a regra depende do convênio do município com o Sistema Nacional
([E0635](../referencia/rejeicoes.md#e0635) e
[E0640](../referencia/rejeicoes.md#e0640)), que não dá para saber sem consultar
a Sefin, então o comando não opina. O que se sabe sobre a consulta ao convênio
está em
[`docs/convenio-municipal.md`](https://github.com/edusouza/nfse-emissor-go/blob/master/docs/convenio-municipal.md).

Por quê: [ADR 0006](../decisoes/0006-validacao-de-aliquota-do-iss.md).
