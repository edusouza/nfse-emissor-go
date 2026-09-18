# Registro de decisões de arquitetura

Cada arquivo aqui descreve uma decisão que mudou o rumo do projeto: qual era o
problema, o que foi decidido, o que se perdeu com isso e o que aprendemos.

O formato é ADR (*Architecture Decision Record*). São curtos de propósito —
a intenção é que o próximo desenvolvedor (ou o mesmo daqui a seis meses)
entenda *por que* o código é como é sem precisar arqueologia no `git log`.

| # | Decisão | Status |
|---|---------|--------|
| [0001](0001-cli-em-vez-de-api.md) | CLI em vez de API REST | Aceita |
| [0002](0002-parser-pkcs12.md) | Troca do parser de PKCS#12 | Aceita |
| [0003](0003-xml-conforme-o-xsd.md) | Ancorar o gerador de XML no XSD oficial | Aceita |
| [0004](0004-assinatura-que-nao-verificava.md) | A assinatura digital nunca verificou | Aceita |
| [0005](0005-contrato-da-sefin-verificado.md) | Cliente da Sefin reescrito contra a especificação oficial | Aceita |
| [0006](0006-validacao-de-aliquota-do-iss.md) | Validar a alíquota do ISS antes de enviar | Aceita |
| [0007](0007-preenchimento-da-configuracao.md) | Preencher a configuração a partir do certificado e do cadastro público | Aceita |
