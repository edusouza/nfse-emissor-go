# Changelog

Todas as mudanças notáveis neste projeto serão documentadas neste arquivo.

O formato é baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.0.0/),
e este projeto adere ao [Versionamento Semântico](https://semver.org/lang/pt-BR/).

> **Sobre a numeração:** a `1.0.0` no fim deste arquivo é a API REST anterior,
> que nunca chegou a ser marcada com uma tag. A `0.5.0` é a **primeira versão
> publicada** deste repositório. O emissor em linha de comando começou do zero
> em `0.x` porque a primeira emissão real contra a Sefin ainda não foi
> confirmada — a `1.0.0` fica reservada para quando for.

## [Não lançado]

Fecha o último campo obrigatório que nenhuma consulta respondia: o código de
tributação nacional do serviço (`cTribNac`).

A pergunta que originou esta parte foi *"fiz o onboarding pelo certificado, mas
ficou faltando o código de tributação nacional — como usar o CNAE primário?"*.
Não dá: o CNAE classifica a atividade econômica da **empresa** para a Receita
Federal, e o `cTribNac` classifica o **serviço** para efeito de ISS, segundo a
LC 116/2003. Duas taxonomias, finalidades diferentes, nenhuma correspondência
oficial — o `ANEXO_B` tem 335 códigos e nenhuma coluna de CNAE. O que dá para
fazer é procurar junto, e é o que foi feito.

### Adicionado

- **A lista nacional de serviços embutida no binário** — os 335 códigos da
  LC 116/2003, gerados do `ANEXO_B` que já estava em `docs/anexos/` por um
  programa versionado junto. Sem dependência nova: um `.xlsx` é um zip de XML,
  e `archive/zip` mais `encoding/xml` bastam. A consulta é local e funciona sem
  rede. Ver [ADR 0009](docs/decisoes/0009-lista-de-servicos-embutida.md).
- **`nfse servico buscar <termo>`** — procura o código pela descrição do
  serviço, sem acento e no plural se for o caso. A busca pesa cada palavra pela
  raridade dela na lista: "serviços" e "congêneres" aparecem em quase todo
  código e não separam nada.
- **`nfse servico ver <codigo>`** e **`nfse servico listar [item]`** — o que um
  código significa, e os 41 itens da lei para quem não acerta a palavra que a
  lista usa ("aula" está lá como "ensino").
- **`nfse onboard --servico <codigo>`**, conferido contra a lista antes de
  gravar, com a descrição oficial escrita como comentário ao lado do código.
- **Sugestões de `cTribNac` no `onboard`**, ordenadas a partir do CNAE que o
  cadastro público informa. Vão para a tela e para o `nfse.yaml`
  **comentadas**, junto com o CNAE que as gerou e a ressalva de que são
  palpite — o campo continua vazio. O motivo está na ADR: a expressão
  "tecnologia da informação" aparece literalmente dentro de um código de
  rastreamento veicular, e um emissor que escolhesse sozinho poria esse código
  em toda nota de uma empresa de software **sem gerar rejeição nenhuma**.

### Alterado

- **`nfse config check` diz o que o código de serviço significa**, e avisa —
  sem recusar — quando o `cTribNac` configurado não está na lista embutida. A
  lista é lei federal e cresce; a cópia embutida é um anexo datado, e recusar
  um código que o governo já publicou seria pior que não conferir.
- A lista de pendências do `onboard` deixa de citar o código de serviço quando
  ele foi informado, e quebra linha em vez de estourar o terminal.

### Verificação

- Um teste regenera o `lista.csv` a partir do anexo e compara byte a byte: o
  arquivo embutido só vale enquanto reproduzir a planilha oficial.
- Um teste em `internal/docs` recusa qualquer `cTribNac` citado na documentação
  que não exista na lista — o mesmo tratamento que as chaves de acesso já
  tinham desde que o README publicou uma com 47 dígitos.

Fecha [#10](https://github.com/edusouza/nfse-emissor-go/issues/10).

## [0.6.0] - 2026-09-18

Menos digitação para começar. O emissor passa a montar a própria configuração a
partir do certificado A1 e do cadastro público de CNPJ, em vez de entregar um
arquivo em branco com cinco campos obrigatórios.

### Adicionado

- **`nfse onboard`** — grava um `nfse.yaml` já preenchido. O CNPJ e a razão
  social saem do certificado A1, que o ICP-Brasil emite com o titular no formato
  `RAZÃO SOCIAL:CNPJ`; o código IBGE do município e o regime tributário
  (MEI ou ME/EPP) vêm de uma consulta ao cadastro público da Receita Federal,
  servido pela [BrasilAPI](https://brasilapi.com.br). Sem o certificado em mãos,
  `--cnpj` faz o mesmo caminho. O que a máquina não descobre fica em branco e é
  listado no fim, com o campo e onde procurar.
  Ver [ADR 0007](docs/decisoes/0007-preenchimento-da-configuracao.md).
- **`--sem-rede`**, para gerar a configuração sem consultar serviço nenhum, e
  **`--fonte`**, para apontar a consulta a outro servidor — quem roda a própria
  instância do [minhareceita](https://docs.minhareceita.org) não precisa falar
  com terceiros. A consulta manda apenas o CNPJ, é anunciada na tela antes de
  acontecer, e o arquivo gerado registra no cabeçalho de onde veio cada valor.
- Uma falha na consulta **não interrompe** o comando: o arquivo é gravado com o
  que o certificado informou e o resto entra na lista de pendências. Uma
  indisponibilidade de terceiro não pode virar um beco sem saída para quem só
  quer começar.
- O `onboard` reaproveita `xmlsigner.CertificateInfo.SubjectCNPJ` e
  `SubjectHolderName`, que a 0.5.1 introduziu para conferir se o certificado é
  do prestador. A mesma leitura do titular serve para as duas coisas: recusar
  uma emissão com o certificado errado e preencher a configuração sem digitação.

### Alterado

- O erro de configuração ausente passa a sugerir `nfse onboard` antes de
  `nfse config init`.
- `nfse config init` continua como está, para quem prefere o modelo em branco.

### Problemas conhecidos

- O regime tributário só é preenchido quando o cadastro responde com certeza.
  Um "não é MEI" sem informação sobre o Simples deixa o campo em branco, com
  aviso: chutar poria um `opSimpNac` errado em toda nota emitida.
- O mapeamento dos campos da resposta do cadastro de CNPJ ainda não foi
  exercitado contra a API real — o ambiente de desenvolvimento não alcança
  `brasilapi.com.br`. Ver
  [#12](https://github.com/edusouza/nfse-emissor-go/issues/12).
- Sobra um campo obrigatório que nenhuma consulta responde:
  `codigo_tributacao_nacional`, o código do serviço na lista da LC 116/2003.
  Ver [#10](https://github.com/edusouza/nfse-emissor-go/issues/10).
  *Resolvido em [Não lançado](#não-lançado).*
## [0.5.2] - 2026-09-18

**A primeira versão que emitiu uma NFS-e de verdade.** Em produção restrita, o
ciclo inteiro foi exercitado contra a Sefin Nacional, ponta a ponta:

```
emitir → enviar → consultar <chave> → consultar --dps → cancelar
```

A DPS foi montada, validada, assinada, aceita e devolvida como nota autorizada,
com chave de acesso de 50 dígitos que o próprio validador do projeto aceita. O
cancelamento — outro XML, outra raiz, outro endpoint — foi registrado na
primeira tentativa.

Seis correções. As **quatro primeiras** eram tudo que impedia a emissão de
atravessar, cada uma encontrada por uma rejeição do governo, uma depois da
outra. As **duas últimas** vieram logo em seguida, de usar o que passou a
funcionar: consultar a nota recém-emitida.

Nenhuma delas teria sido encontrada por inspeção ou por cobertura de testes — a
suíte estava verde com as seis.

### Corrigido

- **O total de tributos era escolhido pelo valor configurado, não pelo regime.**
  A Sefin recusava ME/EPP com `[E0712] Para ME/EPP o indicador de informação de
  valor total de tributos não pode ser informado`.

  O `totTrib` é um *choice* de um filho só, e qual deles é permitido depende da
  situação do prestador no Simples Nacional:

  ```
  E0710 — para MEI,    pTotTribSN nunca pode ser informado
  E0712 — para ME/EPP, indTotTrib nunca pode ser informado
  ```

  O construtor escolhia por outro critério: mandava `pTotTribSN` se houvesse um
  percentual configurado e `indTotTrib` se não houvesse. Errava nos **dois**
  sentidos — um ME/EPP sem percentual declarava `indTotTrib`, e um MEI com
  percentual declarava `pTotTribSN`.

  Um ME/EPP que não sabe a própria alíquota declara `pTotTribSN` igual a zero:
  não existe `indTotTrib` para ele se abster, e o padrão do tipo `TSDec2V2` no
  XSD admite o zero.

  O `opSimpNac` passou a ser calculado num único lugar, lido pelas duas seções
  que dependem dele. Estava duplicado, e duas cópias de uma regra são duas
  chances de divergirem.

- **A razão social do prestador era enviada quando não devia.** A Sefin recusava
  com `[E0121] O nome ou razão social do prestador não deve ser informado quando
  o emitente da DPS for o próprio prestador`.

  As regras E0121 e E0122 formam um par:

  ```
  tpEmit = 1 (o prestador emite)  → xNome NÃO deve ser informado
  tpEmit = 2 ou 3                 → xNome DEVE ser informado
  ```

  O governo já sabe o nome pelo CNPJ quando é o próprio prestador que emite;
  mandá-lo mesmo assim é rejeição, não redundância. Como este CLI sempre emite
  como prestador, o `xNome` simplesmente deixa de ser montado — a condição fica
  no construtor do XML, não numa validação: um documento que não pode ser
  montado errado dispensa quem o confira depois.

  O `pkg/xmlbuilder` continua servindo os casos 2 e 3, e aí informa o nome.

- **O tipo de inscrição federal no identificador da DPS estava invertido.** A
  Sefin recusava com `[E0004] Conteúdo do identificador informado na DPS difere
  da concatenação dos campos correspondentes`.

  A regra oficial, na planilha `ANEXO_I`, é explícita:

  ```
  Tipo de inscrição Federal = 1 / Inscrição Federal = CPF emitente da DPS;
  Tipo de inscrição Federal = 2 / Inscrição Federal = CNPJ emitente da DPS;
  ```

  Os códigos são o **inverso** do que a ordem dos nomes sugere, e estavam
  trocados em dois pacotes (`pkg/dpsid` e `pkg/xmlbuilder`). Toda DPS de empresa
  saía com `1` onde o governo lê `2` — ou seja, toda DPS que este emissor
  existe para emitir.

  A validação interna também decidia por um literal (`if RegistrationType == 1`)
  em vez das constantes nomeadas, então ela concordava com o engano em vez de
  denunciá-lo. Agora decide pelas constantes, e recusa um tipo desconhecido em
  vez de tratá-lo como CPF.

- **A assinatura de toda DPS era inválida: o digest era calculado sem a
  declaração de namespace.** A Sefin recusava cada emissão com
  `[E0714] Arquivo enviado com erro na assinatura`.

  `CanonicalizeSigned` — a função sobre cuja saída o digest da referência é
  calculado — copiava o elemento para um documento novo antes de
  canonicalizá-lo. Isso o desliga dos ancestrais, e o `infDPS` perde o
  `xmlns` que herda do `DPS`:

  ```
  o que assinávamos:    <infDPS Id="DPS4106902...">
  o que o mundo assina: <infDPS xmlns="http://www.sped.fazenda.gov.br/nfse" Id="DPS4106902...">
  ```

  É o defeito do [ADR 0004](docs/decisoes/0004-assinatura-que-nao-verificava.md)
  voltando por outra porta: a correção de namespace tinha sido aplicada a
  `Canonicalize`, e `CanonicalizeSigned` destruía o contexto antes de chamá-la.
  Ver [ADR 0008](docs/decisoes/0008-digest-sem-namespace.md).

  **Toda DPS assinada por uma versão anterior é inválida** e precisa ser
  emitida de novo. Não há conserto no arquivo — a assinatura é parte do que o
  governo valida.

- **`nfse consultar --dps` imprimia o identificador em branco.** O `DpsGetResponse`
  do swagger marca `idDps` como obrigatório na resposta, e o serviço real não o
  envia. O teste que existia não podia ver isso: o *stub* dele foi escrito a
  partir do swagger, e aqui é o swagger que está errado.

  Não vale uma viagem de ida e volta para descobrir algo que o usuário acabou de
  digitar — o identificador consultado preenche a linha quando a resposta o
  omite. Há um teste novo fiel ao que o governo devolve de fato.
- **`nfse consultar` recusava consultar a mesma nota duas vezes**, com uma
  mensagem de emissão: *"o numero da DPS provavelmente ja foi usado. Use outro
  --numero"* — numa consulta, que não tem número de DPS nem a flag `--numero`.

  A guarda contra sobrescrita foi escrita para a emissão, onde repetir um número
  destrói um documento fiscal distinto. Uma consulta é idempotente e a NFS-e é
  imutável no governo: a segunda busca traz o mesmo documento. Recusar não
  protegia nada.

  O `consultar` passa a sobrescrever sem perguntar, e o `--sobrescrever` some
  dele por não ter mais o que fazer. A recusa continua onde ela protege algo —
  emissão e cancelamento —, agora com uma mensagem que não empresta o
  vocabulário da emissão a quem não é emissão.

### Adicionado

- Testes que amarram a escolha do `totTrib` ao regime nos quatro casos — MEI com
  e sem percentual, ME/EPP com e sem —, citando o texto das regras E0710 e
  E0712. O teste anterior afirmava o comportamento defeituoso: um MEI emitindo
  `pTotTribSN`.
- Teste que consulta a mesma nota duas vezes, e outro que verifica que a recusa
  genérica de sobrescrita não menciona `--numero`.
- Testes que amarram o `xNome` ao `tpEmit` nos três valores possíveis, citando o
  texto das regras E0121 e E0122, e que conferem que o prestador continua
  identificado pelo CNPJ.
- Testes que ancoram os códigos de tipo de inscrição no texto da regra E0004, e
  que verificam cada fatia do identificador — município, tipo, inscrição, série
  e número — na posição que a regra define.
- Testes de canonicalização ancorados numa **implementação externa**: a forma
  canônica esperada foi produzida pelo libxml2 (via lxml), não por este pacote,
  e o comentário registra o comando que a reproduz. Mais uma asserção que teria
  pegado o defeito sozinha: sem assinatura envelopada para remover,
  `CanonicalizeSigned` e `Canonicalize` precisam produzir bytes idênticos.

  O teste de ida e volta que o ADR 0004 introduziu não podia detectar isto: o
  verificador usa a mesma função do assinador, então os dois concordavam entre
  si e com mais ninguém.

## [0.5.1] - 2026-09-18

### Corrigido

- **Enviar com o certificado errado só falhava na Sefin, e a mensagem mandava
  procurar no lugar errado.** Quem apontasse `--cert` para o certificado
  descartável de `exemplos/` recebia `403` sem corpo de erro, e o emissor
  sugeria procurar proxy e firewall — quando a resposta estava na máquina: o
  certificado era de outro CNPJ.

  `emitir` e `enviar` passam a comparar o CNPJ do certificado com o do
  prestador **antes de abrir a conexão**, e a recusa nomeia os dois:

  ```
  erro: o certificado nao e do prestador desta nota:
    Certificado  12.345.678/0001-95 (EMPRESA EXEMPLO LTDA)
    Prestador    11.222.333/0001-81
  ```

  No `emitir` a checagem roda **antes de assinar**. Uma DPS assinada com o
  certificado errado não se salva reenviando com o certo: a assinatura dentro
  do XML é parte do que o governo valida, então o arquivo nasce morto. Antes,
  o emissor gravava esse arquivo sem reclamar.
- Um `403` sem envelope da Sefin passa a levantar primeiro a hipótese do
  certificado — não ser um A1 da ICP-Brasil, estar vencido, ou não pertencer a
  uma parte da nota. Proxy e firewall continuam citados, agora em segundo
  lugar, que é onde a probabilidade os coloca num endpoint de TLS mútuo.

### Adicionado

- `xmlsigner.CertificateInfo.SubjectCNPJ` e `SubjectHolderName`, que leem o
  titular do certificado. A ICP-Brasil escreve o portador de um e-CNPJ como
  `RAZÃO SOCIAL:CNPJ` no *common name*. Os dígitos verificadores são
  conferidos: um CN terminado em quatorze dígitos não é evidência suficiente.

### Atenção: a inscrição municipal pode ser a próxima

A regra **E0120** diz que, quando o prestador emite (`tpEmit = 1`) e **não** há
registro complementar do contribuinte no CNC do município, a inscrição municipal
**não deve** ser informada na DPS.

Não dá para saber isso localmente — depende do cadastro do município. E não há
regra nenhuma que **exija** o IM: informá-lo é condicionalmente um erro, omiti-lo
nunca é. Por isso o emissor continua respeitando o que estiver configurado, em
vez de decidir por conta própria.

Se a sua emissão parar em E0120, esvazie `prestador.inscricao_municipal` no
`nfse.yaml` — o campo é opcional.

### Nota sobre a verificação

A comparação só opina quando consegue ler o CNPJ do certificado. Só a
ICP-Brasil garante o formato `RAZÃO SOCIAL:CNPJ`; recusar tudo que fuja disso
travaria quem tem um certificado com outro leiaute. Na dúvida, o emissor cala
e deixa o governo decidir.

## [0.5.0] - 2026-09-18

Primeira versão publicada do emissor em linha de comando. Entrega o ciclo
completo de uma nota — **emitir, enviar, consultar, cancelar** — em um binário
único, sem `cgo` e sem serviço externo.

Substitui a API REST com worker, MongoDB e Redis, removida em
[ADR 0001](docs/decisoes/0001-cli-em-vez-de-api.md). Seis defeitos que
impediam qualquer emissão foram corrigidos no caminho; cada um está registrado
nos [ADRs](docs/decisoes/).

### Corrigido

- **A conexão com a Sefin falhava com `tls: no renegotiation`.** O governo não
  pede o certificado do cliente no handshake inicial: pede depois, numa
  renegociação TLS, que o `crypto/tls` do Go recusa por padrão. Toda emissão
  com `--enviar` parava aí, na primeira requisição real. O cliente agora aceita
  renegociação iniciada pelo servidor.

### Adicionado

- **`nfse enviar <arquivo.xml>`** — transmite uma DPS que já foi gerada e
  assinada, byte a byte. Fechava um buraco no fluxo de conferir antes de
  mandar: quem emitia sem `--enviar` e depois chamava `--enviar` não mandava o
  arquivo conferido, e sim um documento novo, com outro número de DPS e outro
  instante de emissão. O comando não assina nada e não mexe no contador; recusa
  um arquivo sem assinatura antes de gastar a viagem até a Sefin.
- **Validação da alíquota do ISS antes de assinar.** O `emitir` passa a recusar
  as combinações que a Sefin rejeita na recepção: MEI informando alíquota
  (E0600), ME/EPP do Simples informando alíquota sem retenção (E0625), ME/EPP
  com retenção sem informar alíquota ou abaixo de 1,8% (E0621), e qualquer
  alíquota acima de 5% (E0595). A mensagem diz o que fazer e cita o código da
  regra. As regras vêm da planilha oficial em `docs/anexos/`. Ver
  [ADR 0006](docs/decisoes/0006-validacao-de-aliquota-do-iss.md).
- **`--retencao`** (`nao` | `tomador` | `intermediario`) e
  `padroes.valores.retencao_issqn`, que alimentam `tpRetISSQN`. Sem eles não
  havia como declarar que o tomador retém o ISSQN — e, portanto, não havia como
  emitir o único caso em que declarar a alíquota é obrigatório.
- **`prestador.regime_apuracao`** (`sn` | `iss-municipio` | `fora-do-sn`), que
  alimenta `regApTribSN`. O padrão é `sn`, o caso de quem está dentro dos
  limites do Simples.
- **`docs/convenio-municipal.md`** — como consultar se o convênio de um
  município está ativo. Duas regras de alíquota (E0635 e E0640) dependem disso,
  e a resposta não está na máquina: o emissor não opina nesses casos.
- **Numeração automática da DPS.** `--numero` virou opcional: sem ele, o `nfse`
  segue a sequência da série. O último número usado fica em
  `.nfse-estado.json`, gravado de forma atômica e só depois do XML existir em
  disco — uma gravação que falha não queima um número.
- **`nfse numero ver` e `nfse numero definir`** — consultam e ajustam o
  contador, para quem migra de outro emissor ou precisa realinhar depois de
  emitir de outra máquina. Retroceder a contagem avisa sobre o risco de
  repetição.
- **`nfse cancelar <chave-de-acesso>`** — monta o pedido de registro de evento
  de cancelamento, assina e envia. Motivos: `erro-emissao`, `nao-prestado` e
  `outros`, correspondendo aos códigos 1, 2 e 9 de `TSCodJustCanc`.
  A justificativa precisa ter de 15 a 255 caracteres, como `TSMotivo` exige.
  Cancelar em produção pede confirmação no terminal.
- O assinador deixou de ser específico de DPS: `SignDocument` assina qualquer
  documento do sistema nacional pelo par elemento-raiz/elemento-com-Id.
- **`nfse consultar <chave-de-acesso>`** — busca a NFS-e na Sefin Nacional e
  grava o XML.
- **`nfse consultar --dps <id>`** — recupera a chave de acesso pelo
  identificador da declaracao, para reconciliar uma emissao interrompida.
  Com `--existe`, responde apenas se a nota foi gerada: o governo atende essa
  pergunta a qualquer certificado valido, enquanto a chave so e informada a
  quem consta na nota.
- Uma negativa por sigilo fiscal (403) e explicada em vez de repassada como
  status cru.
- Consultar nao exige mais uma configuracao completa de emissao: bastam o
  ambiente e o certificado. Quem so quer olhar uma nota nao precisa preencher
  CNPJ, municipio e regime tributario antes.
- Teste que valida as chaves de acesso mostradas na documentacao contra o
  proprio validador do `nfse`, para nao documentar exemplo que a ferramenta
  recusa.
- Um 403 sem corpo de erro da Sefin passa a levantar a hipotese de proxy ou
  firewall. Os dois casos sao identicos na linha de status, mas mandam o usuario
  para lugares opostos: um significa "este certificado nao e parte da nota", o
  outro "sua rede esta bloqueando a requisicao".
- **`nfse emitir --enviar`** — transmite a DPS assinada para a Sefin Nacional e
  grava a NFS-e autorizada, nomeada pela chave de acesso. A emissão é síncrona:
  o governo valida e devolve a nota ou a rejeição na mesma requisição.
- Rejeições trazem **todos** os motivos de uma vez, com código, descrição e
  complemento. Reportar um por vez custaria uma ida à rede por problema.
- Emitir em ambiente de produção pede confirmação no terminal, já que desfazer
  exige um pedido de evento de cancelamento. `--confirmar` dispensa a pergunta
  em scripts; sem terminal e sem a flag, o comando recusa.
- O ambiente reportado pelo governo na resposta é exibido: quem decide se a nota
  tem valor fiscal é ele, não o arquivo de configuração local.
- Especificações OpenAPI oficiais versionadas em [`docs/api/`](docs/api/).
- **Exemplo executável** em [`exemplos/`](exemplos/), em duas versões — shell e
  PowerShell: configuração pronta de um MEI, uma nota B2B e um script que gera
  certificado descartável. No Windows o script usa os cmdlets nativos, sem
  exigir OpenSSL. Os dados são
  fictícios mas consistentes — CNPJ com dígitos verificadores válidos, código do
  município conferido na tabela do IBGE e código do serviço na lista nacional.
- **Validação de CNPJ e CPF** na configuração e nos dados da nota. O
  `pkg/cnpjcpf` existia desde o início e nunca havia sido ligado ao caminho de
  emissão: o emissor aceitava qualquer sequência de dígitos e deixava a rejeição
  para o governo.
- `nfse emitir` **recusa sobrescrever** um arquivo já existente. O nome vem do
  identificador da DPS, que se repete quando série e número se repetem;
  sobrescrever em silêncio destruiria uma declaração assinada — ou, depois do
  envio, a única cópia local de uma nota que existe no governo. `--sobrescrever`
  permite a substituição deliberada.
- **CLI `nfse`** — binário único, sem `cgo`, instalável com
  `go install github.com/edusouza/nfse-emissor-go/cmd/nfse@latest`.
- `nfse versao` — versão, plataforma e versão do Go.
- `nfse cert info` — inspeciona um certificado A1 (PFX/P12): titular, emissor,
  validade, dias restantes, tamanho da chave e cadeia. Sai com código de erro
  quando o certificado não pode assinar uma DPS, e avisa quando faltam menos de
  30 dias para o vencimento.
- Resolução da senha do certificado por `--senha`, pela variável de ambiente
  `NFSE_CERT_SENHA` ou por prompt interativo, nessa ordem.
- Esteira de CI: formatação, `go mod tidy` limpo, `go vet`, build, testes com
  detector de corrida e relatório de cobertura.
- `nfse config init` — gera um `nfse.yaml` comentado; `nfse config check`
  verifica se está completo e coerente.
- `nfse emitir` — monta, valida e assina o XML da DPS. Os dados vêm da seção
  `padroes` do `nfse.yaml`, de um arquivo passado em `--yaml` e das flags, cada
  camada sobrescrevendo a anterior, de modo que quem sempre emite o mesmo tipo
  de serviço só precisa informar número e valor. `--sem-assinar` gera o XML sem
  assinatura, para inspeção.
- Campos desconhecidos no YAML são rejeitados: num emissor fiscal, um `aliquota_iss`
  digitado no lugar de `iss_aliquota` produziria uma nota errada em silêncio.
- Registro de decisões de arquitetura em [`docs/decisoes/`](docs/decisoes/).

### Corrigido

- **O cliente da Sefin falava um protocolo inexistente.** Era SOAP, com um
  envelope que o próprio código admitia ter inventado; a API é REST/JSON.
  Reescrito a partir da especificação oficial. A especificação corrigiu duas
  suposições que teriam quebrado tudo: `basePath` é `/SefinNacional` (sensível a
  maiúsculas) e `tipoAmbiente` é inteiro, não string. Revelou também que os
  erros vêm em duas formas — `erros` (array) na emissão e `erro` (objeto) nas
  consultas —, e um cliente que lesse só uma perderia o motivo da falha na
  outra. Ver [ADR 0005](docs/decisoes/0005-contrato-da-sefin-verificado.md).

- **A assinatura digital nunca verificou.** Três defeitos somados faziam com que
  toda DPS assinada fosse rejeitada: o documento era reindentado depois de
  assinado (invalidando digest e assinatura), a canonicalização perdia a
  declaração de namespace no ápice da subárvore, e a verificação exigia chave
  privada — que quem verifica nunca tem. Não existia nenhum teste que assinasse
  e depois verificasse; agora existe, cobrindo os três caminhos de assinatura e
  a detecção de adulteração.
  Ver [ADR 0004](docs/decisoes/0004-assinatura-que-nao-verificava.md).
- **O XML da DPS não seguia o schema oficial.** Sete divergências em relação a
  `DPS_v1.00.xsd`, entre elas `regEspTrib` e `tpRetISSQN` ausentes (ambos
  obrigatórios), descontos no elemento errado, `xDescServ` fora de `cServ`,
  `totTrib` no nível errado e com dois filhos onde o schema admite um, `subst`
  emitido como `<subst>2</subst>`, e base de cálculo e valor de ISS inventados
  dentro do elemento de benefício municipal — campos que a DPS não tem, porque
  quem os calcula é o governo.
  Ver [ADR 0003](docs/decisoes/0003-xml-conforme-o-xsd.md).
- O validador estrutural procurava `cTribNac`, `cLocPrest` e `vServPrest` em
  caminhos que não existem no schema, e exigia `subst`, que é opcional.
- **O campo `verAplic` estourava o limite do schema em toda DPS emitida.**
  `TSVerAplic` permite 20 caracteres; o emissor preenchia com `nfse-cli ` mais a
  pseudo-versão do Go — 49 caracteres. A Sefin rejeitaria a declaração por esse
  campo sozinho. Truncado, e agora verificado pelo validador estrutural antes da
  assinatura.
- **O README documentava uma chave de acesso com 47 dígitos**, escrita à mão e
  nunca conferida. Quem copiasse o exemplo recebia `a chave de acesso deve ter
  exatamente 50 digitos`. Corrigida e coberta por teste.
- **A validação da chave de acesso exigia um prefixo `NFSe` inexistente.** O
  XSD define `TSChaveNFSe` como `[0-9]{50}` e a especificação da Sefin diz o
  mesmo ("deve conter 50 números"): são 50 dígitos, sem prefixo. A regra
  anterior teria rejeitado toda chave real.

- **Certificados A1 em formato moderno não eram lidos.** `ParsePFX` usava
  `golang.org/x/crypto/pkcs12`, que só decodifica PKCS#12 com 3DES e MAC SHA-1.
  Arquivos gerados pelo OpenSSL 3 (AES-256-CBC, MAC SHA-256) — o padrão atual —
  falhavam com `unknown digest algorithm`, e a mensagem sugeria erroneamente que
  a senha estava errada. Migrado para `software.sslmate.com/src/go-pkcs12`, com
  teste de regressão cobrindo os dois formatos.
  Ver [ADR 0002](docs/decisoes/0002-parser-pkcs12.md).
- A cadeia de certificados intermediários passa a ser extraída do PFX; antes era
  descartada (`Chain: nil`).

### Alterado

- **O projeto deixou de ser uma API REST e passou a ser um CLI.**
  Ver [ADR 0001](docs/decisoes/0001-cli-em-vez-de-api.md).
- Módulo Go renomeado de `github.com/eduardo/nfse-nacional` para
  `github.com/edusouza/nfse-emissor-go`, agora coincidindo com o repositório —
  requisito para `go install` funcionar.
- Código Go movido de `src/` para a raiz do repositório, como é convenção em Go.
- Dependências diretas reduzidas de 12 para 5 (`etree`, `cobra`, `go-pkcs12`,
  `golang.org/x/term`, `yaml.v3`).
- Versão mínima do Go passou a 1.26, exigida por `golang.org/x/term` e pela
  cadeia do `cobra`. Manter 1.25 custaria rebaixar o `golang.org/x/crypto`, o
  que não compensa num programa que lida com certificado digital.
- Testes dependentes de relógio agora são pulados com `-short`, reduzindo a
  suíte local de ~75s para ~2s.

### Removido

- Cliente SOAP da Sefin e seus 2.561 testes, que passavam porque validavam o
  cliente contra um mock do mesmo contrato inventado.
- `internal/domain/query`, exceto a validação da chave de acesso: o restante
  eram DTOs e mapeamento de erros HTTP da API REST removida.

- API REST (Gin), worker assíncrono (Asynq), MongoDB, Redis, envio de webhooks,
  autenticação por chave de API, *rate limiting*, métricas Prometheus,
  health checks e `docker-compose.yml`.
- `internal/config`, que lia apenas variáveis de ambiente de servidor.
- Entidades de persistência em `internal/domain/entities.go`, mantidos apenas
  `Address` e `Values`, que são usados pelo domínio.

### Problemas conhecidos

- O envio nunca foi exercitado contra o ambiente real do governo: este ambiente
  de desenvolvimento não alcança `gov.br`. O cliente segue a especificação
  oficial e é coberto por testes, mas a primeira emissão de verdade em produção
  restrita ainda é uma verificação pendente.
- A validação chamada de "XSD" é estrutural e não lê os schemas oficiais. O tipo
  foi renomeado para `StructuralValidator` e o parâmetro `schemaDir`, que era
  ignorado, foi removido.
  Ver [#4](https://github.com/edusouza/nfse-emissor-go/issues/4).
- A alíquota de ISS é informada pelo usuário, sem consulta aos parâmetros
  municipais. Ver [#5](https://github.com/edusouza/nfse-emissor-go/issues/5).

---

## [1.0.0] - 2026-01-08

Versão da API REST, anterior à mudança para CLI. Mantida aqui como registro
histórico; o código correspondente está no histórico do git.

### Adicionado

- API REST para emissão de NFS-e com processamento assíncrono, autenticação por
  chave de API, *rate limiting*, webhooks de notificação e métricas Prometheus.
- Assinatura XMLDSig (RSA-SHA256, canonicalização exc-c14n) e leitura de
  certificados A1.
- Montagem do XML da DPS, cálculo de valores e tradução de códigos de rejeição.
