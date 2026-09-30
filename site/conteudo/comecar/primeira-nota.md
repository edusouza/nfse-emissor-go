# A primeira nota

O caminho mais curto do `nfse.yaml` pronto até o PDF na mão do cliente. Cada
passo tem um guia com os detalhes.

!!! tip "Comece pela produção restrita"
    Com `ambiente: producao-restrita` no `nfse.yaml`, o governo processa a nota
    de verdade, mas ela **não tem valor fiscal**. É o lugar certo para o
    primeiro teste.

## 1. Conferir o certificado e a configuração

```bash
export NFSE_CERT_SENHA='sua-senha'
nfse cert info --arquivo certificado.pfx
nfse config check
```

O primeiro sai com código diferente de zero se o certificado não puder
assinar; o segundo lista tudo o que falta no `nfse.yaml`. Detalhes em
[o certificado e a senha](../guias/certificado.md) e em
[configurar](configurar.md).

## 2. Emitir sem enviar

Com a seção `padroes` do `nfse.yaml` preenchida, uma nota para o público em
geral precisa só de valor e descrição:

```bash
nfse emitir --valor 1500 --descricao "Consultoria - agosto/2026"
```

```
DPS DPS410690221234567800019500001000000000000042
  Ambiente       producao-restrita
  Valor          R$ 1500.00
  Servico        Consultoria - agosto/2026
  Assinatura     aplicada
  Arquivo        notas/DPS4106902...042-dps.xml
```

A DPS foi montada, validada contra o schema oficial e assinada, mas ainda não
saiu da máquina. Confira o arquivo se quiser. Para identificar o cliente,
ler a nota de um arquivo ou declarar retenção, veja
[emitir uma nota](../guias/emitir.md).

## 3. Enviar

```bash
nfse enviar notas/DPS4106902...042-dps.xml
```

```
NFS-e emitida
  Chave de acesso  41069022212345678000195000000000000126081234567890
  DPS              DPS410690221234567800019500001000000000000042
  Ambiente         producao-restrita
  Valor            R$ 1500.00
  Processada em    18/09/2026 09:57:36
  DPS enviada      notas/DPS4106902...042-dps.xml
  NFS-e            notas/4106902...67890-nfse.xml

Ambiente de producao restrita: esta nota NAO tem valor fiscal.
```

A resposta é síncrona: a nota autorizada ou a rejeição chegam na mesma
requisição. Se vier uma rejeição, o código está em
[códigos de rejeição](../referencia/rejeicoes.md). Quem não precisa conferir
antes pode juntar os passos 2 e 3 com `nfse emitir --enviar` — veja
[enviar à Sefin Nacional](../guias/enviar.md).

## 4. Gerar o PDF

```bash
nfse danfse notas/41069022212345678000195000000000000126081234567890-nfse.xml
```

Sai o DANFSe, uma página A4 no leiaute da NT 008, para entregar ao cliente. O
XML continua sendo o documento fiscal; veja
[entregar o PDF ao cliente](../guias/danfse.md).
