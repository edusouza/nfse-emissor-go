# 0007 — Preencher a configuração a partir do certificado e do cadastro público

**Status:** Aceita
**Data:** 2026-09-18

## Contexto

O primeiro contato com o emissor era um arquivo em branco. `nfse config init`
escrevia um `nfse.yaml` comentado e devolvia o usuário ao trabalho de preencher
cinco campos obrigatórios:

| Campo | Onde a pessoa encontra |
|---|---|
| `prestador.cnpj` | sabe de cabeça |
| `prestador.nome` | sabe de cabeça, mas precisa ser a razão social exata |
| `prestador.regime_tributario` | sabe, se souber a diferença entre MEI e ME/EPP |
| `prestador.municipio` | **código IBGE de 7 dígitos** — ninguém sabe de cabeça |
| `certificado.arquivo` | sabe |

O código do município é o pior deles: exige abrir uma planilha ou um site,
procurar a cidade e copiar sete dígitos. Um erro aqui não é detectado
localmente — a DPS é montada, assinada, enviada, e o governo rejeita. O custo
de começar era alto o suficiente para ser o obstáculo mais provável entre
instalar o binário e emitir a primeira nota.

Enquanto isso, dois dos cinco campos já estavam à mão da máquina:

- O certificado A1 traz o CNPJ e a razão social do titular. O ICP-Brasil grava
  o titular como `RAZÃO SOCIAL:CNPJ` no *common name* do certificado — é o
  mesmo arquivo que o emissor já lê para assinar.
- O resto (município e opção pelo Simples) é informação pública, publicada pela
  Receita Federal no cadastro aberto de CNPJ.

## Decisão

Um comando novo, `nfse onboard`, que monta o `nfse.yaml` já preenchido.

**O CNPJ sai do certificado.** `--certificado caminho.pfx` lê o titular e
extrai CNPJ e razão social. Os dígitos verificadores são conferidos antes de o
valor ser usado: um *common name* que termina em quatorze dígitos não é
evidência suficiente para preencher um documento fiscal. Quem ainda não tem o
A1 em mãos passa `--cnpj`.

**O resto vem de uma consulta ao cadastro público**, pela
[BrasilAPI](https://brasilapi.com.br) (`/api/cnpj/v1/{cnpj}`), que serve os
dados abertos da Receita Federal. De lá vêm razão social, `codigo_municipio_ibge`
— os sete dígitos que ninguém sabe de cabeça — e as opções pelo MEI e pelo
Simples, que decidem `regime_tributario`.

**A consulta é anunciada e pode ser desligada.** O CNPJ é público, mas a
consulta conta a um terceiro *quem está emitindo nota agora*. Então o comando
diz na tela para onde está mandando o número antes de sair para a rede,
`--sem-rede` desliga a consulta, e `--fonte` aponta para outro servidor — quem
roda a própria instância do [minhareceita](https://docs.minhareceita.org) não
precisa falar com ninguém. Nada além do CNPJ sai da máquina: nem senha, nem
chave privada, nem o arquivo do certificado.

**Uma falha na consulta não é uma falha do comando.** Se o serviço está fora do
ar, ou a rede está bloqueada, o `nfse.yaml` é gravado do mesmo jeito, com o que
o certificado informou, e o que faltou vai listado no fim. Transformar uma
indisponibilidade de terceiro em beco sem saída seria trocar um minuto de
digitação por nenhum caminho.

**O regime tributário nunca é adivinhado.** O cadastro responde três coisas
diferentes: sim, não, e *não sei* (`null`). Quando a resposta é "não é MEI" e
nada se sabe sobre o Simples, o campo fica em branco com um aviso. Chutar aqui
poria um `opSimpNac` errado em toda nota emitida, e o erro só apareceria na
rejeição.

`nfse config init` continua existindo, sem consulta e sem certificado, para quem
prefere o modelo em branco.

## Consequências

- O caminho do zero à primeira nota passa de "preencha cinco campos, dois deles
  pesquisando" para "rode um comando e escolha o código do serviço".
- Sobra **um** campo que a máquina não tem como descobrir:
  `codigo_tributacao_nacional`, o código do serviço na lista da LC 116/2003.
  Ele depende do que a pessoa faz, não de quem ela é. Está registrado na
  [issue #10](https://github.com/edusouza/nfse-emissor-go/issues/10).
- Nenhuma dependência nova: o cliente é `net/http` e `encoding/json`. Num
  binário que lida com certificado digital, cada dependência é superfície.
- O emissor passa a ter um caminho de rede que não é o governo. Ele é opcional,
  desligável, e nunca bloqueia a emissão — a configuração gerada é um arquivo de
  texto comum, que continua valendo se a BrasilAPI sumir amanhã.
- O mapeamento dos campos da resposta **não foi exercitado contra a API real**:
  o ambiente onde este código foi escrito não alcança `brasilapi.com.br`. Os
  nomes vêm da documentação do serviço e os testes usam um servidor local. É a
  mesma pendência de verificação que a primeira emissão em produção tem, e está
  na mesma lista de problemas conhecidos do CHANGELOG.

## Alternativas consideradas

**Embutir a tabela do IBGE.** O `ANEXO_A` com os códigos de município já está
versionado em `docs/anexos/`. Daria para perguntar "cidade e UF?" e resolver o
código offline. Foi descartado como primeira opção porque resolve um campo dos
quatro, e ainda exige que a pessoa digite algo — enquanto a consulta responde
razão social, município e regime de uma vez, a partir de um número que o
certificado já tem. A tabela continua sendo a saída natural para melhorar o
`--sem-rede` depois.

**Perguntar tudo num questionário interativo.** Adiada. Um assistente que
pergunta oito coisas ainda é oito perguntas; o ganho real estava em não
precisar perguntar. As perguntas que sobrarem — série, descrição padrão — cabem
num segundo passo, quando o comando tiver provado que a parte automática
funciona.

## Aprendizado

O dado mais difícil de preencher já estava dentro do arquivo que o programa
abria em toda emissão. O certificado A1 não é só uma chave: é um documento de
identidade emitido por uma autoridade, com o CNPJ do titular gravado por quem
conferiu os papéis. O emissor lia esse arquivo desde a primeira versão — só
nunca tinha perguntado a ele quem era o usuário.
