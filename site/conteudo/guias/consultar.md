# Consultar uma nota

```bash
nfse consultar 41069022212345678000195000000000000126081234567890
```

```
NFS-e encontrada
  Chave de acesso  41069022212345678000195000000000000126081234567890
  Ambiente         producao-restrita
  Arquivo          notas/4106902...67890-nfse.xml
```

A consulta grava o XML da nota, o mesmo que o [DANFSe](danfse.md) usa. O que
compõe os 50 dígitos está em [a chave de acesso](../referencia/chave-de-acesso.md).

## Emissão interrompida

Se uma emissão foi interrompida e você não sabe se a nota saiu, consulte pelo
identificador da declaração:

```bash
nfse consultar --dps DPS410690221234567800019500001000000000000042 --existe
```

`--existe` responde apenas sim ou não — o governo atende essa pergunta a
qualquer certificado válido. Sem a flag, ele devolve a chave de acesso, que por
sigilo fiscal só é informada a quem consta na nota (prestador, tomador ou
intermediário).
