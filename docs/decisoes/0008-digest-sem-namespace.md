# 0008 — O digest era calculado sem o namespace

**Status:** Aceita
**Data:** 2026-09-18

> Numeração: a 0007 pertence ao trabalho do `nfse onboard`, em revisão em
> paralelo. Esta decisão saiu antes por ser uma correção de emissão bloqueante.

## Contexto

A primeira emissão real contra a Sefin Nacional foi recusada:

```
erro: documento rejeitado pela Sefin Nacional
  - [E0714] Arquivo enviado com erro na assinatura.
```

A regra por trás do código, na planilha oficial
`docs/anexos/ANEXO_I-...xlsx`, é *"A assinatura deve ser feita com o certificado
digital do emitente da DPS"*. A primeira hipótese foi essa — certificado errado.
Estava errada: o usuário reemitiu com o A1 correto e a recusa se repetiu.

### O que o defeito era

`CanonicalizeSigned` é a função sobre cuja saída o digest da referência é
calculado. Ela começava assim:

```go
doc := etree.NewDocument()
doc.SetRoot(element.Copy())
root := doc.Root()
```

Copiar o elemento para um documento novo **o desliga dos ancestrais**. O
`infDPS` herda `xmlns="http://www.sped.fazenda.gov.br/nfse"` do `DPS` que o
contém; destacado, essa herança deixa de existir antes que alguém possa
procurá-la. O resultado:

```
o que assinávamos:   <infDPS Id="DPS4106902...">
o que o mundo assina: <infDPS xmlns="http://www.sped.fazenda.gov.br/nfse" Id="DPS4106902...">
```

Dois documentos diferentes, dois digests diferentes. O governo canonicaliza
conforme a especificação, chega a um valor, compara com o nosso e recusa.

Isto é exatamente o defeito do [ADR 0004](0004-assinatura-que-nao-verificava.md)
voltando por outra porta. Lá, a correção — materializar no ápice os namespaces
herdados — foi aplicada a `Canonicalize`. `CanonicalizeSigned` destrói o
contexto de ancestrais *antes* de chamá-la, então a correção não tinha o que
materializar.

### Por que a suíte estava verde

O verificador do projeto usa **a mesma** `CanonicalizeSigned`. Assinatura e
verificação concordavam uma com a outra e com mais ninguém. O teste de ida e
volta que o ADR 0004 introduziu — e que era a lição daquele ADR — não podia
detectar isto, porque compara o código com ele mesmo.

Cem por cento da suíte passava sobre uma assinatura que nenhuma outra
implementação aceita.

## Decisão

**Materializar os namespaces herdados enquanto o elemento ainda tem ancestrais**,
e só então destacá-lo:

```go
apex := materializeInheritedNamespaces(element).Copy()
removeSignatureElements(apex)
return Canonicalize(apex)
```

**Ancorar os testes num valor de fora.** Os novos testes de canonicalização
comparam a saída com uma forma canônica produzida pelo **libxml2 (via lxml)**,
não por este pacote, e o comentário registra o comando que a reproduz. Há
também uma asserção que teria pegado o defeito sozinha: sem assinatura
envelopada para remover, `CanonicalizeSigned` e `Canonicalize` têm de produzir
os mesmos bytes.

A correção foi confirmada fora do projeto: uma verificação XMLDSig completa
escrita em Python (lxml + cryptography) recusa o XML anterior no digest da
referência e aceita o novo.

## Consequências

- **Toda DPS assinada por uma versão anterior é inválida.** Não há conserto
  possível no arquivo: a assinatura é parte do que o governo valida. Quem tiver
  XML gerado antes desta versão precisa emitir de novo.
- A numeração da série avançou nessas tentativas. `nfse numero definir` realinha
  o contador quando fizer sentido.
- O `verAplic` de cada nota registra a versão do emissor, então dá para saber
  quais documentos saíram com o defeito.

## Aprendizado

O ADR 0004 disse que faltava um teste de ida e volta. Estava certo e era
insuficiente. Um teste de ida e volta prova **consistência interna**: que o
código concorda consigo mesmo. Não prova **conformidade**: que o código
concorda com a especificação.

Para tudo que é lido por outra parte — XML canônico, assinatura, contrato de
API — a única asserção que vale é contra algo que não saiu daqui. O XSD
versionado, o swagger oficial, a planilha de regras, e agora a saída de uma
segunda implementação. Cinco defeitos deste projeto foram encontrados assim, e
nenhum por inspeção ou por cobertura.

Vale notar também o custo da hipótese confortável. A primeira explicação —
"certificado errado" — era plausível, tinha respaldo na regra oficial, e
consumiu duas tentativas do usuário antes de ser descartada. O que a derrubou
não foi pensar melhor: foi reemitir com o certificado certo e ver o mesmo erro.
Quando a hipótese é testável em um comando, testar vem antes de argumentar.
