# Cancelar uma nota

```bash
nfse cancelar 41069022212345678000195000000000000126081234567890 \
  --motivo erro-emissao \
  --justificativa "Valor do servico lancado incorretamente na nota"
```

```
Cancelamento registrado
  NFS-e            41069022212345678000195000000000000126081234567890
  Pedido           PRE41069022212345678000195000000000000126081234567890101101
  Ambiente         producao-restrita
  Evento           notas/PRE4106902...101101-evento.xml
```

O cancelamento é um documento à parte — um pedido de registro de evento,
assinado com o mesmo certificado. Os motivos aceitos são `erro-emissao`,
`nao-prestado` e `outros`.

## A justificativa

A justificativa entra no registro fiscal e o schema exige **entre 15 e 255
caracteres**. Não é capricho da ferramenta: `TSMotivo` impõe o mínimo, que na
prática obriga a explicar o que aconteceu em vez de escrever "erro".

!!! warning "Em produção, é definitivo"
    Cancelar em `producao` pede confirmação no terminal: a operação não se
    desfaz.

Se o que você quer é corrigir a nota, e não anulá-la, veja
[substituir uma nota](substituir.md).
