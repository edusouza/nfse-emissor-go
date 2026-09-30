# Achar o código do serviço

O `cTribNac` é o único campo obrigatório que **nenhuma consulta responde**: ele
diz o que você presta, não quem você é. O CNAE não serve para deduzi-lo — o
CNAE classifica a atividade econômica da empresa para a Receita Federal, e o
`cTribNac` classifica o serviço para efeito de ISS pela LC 116/2003. São duas
taxonomias sem correspondência oficial, e o anexo do governo não tem coluna de
CNAE.

O que dá para fazer é procurar. Os 335 códigos da lista nacional estão
embutidos no binário, então a busca é local:

```bash
nfse servico buscar suporte tecnico
```

```
1 resultado para "suporte tecnico":

  010701  (subitem 1.07 da LC 116/2003)
    Suporte técnico em informática, inclusive instalação, configuração
    e manutenção de programas de computação e bancos de dados.
    item 1 — Serviços de Informática e congêneres.
```

## Quando a palavra não casa

A lista fala a língua da lei, que nem sempre é a do dia a dia: "aula" está lá
como *ensino*, "software" como *programa de computação*. Quando a busca não
acha, o caminho é pelos grupos:

```bash
nfse servico listar      # os 41 itens da lei
nfse servico listar 1    # os códigos de um item
nfse servico ver 010701  # um código, por extenso
```

## Depois de escolher

Ponha o código em `padroes.servico.codigo_tributacao_nacional` no `nfse.yaml`.
O `nfse config check` confirma o que os seis dígitos significam e avisa se o
código não estiver na lista que o binário carrega.

O `nfse onboard` sugere códigos a partir do texto do CNAE, mas nunca os
preenche: são palpites, não um mapeamento oficial. Por quê:
[ADR 0009](../decisoes/0009-lista-de-servicos-embutida.md).

!!! note "Código válido não é código aceito"
    Um código que existe na lista nacional ainda pode ser recusado se o
    município de incidência não o administra na data da competência — é a
    rejeição [E0312](../referencia/rejeicoes.md#e0312), que o anexo não aplica
    a MEI. É uma regra que depende do município, e só a Sefin a confere.
