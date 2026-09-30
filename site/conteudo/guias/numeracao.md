# A numeração

`--numero` é opcional: sem ele, o `nfse` usa o próximo número da série. O último
número usado fica em `.nfse-estado.json`, ao lado do `nfse.yaml`.

```console
$ nfse numero ver
Ultimo numero usado por serie (registro local):

  00001  ultimo 2, proximo 3   18/09/2026 13:52  <- serie configurada
```

## Vindo de outro emissor

Quem migra de outro emissor precisa retomar a numeração existente:

```bash
nfse numero definir 500     # a próxima será a 501
```

## O arquivo não é a verdade

**O arquivo é uma conveniência local.** Quem decide o que foi realmente emitido
é a Sefin. Emitir da mesma configuração em duas máquinas dessincroniza a
contagem — nesse caso confira na Sefin e realinhe com `nfse numero definir`.

Informar `--numero` explicitamente continua funcionando e **nunca puxa o
contador para trás**: preencher uma lacuna com um número antigo não faz a
próxima emissão automática colidir com uma nota já emitida.
