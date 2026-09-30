# Entregar o PDF ao cliente

O DANFSe — o documento auxiliar que se entrega ao cliente — era gerado por uma
API do governo, e a
[NT 008 v1.02, de 14/07/2026](https://github.com/edusouza/nfse-emissor-go/blob/master/docs/notas-tecnicas/nt-008-se-cgnfse-danfse-20260714-v1-02.pdf)
**suspendeu essa API em 03/08/2026**, passando a geração para os softwares de
emissão. O `nfse` gera o documento a partir do XML da nota:

```bash
nfse consultar 41069022212345678000195000000000000126081234567890
nfse danfse notas/41069022212345678000195000000000000126081234567890-nfse.xml
```

Sai uma página A4 no leiaute da NT: cabeçalho com QR Code da consulta pública,
identificação, prestador, tomador, destinatário, intermediário, serviço,
tributação municipal, federal e IBS/CBS, totais, informações complementares e
canhoto.

| Opção | Para quê |
|---|---|
| `-o arquivo.pdf` | grava em outro caminho (padrão: o mesmo nome do XML) |
| `--sobrescrever` | substitui o PDF se ele já existir |
| `--sem-canhoto` | omite o canhoto de recebimento, que a NT deixa opcional |
| `--cancelada` / `--substituida` | imprime a marca d'água correspondente |
| `--sem-rede` | não consulta o nome dos municípios; usa só o cache |
| `--fonte URL` | outro servidor para a consulta de municípios (padrão: o serviço do IBGE) |
| `--cache arquivo` | onde guardar os municípios já consultados (padrão: o diretório de cache do sistema) |

## O que saber antes de entregar

**A marca d'água vem de você, não do XML.** A NFS-e não guarda registro de ter
sido cancelada — o cancelamento é um evento à parte — nem de ter sido
substituída.

**Nota de homologação sai com tarja.** Quando `tpAmb = 2`, o documento leva
"NFS-e SEM VALIDADE JURÍDICA" em vermelho no cabeçalho, como a NT exige.

**O nome do município é consultado.** O XML traz o código de 7 dígitos do IBGE,
e a NT pede o nome; o comando pergunta ao serviço público do IBGE e guarda a
resposta em cache, então a segunda impressão da mesma nota não depende da rede.
Se a consulta não responder, o documento sai com o código no lugar do nome —
nunca deixa de sair. Ver [ADR 0012](../decisoes/0012-municipio-por-consulta.md).

**A assinatura do governo é conferida, e isso tem limite.** O comando avisa
quando o XML não traz a assinatura da Sefin ou quando ela não confere com o
conteúdo — sinal de um XML alterado depois de autorizado. Mas o certificado
vem de dentro do próprio XML, e nada o liga a uma autoridade confiável: a
conferência mostra que o conteúdo não mudou, não que a nota exista. A prova é a
consulta pela chave de acesso no portal nacional, que o QR Code do documento
abre.

**Descrição longa quebra em linhas.** A descrição do serviço aceita até 1300
caracteres. Ela quebra dentro do quadro, que cresce o quanto precisa — como o
item 2.3 da NT permite — tomando o espaço das informações complementares. Se
nem assim couber, o texto termina em reticências.

O XML continua sendo o documento fiscal válido; o DANFSe é a representação
auxiliar. O portal também gera o seu, em
[nfse.gov.br](https://www.nfse.gov.br) → *Download DANFSe*.

Como o documento é desenhado, e com quais bibliotecas:
[ADR 0010](../decisoes/0010-substituicao-e-danfse.md) e
[ADR 0011](../decisoes/0011-bibliotecas-de-pdf-e-qr-code.md).
