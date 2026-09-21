# 0010 — Embutir a tabela do IBGE para o `--sem-rede`

**Status:** Aceita
**Data:** 2026-09-21

## Contexto

Duas decisões anteriores falaram desta tabela e chegaram a respostas opostas.

A [ADR 0007](0007-preenchimento-da-configuracao.md) descartou embuti-la:

> Foi descartado como primeira opção porque resolve um campo dos quatro, e
> ainda exige que a pessoa digite algo — enquanto a consulta responde razão
> social, município e regime de uma vez. **A tabela continua sendo a saída
> natural para melhorar o `--sem-rede` depois.**

A [ADR 0009](0009-lista-de-servicos-embutida.md) embutiu a lista de serviços e
justificou a diferença numa tabela cujo critério era "existe consulta que
responde?" — sim para o município, não para o serviço.

O critério estava certo e incompleto: ele vale para o **caminho padrão**. O
`--sem-rede` é, por definição, o caminho onde consulta nenhuma responde. Quem
usa a flag — ou quem está atrás de um firewall, ou tem a BrasilAPI fora do ar —
fica sem o campo mais chato de todos: sete dígitos que ninguém sabe de cabeça,
e cujo erro não aparece localmente. A DPS é montada, assinada, enviada, e a
nota vai para o município errado com a Sefin aceitando.

Depois que a v0.6.0 fechou o `cTribNac`, o município passou a ser o único campo
que o `--sem-rede` deixava em branco sem ter como ajudar.

## Decisão

**A tabela do IBGE passa a ser embutida no binário** — 5570 municípios, 132 KB,
gerados do `ANEXO_A` por um programa versionado junto, com um teste que refaz a
geração e compara byte a byte. É a mesma mecânica da ADR 0009, e sem
dependência nova.

```
nfse onboard --sem-rede --municipio "Curitiba/PR"
nfse municipio buscar "bom jesus"
nfse municipio ver 4106902
```

O nome é aceito como as pessoas escrevem: com ou sem acento, em qualquer caixa,
com a UF separada por `/`, ` - ` ou `,`, ou sem UF nenhuma. Pontuação é
descartada por inteiro em vez de normalizada, o que faz de
`Alta Floresta D'Oeste`, `Alta Floresta D Oeste` e `alta floresta doeste` uma
chave só — nenhum par de municípios do mesmo estado colide assim, e um teste
segura isso.

**Nome ambíguo nunca vira escolha.** 232 nomes se repetem entre UFs. O comando
para e mostra os candidatos, porque escolher um poria a nota no município
errado e nada reclamaria.

**Entre `--municipio` e o cadastro público, vale a flag** — é o que a pessoa
digitou, agora, de propósito — mas a divergência é avisada. O cadastro conhece
o endereço que a Receita tem em arquivo, e discordância costuma significar que
um dos dois está desatualizado.

## O que o anexo não tinha

A coluna **"Sigla UF" do `ANEXO_A` está incompleta**: preenchida em 450 das
5570 linhas, e vazia em *todas* as linhas de 20 das 27 unidades federativas —
quem montou a planilha parou de preencher depois do Tocantins.

A sigla é, portanto, a única coisa que teve de vir de fora. Ela está numa
tabela de 27 entradas no gerador, e não é aceita de boa fé: o gerador a
verifica de três maneiras, e falha se qualquer uma discordar.

1. Todo código tem de começar com um prefixo que esteja na tabela.
2. Todas as linhas de um mesmo nome de UF têm de compartilhar um prefixo.
3. Onde o anexo **informa** a sigla, ela tem de ser a mesma da tabela.

O terceiro é o que importa: das 27 siglas, 7 são confirmadas pelo próprio
documento oficial, e as outras 20 estão amarradas ao prefixo do código IBGE,
que o anexo traz completo.

## Consequências

- **O `--sem-rede` deixa de ter buraco que o emissor possa tapar.** Com o
  certificado, `--municipio` e `--servico`, o único campo que sobra é
  `prestador.regime_tributario` — que é justamente o que a ADR 0007 decidiu não
  adivinhar. O que falta ali falta por escolha, não por falta de dado.
- +132 KB no binário, e nenhuma dependência nova.
- O `nfse municipio ver` resolve nos dois sentidos, código → nome e nome →
  código, o que serve para conferir um `nfse.yaml` que veio de outro lugar.
- Um anexo novo se aplica com `go generate ./internal/domain/municipio`. Até
  lá, o teste de sincronia falha de propósito.
- A dobra de acentos saiu do pacote `servico` para `internal/domain/texto`,
  compartilhada pelos dois. Duas cópias da mesma tabela significariam consertar
  uma e publicar a outra.

## Alternativas consideradas

**Continuar sem.** É o que a ADR 0007 decidiu, e o argumento dela era o caminho
padrão. Não responde ao `--sem-rede`, que existe exatamente para quem não quer
ou não pode consultar.

**Perguntar a cidade e a UF interativamente.** Continua sendo a
[issue #14](https://github.com/edusouza/nfse-emissor-go/issues/14), e agora fica
mais fácil: com a tabela embutida, a pergunta tem como validar a resposta na
hora em vez de aceitar sete dígitos quaisquer.

**Resolver por CEP ou por geolocalização.** Traria de volta a rede, que é
exatamente o que a flag desliga.

## Aprendizado

Um critério de decisão pode estar certo e ainda assim ser estreito demais.
"Existe consulta que responde isso?" era a pergunta certa na ADR 0009, mas a
resposta depende de qual caminho se está olhando — e o projeto tinha um caminho
declarado, com flag e tudo, em que a resposta era não.

E o dado oficial veio incompleto de novo, numa coluna que ninguém teria
conferido: a sigla da UF. O que salvou não foi desconfiar do documento, foi
cruzá-lo com ele mesmo — o prefixo do código, o nome da UF e as 450 linhas que
alguém chegou a preencher dizem a mesma coisa três vezes, e uma tabela escrita
à mão só entra no lugar que as três deixam vazio.
