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

## Quando a Sefin recusa

O prazo e o valor máximo para cancelar são do município, parametrizados no
Sistema Nacional. Passado o prazo, a recusa é esta:

```
erro: documento rejeitado pela Sefin Nacional
  - [E0822] O prazo para o cancelamento da NFS-e expirou, conforme parametrização do município emissor da NFS-e.
      Campo   pedRegEvento/infPedReg/chNFSe
      Regra   Não pode ocorrer cancelamento de NFS-e fora do prazo limite para o cancelamento da NFS-e, conforme parametrização do município emissor da NFS-e.
      Depende do municipio: segue a parametrizacao que ele fez no Sistema Nacional.
      Mais    https://edusouza.github.io/nfse-emissor-go/referencia/rejeicoes-de-eventos/#e0822
```

Os códigos do cancelamento vêm do ANEXO II, não do ANEXO I da emissão, e o
`nfse` explica cada um como explica as rejeições da DPS: o campo do pedido, a
regra e o aviso quando ela depende do município. A lista completa está em
[códigos de rejeição de eventos](../referencia/rejeicoes-de-eventos.md).

Se o que você quer é corrigir a nota, e não anulá-la, veja
[substituir uma nota](substituir.md).
