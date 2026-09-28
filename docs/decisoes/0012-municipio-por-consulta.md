# 0012 — Traduzir o código do município por consulta, não por tabela embutida

**Status:** Aceita
**Data:** 2026-09-22

## Contexto

A [NT 008](../notas-tecnicas/nt-008-se-cgnfse-danfse-20260714-v1-02.pdf) manda o
DANFSe imprimir o **nome** do município em quatro blocos — prestador, tomador,
destinatário e intermediário: *"Leiaute prevê a informação do código do
município com 7 dígitos da Tabela do IBGE. Utilizar a descrição destes
códigos"*.

O XML só tem o código. O `endNac` do leiaute carrega `cMun` e `CEP`, e mais
nada; o nome aparece uma única vez no documento inteiro, no `xLocEmi` do
emitente. Para todo o resto, sem uma tradução de código em nome, o campo sairia
`3550308`.

Traduzir 5570 códigos tem dois caminhos: carregar a tabela dentro do binário —
que é o trabalho já pronto da [issue #11](https://github.com/edusouza/nfse-emissor-go/issues/11),
131 KB de CSV gerado a partir do `ANEXO_A` oficial — ou perguntar a um serviço.

## Decisão

**Perguntar ao serviço público do IBGE**, em
`https://servicodados.ibge.gov.br/api/v1/localidades/municipios/{codigo}`.

A escolha é do autor do projeto, e a razão é explícita: **não carregar tanta
informação dentro de um binário só**. O emissor já embute a lista nacional de
serviços ([ADR 0009](0009-lista-de-servicos-embutida.md)); somar a tabela do
IBGE seria embutir um segundo cadastro inteiro para resolver um campo.

A consulta é um caminho de conveniência, nunca um requisito, e o desenho
inteiro sai dessa frase:

**O documento sai de qualquer jeito.** Rede fora, proxy bloqueado, serviço
indisponível, código que o IBGE não conhece: a resposta vira "não sei" e o
campo imprime o código. Um DANFSe que não imprime é pior do que um que imprime
`3550308`.

**O que foi perguntado uma vez não se pergunta de novo.** As respostas vão para
um cache em disco, no diretório de cache do sistema operacional. É o ponto que
mais pesa: uma nota fiscal é guardada por cinco anos e pode ser reimpressa a
qualquer momento — a segunda impressão da mesma nota não depende de o serviço
existir. Nome de município não muda de um dia para o outro.

**Dá para desligar.** `--sem-rede` responde só pelo cache, como no
[`onboard`](0007-preenchimento-da-configuracao.md). E, como lá, o comando diz
para onde está mandando os códigos antes de sair para a rede.

**Uma falha é dita uma vez.** Quatro blocos na mesma página não viram quatro
mensagens iguais.

## Consequências

- **O binário não cresce.** A tabela do IBGE fica de fora, e o trabalho da #11
  segue arquivado em `refs/pull/19/head` — a issue continua aberta, agora com
  outro uso: o `onboard --sem-rede`, que não tem para onde apelar.
- **O DANFSe passa a ter um caminho de rede que não é o governo.** É opcional,
  desligável, e não bloqueia a impressão. Os códigos de município da nota vão
  para um terceiro — informação pública, mas que conta a ele *quem* você
  fatura. Por isso a consulta é anunciada.
- **Um documento pode sair diferente do mesmo XML** conforme o cache e a rede:
  com o nome numa máquina, com o código em outra. É o preço que a tabela
  embutida não cobrava, e o cache reduz — mas não elimina.
- **O contrato não foi exercitado contra o serviço real.** O ambiente onde este
  código foi escrito não alcança `servicodados.ibge.gov.br` (o proxy recusa a
  conexão), então os campos vêm da documentação do serviço e os testes usam um
  servidor local. É a mesma pendência da [ADR 0007](0007-preenchimento-da-configuracao.md)
  com a BrasilAPI, e está na mesma lista de problemas conhecidos do CHANGELOG.
- **A sigla da UF é lida de dois lugares.** O serviço já moveu a UF entre ramos
  da própria resposta — `microrregiao.mesorregiao.UF` e
  `regiao-imediata.regiao-intermediaria.UF`. Ler só um deles perderia a sigla
  em parte das respostas, e um município sem UF é meio campo num documento
  fiscal.

## Alternativas consideradas

**Embutir a tabela.** Tecnicamente melhor para este caso: resolve offline, é
determinística, não conta nada a ninguém, e o código já existe testado. Custa
131 KB — 0,9% do binário. Foi recusada pelo autor, que não quer o binário
acumulando cadastros; a decisão é dele e está registrada aqui com o custo que
ela traz.

**BrasilAPI**, que o `onboard` já usa. Ela expõe municípios *por UF*
(`/api/ibge/municipios/v1/{UF}`), não por código, o que obrigaria a baixar um
estado inteiro para traduzir um número — ou a adivinhar a UF antes de saber
qual é.

**Deixar o código mesmo.** É o que o emissor faz quando a consulta não
responde, e seria aceitável como estado permanente apenas se a NT não pedisse o
nome. Ela pede.

## Aprendizado

Uma dependência de rede muda de peso conforme o que ela alimenta. No `onboard`,
uma consulta que falha custa um minuto de digitação: o `nfse.yaml` é gravado do
mesmo jeito. No DANFSe, a mesma falha mudaria o **conteúdo de um documento
fiscal** — e é por isso que aqui ela ganhou cache em disco, desligamento
explícito e degradação silenciosa, coisas que lá não foram necessárias.
