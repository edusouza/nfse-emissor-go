# 0001 — CLI em vez de API REST

**Status:** Aceita
**Data:** 2026-09-18

## Contexto

O projeto nasceu como um serviço de backend: API REST em Gin, worker assíncrono
com Asynq, MongoDB para status das requisições e chaves de API, Redis para fila
e *rate limiting*. Eram cerca de 35 mil linhas de Go, com `docker-compose`,
health checks para Kubernetes e métricas Prometheus.

Nada disso emitia uma nota fiscal. A arquitetura de um SaaS multi-inquilino
estava pronta antes de existir um caminho funcionando do dado até o XML assinado
na mão do contribuinte.

O objetivo real é outro: um MEI/ME/EPP emitir a própria nota. Uma pessoa, um
certificado, uma nota por vez.

## Decisão

Transformar o projeto em um binário único de linha de comando e remover a
camada de serviço: `cmd/api`, `cmd/worker`, `internal/api`, `internal/jobs`,
`internal/infrastructure/{mongodb,redis,webhook}`, `internal/config` (que só
lia variáveis de ambiente de servidor) e `docker-compose.yml`.

## Consequências

**O que ganhamos**

- As dependências diretas caíram de 12 para 4. Saíram Gin, Asynq, MongoDB
  driver, Redis, Prometheus, `redis_rate`, `miniredis`, `uuid`.
- Some a operação: sem banco, sem fila, sem servidor para manter de pé. Quem
  usa baixa um binário.
- A suíte de testes deixa de depender de Redis e Mongo.

**O que perdemos**

- Processamento assíncrono e *webhooks*. Para uma nota por vez não fazem falta:
  a API do Sistema Nacional é **síncrona** — o `POST /nfse` já devolve a NFS-e
  autorizada ou a rejeição.
- Autenticação por chave de API e *rate limiting*. Num CLI local, a autenticação
  é o próprio certificado digital.

**O que foi preservado**

O núcleo de domínio, que é onde estava o trabalho difícil e que já tinha testes:

- `pkg/xmlbuilder` — montagem do XML da DPS
- `internal/infrastructure/xmlsigner` — assinatura XMLDSig, canonicalização
  exc-c14n, validação de certificado
- `pkg/cnpjcpf`, `pkg/dpsid` — validação de CNPJ/CPF e do identificador da DPS
- `internal/domain/{emission,validation,query}` — cálculo de valores, validações
  e tradução de códigos de rejeição

O código removido continua no histórico do git. Se um dia fizer sentido expor
uma API, ela volta como uma casca fina sobre esse mesmo núcleo.

## Aprendizado

A infraestrutura foi construída antes do caso de uso. O sintoma clássico:
`docker-compose` com três serviços, *readiness probe* e métricas — e nenhuma
nota emitida. A ordem correta era provar o caminho mais estreito primeiro
(dados → XML → assinatura → envio) e só então decidir como distribuir.
