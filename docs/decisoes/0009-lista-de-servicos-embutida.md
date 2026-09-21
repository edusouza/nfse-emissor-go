# 0009 — Embutir a lista nacional de serviços e procurar o cTribNac

**Status:** Aceita
**Data:** 2026-09-21

## Contexto

A [ADR 0007](0007-preenchimento-da-configuracao.md) deixou o `nfse onboard`
preenchendo quatro dos cinco campos obrigatórios e registrou o que sobrou:

> Sobra **um** campo que a máquina não tem como descobrir:
> `codigo_tributacao_nacional`, o código do serviço na lista da LC 116/2003.

Na prática, quem rodou o `onboard` chegou exatamente nesse ponto e perguntou o
que parece a saída óbvia: *o CNAE primário não serve?* O cadastro da Receita
responde o CNAE fiscal, é a única coisa que ele sabe sobre o que a empresa faz,
e está a um campo de distância na resposta que o comando já lê.

Não serve — e a razão importa, porque é o mesmo tipo de engano que a ADR 0007
evitou ao não chutar o regime tributário:

- O **CNAE** classifica a *atividade econômica da empresa* para a Receita
  Federal. É um atributo do CNPJ.
- O **cTribNac** classifica o *serviço prestado* para efeito de ISS, segundo a
  lista da LC 116/2003. É um atributo da nota, e a mesma empresa emite notas com
  códigos diferentes.

São duas taxonomias escritas para finalidades diferentes, sem correspondência
oficial entre elas. O `ANEXO_B` — a lista nacional, versionada em
`docs/anexos/` — tem 335 códigos e cinco colunas: código de tributação
nacional, item, subitem, desdobro nacional e descrição. **Nenhuma delas é
CNAE.** Alguns municípios publicam um de-para do CNAE para a própria lista
municipal de serviços, mas isso é municipal, não vale para a lista nacional, e
não está em lugar nenhum do Sistema Nacional.

O que restava era o padrão já conhecido deste repositório: a tabela que
responde a pergunta estava versionada desde o começo e nenhuma linha de código
a tinha lido.

## Decisão

**A lista nacional passa a ser embutida no binário**, gerada do anexo oficial
por um programa versionado junto (`internal/domain/servico/gerar_lista.go`,
saída em `lista.csv`). São 66 KB para 335 códigos, sem dependência nova: um
`.xlsx` é um zip de XML, e `archive/zip` mais `encoding/xml` bastam. Um
`go run` reconstrói o arquivo, e um teste refaz a geração e compara byte a
byte — o CSV é confiável só enquanto reproduzir o anexo.

**Três comandos para achar o código:**

```
nfse servico buscar "suporte tecnico"   procura pela descrição
nfse servico ver 010701                 mostra o que um código significa
nfse servico listar [item]              percorre os 41 itens da lei
```

A busca pesa cada palavra pela raridade dela na lista: "serviços" e
"congêneres" aparecem em quase todo código e não separam nada, enquanto
"veterinário" aponta para um punhado. Casa também por prefixo, o que resolve
plural e flexão sem carregar um *stemmer*.

**O `onboard` ganha `--servico`**, conferido contra a lista antes de gravar, e
**sugere candidatos a partir do CNAE** quando o código não é informado. As
sugestões vão para a tela e para o `nfse.yaml` **comentadas**, com o CNAE que as
gerou e a ressalva de que são palpite. O campo continua vazio.

**O `config check` diz o que o código significa** e avisa — sem recusar —
quando o cTribNac configurado não está na lista embutida.

## Por que embutir aqui, se a ADR 0007 recusou embutir a tabela do IBGE

A ADR 0007 descartou embutir o `ANEXO_A` dos municípios, e a diferença entre os
dois casos é inteira:

| | Municípios (ADR 0007) | Serviços (esta) |
|---|---|---|
| Tamanho | 5.570 | 335 |
| Existe consulta que responde? | **sim**, e o comando já a fazia | **não**, nenhuma |
| Resolve com uma pergunta? | sim, "cidade e UF" | não — depende do que a pessoa faz |

O código do município saía de graça de uma consulta que o comando já fazia por
outro motivo. O código do serviço não sai de lugar nenhum: nenhum serviço,
público ou privado, sabe qual serviço um prestador presta. Embutir a tabela é a
única forma de o emissor ajudar — e ajudar, aqui, é procurar junto, não
responder no lugar.

## Por que a sugestão nunca preenche o campo

A tentação é gravar o primeiro candidato. O caso que aparece no teste diz por
que não: o CNAE 6209-1/00 se descreve como *"Suporte técnico, manutenção e
outros serviços em tecnologia da informação"*, e a expressão "tecnologia da
informação" está **escrita literalmente** dentro do código 110501, que trata de
monitoramento e rastreamento de veículos. Numa ordenação por palavras, esse
código disputa o primeiro lugar com o 010701, que é o certo.

Se o emissor escolhesse sozinho, uma empresa de software poderia sair emitindo
toda nota com o código de rastreamento veicular — e **nada reclamaria**. O
código existe, tem seis dígitos, passa no XSD e passa na Sefin. Não haveria
rejeição para avisar: haveria o tributo errado em toda nota, indefinidamente.
Um campo em branco que a pessoa preenche é melhor que um campo preenchido que
ninguém confere.

Pela mesma razão o `config check` avisa em vez de recusar: a lista nacional é
lei federal e cresce, a cópia embutida é um anexo datado, e travar a emissão de
um serviço que o governo já publicou seria pior que os seis dígitos sem
conferência — a Sefin valida o código de qualquer forma, na recepção.

## Consequências

- O caminho do zero à primeira nota deixa de ter um campo sem resposta. Com
  `--servico`, o que sobra no `onboard` é a descrição do serviço — texto livre,
  que só a pessoa escreve.
- Nenhuma dependência nova, e +66 KB no binário.
- A busca fala a língua da lei, que nem sempre é a do dia a dia: "aula" não
  casa com nada porque a lista diz "ensino"; "software" não casa porque a lista
  diz "programa de computação". Por isso existe o `nfse servico listar`, que
  percorre os 41 itens — achar o grupo é o caminho de quem não acerta a palavra.
- Um anexo novo do governo se aplica com `go generate ./internal/domain/servico`.
  Até que isso aconteça, o teste de sincronia falha — de propósito.
- A documentação passa a ser conferida contra a lista: um teste em
  `internal/docs` recusa qualquer cTribNac citado no README ou no CHANGELOG que
  não exista de verdade.

## Alternativas consideradas

**Uma tabela de-para CNAE → cTribNac.** Seria preciso inventá-la: não existe
publicada. Uma tabela inventada tem a aparência de autoridade de uma tabela
oficial e seria copiada como tal.

**Deixar só o comando de busca, fora do `onboard`.** Era o desenho da
[issue #10](https://github.com/edusouza/nfse-emissor-go/issues/10). Foi
descartado porque o momento em que a pessoa precisa do código é exatamente o
momento em que o `onboard` termina — mandar procurar em outro comando é
devolver o problema com um mapa.

**Preencher com o melhor candidato e avisar.** Ver a seção acima. O aviso é
lido uma vez; o código fica na configuração para sempre.

**Perguntar num questionário interativo.** Continua adiada, pelo mesmo motivo
da ADR 0007.

## Aprendizado

O gerador recusa gravar um código sem descrição. Foi essa recusa que expôs um
defeito de leitura do `.xlsx`: um texto formatado em pedaços é gravado dividido
em *runs*, e o primeiro parser lia só o `<t>` direto — devolvendo string vazia
para as células estilizadas. Um gerador complacente teria escrito os 335 códigos
com buracos, o `lista.csv` teria carregado descrições em branco, e **todos os
testes do pacote teriam passado**, porque testam o CSV contra ele mesmo.

É a mesma forma dos defeitos da ADR 0008, num lugar novo: o que salvou foi
exigir que o dado fizesse sentido no momento em que ele é produzido, em vez de
verificar depois que ele existe.

E há uma diferença que vale anotar em relação a todos os defeitos anteriores
deste projeto: nenhum deles gerava rejeição. Este campo não estava errado —
estava **vazio**. Campo vazio não produz rejeição do governo; produz uma pessoa
que desiste antes da primeira nota, e isso não aparece em log nenhum.
