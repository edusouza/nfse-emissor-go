# A chave de acesso

A chave de acesso identifica uma NFS-e emitida. São **50 dígitos**, sem
prefixo, compostos assim:

| Posições | Campo | Exemplo |
|---|---|---|
| 1–7 | Código IBGE do município | `4106902` |
| 8 | Ambiente gerador (1 = sistema próprio, 2 = Sefin Nacional) | `2` |
| 9 | Tipo de inscrição (1 = CPF, 2 = CNPJ) | `2` |
| 10–23 | Inscrição federal (CPF preenchido com zeros à esquerda) | `12345678000195` |
| 24–36 | Número da NFS-e | `0000000000001` |
| 37–40 | Ano e mês da emissão | `2608` |
| 41–49 | Código numérico aleatório | `123456789` |
| 50 | Dígito verificador | `0` |

O guia do emissor público diz "44 dígitos", mas o próprio exemplo que ele
apresenta tem 50, e a soma dos campos acima também. O XSD é a fonte:
`TSChaveNFSe` restringe o tipo a `[0-9]{50}`.

Ela é o argumento de `nfse consultar` e `nfse cancelar`, a opção
`--substitui` de `nfse emitir`, e o nome do arquivo em que a nota é gravada.
