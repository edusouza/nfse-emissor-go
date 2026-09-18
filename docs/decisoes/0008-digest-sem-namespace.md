# 0008 — Três defeitos que só a emissão real revelou

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

## Um segundo defeito, encontrado do mesmo jeito

Com a assinatura aceita, a emissão seguinte parou em
`[E0004] Conteúdo do identificador informado na DPS difere da concatenação dos
campos correspondentes`.

O identificador da DPS conferia com os campos do próprio XML — município com
`cLocEmi`, inscrição com `prest/CNPJ`, série com `serie`, número com `nDPS`.
Era consistente consigo mesmo. A divergência era com a regra, que a planilha
enuncia assim:

```
Tipo de inscrição Federal = 1 / Inscrição Federal = CPF emitente da DPS;
Tipo de inscrição Federal = 2 / Inscrição Federal = CNPJ emitente da DPS;
```

Os códigos são o inverso do que a ordem dos nomes sugere, e estavam trocados em
dois pacotes. Pior: a validação decidia por um literal — `if RegistrationType
== 1` — em vez das constantes nomeadas, então ela concordava com o engano em
vez de denunciá-lo. Trocar as constantes sozinho deixaria a validação
consistente e errada.

Vale notar o formato do defeito. Não era uma regra sutil nem um canto escuro da
especificação: era uma linha de documentação, em português, na planilha que já
estava no repositório. O custo de não tê-la lido foi uma rejeição do governo.

## Um terceiro, no mesmo dia

Com o identificador aceito, a emissão seguinte parou em
`[E0121] O nome ou razão social do prestador não deve ser informado quando o
emitente da DPS for o próprio prestador`.

A planilha enuncia o par:

```
tpEmit = 1 (o prestador emite)  → xNome NÃO deve ser informado
tpEmit = 2 ou 3                 → xNome DEVE ser informado
```

O governo já sabe o nome pelo CNPJ quando é o próprio prestador que emite.
Mandá-lo é rejeição, não redundância — uma inversão da intuição de que
informação a mais não faz mal.

A correção ficou no construtor do XML, não numa validação. Um campo que só pode
existir sob uma condição é melhor não ser montado fora dela: o documento passa a
não poder ser construído errado, e não sobra nada para conferir depois.

## Aprendizado

Os três defeitos têm a mesma assinatura: o código era **internamente
consistente** e discordava de um documento. O digest concordava com o
verificador; o identificador concordava com os campos do próprio XML; o `xNome`
era um campo válido, preenchido com o valor certo. Nenhum teste podia falhar,
porque nenhum teste tinha por onde discordar.

O ADR 0004 disse que faltava um teste de ida e volta. Estava certo e era
insuficiente. Um teste de ida e volta prova **consistência interna**: que o
código concorda consigo mesmo. Não prova **conformidade**: que o código
concorda com a especificação.

Para tudo que é lido por outra parte — XML canônico, assinatura, contrato de
API — a única asserção que vale é contra algo que não saiu daqui. O XSD
versionado, o swagger oficial, a planilha de regras, e agora a saída de uma
segunda implementação. Cinco defeitos deste projeto foram encontrados assim, e
nenhum por inspeção ou por cobertura.

Há um corolário prático: **constantes nomeadas não se auto-verificam**. Um par
`RegistrationTypeCNPJ = 1 / RegistrationTypeCPF = 2` lê-se perfeitamente bem e
estava invertido. O que o prende à realidade é um teste que cita a regra, não o
nome da constante — e uma validação que decide pelas constantes, nunca por um
literal que pode discordar delas em silêncio.

Vale notar também o custo da hipótese confortável. A primeira explicação —
"certificado errado" — era plausível, tinha respaldo na regra oficial, e
consumiu duas tentativas do usuário antes de ser descartada. O que a derrubou
não foi pensar melhor: foi reemitir com o certificado certo e ver o mesmo erro.
Quando a hipótese é testável em um comando, testar vem antes de argumentar.
