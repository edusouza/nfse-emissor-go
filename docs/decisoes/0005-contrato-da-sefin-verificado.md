# 0005 — Cliente da Sefin reescrito contra a especificação oficial

**Status:** Aceita
**Data:** 2026-09-18

## Contexto

O cliente original falava **SOAP** com um envelope que o próprio código admitia
ter inventado. A API do Sistema Nacional é REST/JSON. Nada que o projeto
produziu jamais chegou ao governo.

Pior que o erro: os 2.561 linhas de teste que o acompanhavam passavam, porque
testavam o cliente contra um *mock* que reproduzia o mesmo contrato inventado.
Um teste verde que valida uma ficção é pior que nenhum teste — ele dá confiança.

O impasse era de verificação, não de implementação: este ambiente bloqueia
domínios `gov.br`, então o contrato real não podia ser conferido daqui. Foi
resolvido do jeito certo — as especificações OpenAPI oficiais foram colocadas no
repositório, em `docs/api/`.

## Decisão

Apagar o cliente SOAP e seus testes por inteiro e reescrever a partir de
`docs/api/sefin-nacional-swagger.json` ("API NFS-e - Sefin Nacional").

## O que a especificação corrigiu

Antes dela chegar, três detalhes tinham sido implementados a partir de contratos
públicos documentados por integradores. Dois estavam certos, dois estavam
errados — e os errados são instrutivos:

| Detalhe | Suposição | Especificação |
|---|---|---|
| Campo da requisição | `dpsXmlGZipB64` | ✅ igual |
| Campo da resposta | `nfseXmlGZipB64` | ✅ igual |
| `basePath` | `/sefinnacional` | ❌ **`/SefinNacional`** — caminho é sensível a maiúsculas |
| `tipoAmbiente` | string | ❌ **inteiro** (1-Produção, 2-Homologação) |
| Sucesso | 2xx | **201 Created** |

O `tipoAmbiente` como string faria toda resposta falhar na desserialização. O
`basePath` em minúsculas daria 404 em todas as chamadas. Nenhum dos dois é o
tipo de coisa que se descobre por leitura — só por 404 em produção.

A especificação também revelou uma armadilha que eu não teria previsto: os
erros vêm em **duas formas**. `NFSePostResponseErro` traz `erros` (array),
enquanto `ResponseErro`, usado nas consultas e eventos, traz `erro` (objeto
único). Um cliente que lesse só o plural perderia silenciosamente o motivo da
falha em metade dos endpoints.

## Consequências

- O contrato está inteiramente rastreável a um documento publicado. Onde algo
  não estiver, o comentário diz isso em vez de deixar parecer resolvido.
- `Emit` **não tenta novamente**. Emissão não é idempotente: uma requisição que
  chegou ao governo, gerou a nota e falhou na volta produziria uma segunda nota
  numa retentativa. Há teste fixando esse comportamento.
- O ambiente padrão é produção restrita. Errar para o lado que não gera
  documento fiscal é a forma barata de errar.
- Emitir em produção pede confirmação no terminal, porque desfazer exige um
  pedido de evento de cancelamento.
- O `tipoAmbiente` da resposta é lido de volta e exibido: quem decide se a nota
  tem valor fiscal é o governo, não o arquivo de configuração local.

## Aprendizado

Duas coisas.

A primeira é sobre *mocks*. O cliente antigo tinha 95% de cobertura. A cobertura
media o quanto o código foi exercitado, não o quanto ele estava certo — e como o
mock e o cliente compartilhavam a mesma premissa falsa, nenhum teste podia
detectá-la. Um teste só vale contra um contrato externo se o contrato vier de
fora.

A segunda é sobre onde parar. Entre não ter a especificação e tê-la, a estratégia
foi concentrar tudo que era suposição num único arquivo (`contract.go`), marcado
como tal. Quando a especificação chegou, corrigir foram duas constantes e um
tipo. Isolar a incerteza é mais barato que espalhá-la.
