# Enviar à Sefin Nacional

Há dois jeitos de transmitir: emitir e enviar de uma vez, ou emitir, conferir e
enviar depois o mesmo arquivo.

## De uma vez

```bash
nfse emitir --numero 42 --valor 1500 --descricao "Consultoria" --enviar
```

```
NFS-e emitida
  Chave de acesso  41069022212345678000195000000000000126081234567890
  DPS              DPS410690221234567800019500001000000000000042
  Ambiente         producao-restrita
  Valor            R$ 1500.00
  Processada em    18/09/2026 09:57:36
  DPS assinada     notas/DPS4106902...042-dps.xml
  NFS-e            notas/4106902...67890-nfse.xml

Ambiente de producao restrita: esta nota NAO tem valor fiscal.
```

## Conferir agora, enviar depois

`nfse emitir` sem `--enviar` para na assinatura e grava a DPS. Para transmitir
**aquele mesmo arquivo** depois, use `nfse enviar`:

```bash
nfse emitir --valor 1500 --descricao "Consultoria - agosto/2026"
# confira notas/DPS4106902...-dps.xml
nfse enviar notas/DPS4106902...-dps.xml
```

Chamar `nfse emitir --enviar` de novo **não** manda o arquivo anterior: monta um
documento novo, com o próximo número da série e outro instante de emissão. A
nota que você conferiu ficaria para trás, e o número já gasto viraria um buraco
na sequência.

`nfse enviar` não assina nada e não mexe no contador — a numeração pertence à
emissão. Um arquivo sem assinatura é recusado antes de sair da máquina, porque
é o engano provável: `--sem-assinar` grava com nome parecido.

## Quando a Sefin recusa

A emissão é **síncrona**: o governo valida e devolve a nota autorizada ou a
rejeição na mesma requisição. A rejeição vem com um código por motivo, todos de
uma vez. Esta foi a resposta à primeira emissão real deste projeto, como o
`nfse` a mostra hoje:

```
erro: documento rejeitado pela Sefin Nacional
  - [E0714] Arquivo enviado com erro na assinatura.
      Campo   DPS/Signature
      Regra   A assinatura da DPS deve ser válida.
      Mais    https://edusouza.github.io/nfse-emissor-go/referencia/rejeicoes/#e0714
```

A primeira linha de cada motivo é a da Sefin. As de baixo vêm do ANEXO I: o
campo da DPS, a regra que falhou e, quando a regra depende do município, um
aviso. O link leva ao código em [códigos de rejeição](../referencia/rejeicoes.md),
que traz também as observações do anexo. A história deste E0714,
e das três rejeições que vieram depois dele, está na
[ADR 0008](../decisoes/0008-digest-sem-namespace.md).

## Produção

O ambiente vem do `nfse.yaml`. Com `ambiente: producao` a nota tem **valor
fiscal** e o comando pede confirmação no terminal antes de enviar — cancelar uma
nota emitida exige um pedido de evento. Use `--confirmar` para dispensar a
pergunta em scripts.

!!! danger "O envio não é repetido automaticamente"
    Emissão não é idempotente: uma requisição que chegou ao governo e falhou na
    volta geraria uma segunda nota numa retentativa. Se a rede falhar no meio
    do envio, [consulte pelo identificador da DPS](consultar.md#emissao-interrompida)
    antes de tentar de novo.
