# 0016 — Parâmetros municipais consultados no ADN, com cache de 24 horas

**Status:** Aceita
**Data:** 2026-09-30

## Contexto

A alíquota do ISSQN depende de **município + serviço + competência**, e duas
regras de recepção da DPS (E0635 e E0640) dependem de o convênio do município
estar ativo ([ADR 0006](0006-validacao-de-aliquota-do-iss.md)). As duas
respostas estão na API de Parâmetros Municipais do ADN, cujo contrato foi
versionado e conferido no serviço real (issue
[#5](https://github.com/edusouza/nfse-emissor-go/issues/5),
[docs/convenio-municipal.md](../convenio-municipal.md)).

Diferente da lista de serviços ([ADR 0009](0009-lista-de-servicos-embutida.md))
e dos municípios do IBGE ([ADR 0012](0012-municipio-por-consulta.md)), estes
dados **mudam sem aviso**: o município reparametriza quando quer, pelo Painel
Administrativo Municipal. Uma tabela embutida envelheceria em silêncio, e uma
resposta guardada para sempre também.

Consultar a cada uso tem outro custo: a mesma pergunta sai de novo a cada nota
do dia, por TLS mútuo, contra um serviço cujo primeiro *handshake* costuma
falhar.

## Decisão

**Consultar o ADN e guardar a resposta por 24 horas**, em
`parametros.json` no diretório de cache do sistema operacional, ao lado do
cache de municípios.

- **24 horas foi a escolha do autor.** Poupa a viagem nas emissões do mesmo dia
  e expira antes que uma mudança do município sobreviva a um dia útil.
- **Só respostas são guardadas.** Um convênio inativo é resposta, e fica. Uma
  recusa (400, 404 de alíquota) ou uma falha de rede não ficam: são perguntadas
  de novo na próxima vez.
- **A chave é o endereço consultado**, com a URL base. Produção e produção
  restrita nunca respondem uma pela outra, e as duas têm dados diferentes.
- **Nada de resposta vencida como reserva.** Se o ADN não responde e o que está
  guardado venceu, a consulta falha. Mostrar uma resposta velha como se fosse
  atual é o erro que a validade existe para evitar.
- **`--sem-cache` pergunta de novo** e guarda o que vier. Serve para quem sabe
  que o município acabou de mudar.
- **O arquivo é conferido na leitura.** Qualquer coisa pode tê-lo escrito, e uma
  entrada que não bate com a rota da chave, sem data, com vigência invertida ou
  datada no futuro é descartada e perguntada de novo.
- **A saída diz quando a resposta foi obtida**, para que ninguém confunda o que
  o cache lembra com o que o ADN acabou de dizer.

## Consequências

- Uma mudança do município pode levar até 24 horas para aparecer. O pior caso
  para o convênio é uma rejeição da Sefin (E0635 ou E0640), **nunca uma nota
  errada**: quem decide é a Sefin, que consulta a própria parametrização.
- Um cache que não pode ser lido nem gravado custa uma consulta, nunca um
  resultado.
- A gravação atômica e o diretório de cache passam a ser comuns aos dois caches
  (`internal/infrastructure/arquivo`).

## Alternativas consideradas

**Sem cache.** É o mais simples e nunca está desatualizado, mas faz toda
emissão depender de uma segunda viagem ao governo, e o ADN é justamente o
serviço cujo *handshake* falha com frequência.

**Até o fim do mês da competência.** Casa com o ciclo de apuração, mas guarda
uma resposta por até 30 dias em dados que o município muda quando quer.

**7 dias.** Menos rede, com uma semana de defasagem depois de uma mudança.
