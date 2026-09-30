# Configurar

O `nfse` guarda a configuração num `nfse.yaml`: quem você é, o certificado, o
ambiente e os padrões que se repetem em toda nota. Há duas formas de criá-lo.

## Em um comando

O `nfse onboard` monta a configuração a partir do que já se sabe sobre você. O
CNPJ sai do próprio certificado — o ICP-Brasil o grava numa extensão do A1,
a mesma que a Sefin lê para saber quem assina — e o resto vem do cadastro
público da Receita Federal:

```bash
export NFSE_CERT_SENHA='sua-senha'
nfse onboard --certificado certificado.pfx
```

```
Certificado  certificado.pfx
  Titular     EMPRESA EXEMPLO LTDA:12345678000195
  Valido ate  10/03/2027

Consultando o CNPJ 12.345.678/0001-95 no cadastro publico da Receita Federal, via brasilapi.com.br...
  Razao social  EMPRESA EXEMPLO LTDA
  Municipio     CURITIBA/PR (IBGE 4106902)
  Regime        mei
  Atividade     6209-1/00 Suporte tecnico, manutencao e outros servicos em tecnologia da informacao
  Situacao      ATIVA

Codigos de servico parecidos com a sua atividade (6209-1/00):
  010701  Suporte tecnico em informatica, inclusive instalacao...
  110501  Servicos relacionados ao monitoramento e rastreamento a...
Sao palpites a partir do texto do CNAE, nao um mapeamento oficial.
Confira com 'nfse servico ver <codigo>' ou procure com 'nfse servico buscar'.

nfse.yaml criado.

Falta preencher em nfse.yaml:
  - padroes.servico.codigo_tributacao_nacional — 6 digitos da lista nacional
    (LC 116/2003); os candidatos acima estao no arquivo, comentados (o mais
    proximo e 010701)
  - padroes.servico.descricao — o que voce presta

Depois:
  nfse config check
  nfse emitir --valor 100,00
```

O código IBGE do município — sete dígitos que ninguém sabe de cabeça — e a
razão social exata vêm prontos. Se você já sabe o código do serviço, passe
`--servico 010701` e ele sai conferido e comentado no arquivo. Se não sabe,
veja [achar o código do serviço](../guias/codigo-do-servico.md).

Sem o certificado em mãos, `--cnpj 12345678000195` faz o mesmo caminho.

### O que sai da sua máquina

A consulta manda **só o seu CNPJ** para um serviço de terceiros
([BrasilAPI](https://brasilapi.com.br), que serve os dados abertos da Receita) —
nada do certificado sai da máquina. O comando avisa antes de sair para a rede,
`--sem-rede` desliga a consulta, e `--fonte` aponta para outro servidor, para
quem roda a própria instância do
[minhareceita](https://docs.minhareceita.org). Se a consulta falhar, o arquivo
é gravado assim mesmo com o que o certificado informou.

```bash
nfse onboard --certificado certificado.pfx --sem-rede
```

Por quê: [ADR 0007](../decisoes/0007-preenchimento-da-configuracao.md).

### As perguntas no terminal

No terminal, o `onboard` pergunta o que a consulta não respondeu, e só isso:

```
Faltam alguns campos. Enter deixa o campo em branco, para preencher depois no arquivo.

Serie da DPS [00001]:
Codigo do servico (cTribNac, 6 digitos): 010701
  Suporte tecnico em informatica, inclusive instalacao, configuracao e manutencao...
Descricao padrao do servico, a que vai na nota: Suporte tecnico mensal
```

O regime tributário é explicado e nunca sugerido. Quando o cadastro não sabe,
sugerir um valor seria adivinhar. `--nao-interativo` desliga as perguntas, e
sem terminal (num script, por exemplo) elas nunca aparecem.

### O município

Para informar o município — quando a consulta está desligada, falhou, ou o
cadastro está desatualizado — use `--municipio`, com o código ou com o nome:

```bash
nfse onboard --certificado certificado.pfx --municipio "Curitiba/PR"
nfse onboard --certificado certificado.pfx --sem-rede --municipio 4106902
```

O código é conferido pelo dígito verificador do IBGE, sem rede. O nome precisa
da UF, porque há municípios com o mesmo nome em estados diferentes. Ele é
procurado na lista do estado no IBGE, e a lista inteira fica em cache: da
segunda vez em diante, qualquer município daquela UF funciona com
`--sem-rede`. Por quê: [ADR 0013](../decisoes/0013-municipio-informado-pelo-nome.md).

## À mão

Quem prefere preencher tudo à mão usa `nfse config init`, que escreve o mesmo
arquivo em branco e comentado:

```bash
nfse config init
$EDITOR nfse.yaml
```

## Conferir

Qualquer que seja o caminho, confira antes de emitir:

```bash
nfse config check
```

```
nfse.yaml esta valido.

  Ambiente    producao-restrita
  Prestador   EMPRESA EXEMPLO LTDA (12345678000195)
  Regime      mei
  Municipio   4106902
  Serie       00001
```

Se faltar algo, ele lista **tudo** que falta de uma vez, em vez de parar no
primeiro problema. Ele também confere o dígito verificador do código de
município que você digitar e diz o que os seis dígitos do código do serviço
significam.

## Próximo passo

[A primeira nota](primeira-nota.md).
