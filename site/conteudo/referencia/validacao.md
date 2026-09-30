# O que a validação local cobre

O `nfse` valida a DPS antes de assinar, em duas camadas. Tudo o que ele recusa
aqui é algo que a Sefin recusaria, e a recusa chega antes de o certificado ser
usado.

## O schema oficial, inteiro

A DPS é conferida contra os XSDs do pacote v1.01 do Sistema Nacional, os mesmos
que a Sefin usa, lidos dos próprios arquivos: ordem e cardinalidade dos
elementos, escolhas (`CNPJ` ou `CPF`, `vDR` ou `pDR`), enumerações, padrões,
tamanhos e a assinatura XML-DSig. Cada problema sai com o caminho do campo:

```
a DPS nao passou no schema oficial (XSD v1.01), e a Sefin a recusaria:
  - /DPS/infDPS/verAplic: "nfse—v0.9.0" nao segue o formato de TSString (...);
    o caractere '—' esta fora do Latin-1 que o schema aceita
```

O validador é Go puro, sem `cgo`, e implementa o subconjunto de XSD que esses
schemas usam. Se uma versão futura trouxer uma construção nova, ele se recusa
a carregar em vez de ignorá-la. Por quê:
[ADR 0014](../decisoes/0014-validacao-pelo-xsd.md).

## As regras de negócio que o schema não expressa

- valores monetários;
- as regras de alíquota do ISS que dependem só do regime do prestador:
  [E0595](rejeicoes.md#e0595), [E0600](rejeicoes.md#e0600),
  [E0621](rejeicoes.md#e0621) e [E0625](rejeicoes.md#e0625) — veja
  [alíquota do ISS e retenção](../guias/aliquota-e-retencao.md);
- o dígito verificador de CNPJ, CPF e código de município.

## O que fica de fora, de propósito

As parametrizações municipais. As regras que dependem do convênio do município
com o Sistema Nacional — como [E0635](rejeicoes.md#e0635) e
[E0640](rejeicoes.md#e0640) — só a Sefin conhece. Na lista de
[códigos de rejeição](rejeicoes.md), são as marcadas como **nível 3**.

A palavra final é sempre do governo. O trabalho para consultar essas regras
antes de enviar está na
[issue #5](https://github.com/edusouza/nfse-emissor-go/issues/5).
