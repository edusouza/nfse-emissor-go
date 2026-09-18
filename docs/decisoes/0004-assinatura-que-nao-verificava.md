# 0004 — A assinatura digital nunca verificou

**Status:** Aceita
**Data:** 2026-09-18

## Contexto

O primeiro teste que assinou uma DPS e em seguida verificou a assinatura falhou.

O projeto tinha testes de assinatura e testes de verificação, todos passando.
O que não existia era um teste de **ida e volta**: os testes de verificação só
usavam fixtures negativas escritas à mão (XML sem assinatura, XML inválido,
`KeyInfo` ausente). Nenhum verificava algo que o próprio projeto tinha assinado.

Nessa lacuna cabiam três defeitos, cada um suficiente para o governo rejeitar
toda nota emitida.

### 1. O documento era reindentado depois de assinado

```go
dps.AddChild(signature)
doc.Indent(2)          // insere espaços em branco...
signedXML, _ := doc.WriteToString()
```

XML canônico é sensível a espaço em branco. `Indent` inseria nós de texto dentro
de `infDPS` e de `SignedInfo` **depois** que seus digests tinham sido calculados,
invalidando de uma vez o digest da referência e a própria assinatura.

### 2. A canonicalização perdia o namespace no ápice

`collectNamespacesInScope` começava a subida pelo próprio elemento, então a
declaração `xmlns` dele entrava no mapa de "já em escopo" e era suprimida na
saída. Um `<SignedInfo xmlns="http://www.w3.org/2000/09/xmldsig#">` assinado
era canonicalizado como `<SignedInfo>` pelado.

A correção não é só subir a partir do pai. Uma subárvore canonicalizada para
XMLDSig é tratada como um documento próprio: seu ápice não tem ancestrais de
saída, então **todo namespace que ele utiliza visivelmente precisa ser escrito
nele**, mesmo que o documento ao redor já declare a mesma URI acima. Foi o que
`materializeInheritedNamespaces` passou a fazer.

### 3. Verificar exigia chave privada

```go
certInfo := &CertificateInfo{Certificate: cert}   // extraído do KeyInfo
if err := v.CertificateValidator.Validate(certInfo); err != nil { ... }
```

`Validate` exigia `PrivateKey != nil`. Quem verifica uma assinatura só tem o
certificado público — a checagem tornava toda verificação impossível. Ela foi
movida para `ValidateForSigning`, onde a chave é de fato necessária.

## Decisão

Corrigir os três defeitos e adicionar `roundtrip_test.go`, que assina e verifica
pelos três caminhos públicos (`SignDPS`, `SignDPSCompact`, `SignDPSWithResult`)
e confirma que um documento adulterado é rejeitado.

## Consequências

- As assinaturas produzidas passam a verificar. Antes, nenhuma verificava.
- O XML assinado sai sem reindentação, então o bloco `<Signature>` fica compacto.
  É menos bonito e é o preço de estar correto; o conteúdo vai comprimido para a
  Sefin de qualquer forma.
- `Canonicalize` mudou de comportamento para qualquer subárvore aninhada. É a
  forma conforme ao exc-c14n, mas é uma mudança de comportamento e não apenas
  uma correção local.

## Aprendizado

Cobertura de teste alta não diz nada sobre o que **não** é testado. Assinar e
verificar estavam ambos cobertos; o que faltava era a composição dos dois —
exatamente onde moravam os três defeitos.

Para qualquer transformação com inversa (assinar/verificar, serializar/parsear,
cifrar/decifrar), o teste de ida e volta é o primeiro que deveria existir, não o
último. É barato, e é o único que exercita o contrato de verdade.

Vale registrar também que os três defeitos são silenciosos: o código não falha,
não loga, e devolve um XML de aparência perfeita. A única forma de perceber era
tentar verificar.
