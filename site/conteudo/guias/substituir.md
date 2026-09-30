# Substituir uma nota

Substituição **não é cancelamento**. O [cancelamento](cancelar.md) é um evento
que anula a nota; a substituição emite uma nota nova no lugar da anterior, e por
isso pede todos os dados de uma emissão:

```bash
nfse emitir --valor 1500 --descricao "Consultoria - agosto/2026" \
  --substitui 41069022212345678000195000000000000126081234567890 \
  --motivo saiu-do-simples --enviar
```

Os motivos são um conjunto fechado, **diferente do conjunto do cancelamento**:

| `--motivo` | Código | Quando |
|---|---|---|
| `saiu-do-simples` | 01 | desenquadramento do Simples Nacional |
| `entrou-no-simples` | 02 | enquadramento no Simples Nacional |
| `incluiu-isencao` | 03 | inclusão retroativa de imunidade/isenção |
| `excluiu-isencao` | 04 | exclusão retroativa de imunidade/isenção |
| `recusada-pelo-tomador` | 05 | rejeição pelo tomador ou intermediário responsável |
| `outros` | 99 | outros — use `--motivo-texto` para explicar |

Mandar um motivo de cancelamento aqui é recusado antes de qualquer coisa: os
códigos são disjuntos e a DPS seria rejeitada pelo schema.

Por quê: [ADR 0010](../decisoes/0010-substituicao-e-danfse.md).
