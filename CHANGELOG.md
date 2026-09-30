# Changelog

Todas as mudanças notáveis neste projeto serão documentadas neste arquivo.

O formato é baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.1.0/),
e este projeto adere ao [Versionamento Semântico](https://semver.org/lang/pt-BR/).

> **Sobre a numeração:** a `1.0.0` no fim deste arquivo é a API REST anterior,
> que nunca chegou a ser marcada com uma tag. A `0.5.0` é a **primeira versão
> publicada** deste repositório. O emissor em linha de comando começou do zero
> em `0.x` porque a primeira emissão real contra a Sefin ainda não foi
> confirmada — a `1.0.0` fica reservada para quando for.

## [Não lançado]

### Adicionado

- Especificação OpenAPI oficial da API de Parâmetros Municipais do ADN em `docs/api/adn-parametrizacao-swagger.json` ([#5](https://github.com/edusouza/nfse-emissor-go/issues/5)).
- `docs/convenio-municipal.md` documenta o que o swagger não diz, conferido no serviço real: o código completo do serviço (`01.07.01.000`), a competência em `AAAA-MM-DD`, a alíquota em percentual e os dois formatos de erro ([#5](https://github.com/edusouza/nfse-emissor-go/issues/5)).

### Corrigido

- `docs/convenio-municipal.md` ensinava os caminhos do manual em PDF (`/parametros_municipais/...`), que não existem na API. Agora segue o swagger ([#5](https://github.com/edusouza/nfse-emissor-go/issues/5)).

## [0.9.0] - 2026-09-30

O `onboard` completa o `nfse.yaml` no terminal e aceita o município pelo nome, o CNPJ do certificado passa a ser lido de onde a Sefin o lê, e a DPS passa a ser validada contra os XSDs oficiais.
Ver [ADR 0013](docs/decisoes/0013-municipio-informado-pelo-nome.md), [ADR 0014](docs/decisoes/0014-validacao-pelo-xsd.md) e [ADR 0015](docs/decisoes/0015-ibs-cbs-em-2027.md).

### Adicionado

- No terminal, o `nfse onboard` pergunta só o que o certificado e o cadastro público não responderam ([#14](https://github.com/edusouza/nfse-emissor-go/issues/14)).
- O regime tributário é explicado e nunca sugerido; se o cadastro diz que a empresa está fora do Simples, ele nem é perguntado ([#14](https://github.com/edusouza/nfse-emissor-go/issues/14)).
- `--nao-interativo` mantém o `onboard` sem perguntas, que continua sendo o padrão quando não há terminal ([#14](https://github.com/edusouza/nfse-emissor-go/issues/14)).
- O `onboard` preenche `padroes.servico.descricao` ([#14](https://github.com/edusouza/nfse-emissor-go/issues/14)).
- `nfse onboard --municipio` aceita o código IBGE, conferido sem rede, ou "Cidade/UF", resolvido pela lista do IBGE e guardado em cache ([#11](https://github.com/edusouza/nfse-emissor-go/issues/11)).
- Opções `--fonte-municipios` e `--cache-municipios` no `onboard` ([#11](https://github.com/edusouza/nfse-emissor-go/issues/11)).
- `pkg/codmun` confere um código de município pelo prefixo da UF e pelo dígito verificador, sem embutir a tabela ([#12](https://github.com/edusouza/nfse-emissor-go/issues/12)).
- Testes de contrato contra a BrasilAPI e o IBGE (`NFSE_TESTE_CONTRATO=1`), rodados por um workflow semanal ([#12](https://github.com/edusouza/nfse-emissor-go/issues/12)).
- O `emitir` avisa quando a competência é de 2027 em diante, quando o grupo IBS/CBS passa a ser obrigatório no Simples ([#21](https://github.com/edusouza/nfse-emissor-go/issues/21)).

### Modificado

- O CNPJ do certificado passa a ser lido do `subjectAltName` (OID 2.16.76.1.3.3), onde a Sefin o lê; o *common name* vira reserva ([#13](https://github.com/edusouza/nfse-emissor-go/issues/13)).
- Um certificado com CNPJs divergentes ou ilegíveis na extensão é recusado como ambíguo, sem escolher um lado ([#13](https://github.com/edusouza/nfse-emissor-go/issues/13)).
- A DPS passa a ser validada contra os XSDs oficiais do pacote v1.01, por um validador em Go puro, com todos os problemas de uma vez ([#4](https://github.com/edusouza/nfse-emissor-go/issues/4)).

### Removido

- O validador estrutural escrito à mão (`validation.StructuralValidator`), substituído pela validação pelo XSD ([#4](https://github.com/edusouza/nfse-emissor-go/issues/4)).
- Os montadores de endereço sem uso de `pkg/xmlbuilder` (`BuildAddressXML`, `BuildNationalAddressXML`, `BuildForeignAddressXML`, `AddressFromDomain`) ([#4](https://github.com/edusouza/nfse-emissor-go/issues/4)).

### Corrigido

- `--deducoes` escrevia `vDR` e `pDR` juntos, e a Sefin recusaria a DPS ([#4](https://github.com/edusouza/nfse-emissor-go/issues/4)).
- O endereço do tomador em `pkg/xmlbuilder` saía fora do leiaute do `TCEndereco` ([#4](https://github.com/edusouza/nfse-emissor-go/issues/4)).
- O `onboard` gravava o código de município do cadastro sem conferir o dígito verificador e a UF ([#12](https://github.com/edusouza/nfse-emissor-go/issues/12)).
- O `config check` e o `emitir` conferiam só o tamanho de `prestador.municipio` e `servico.municipio_prestacao`, e não o dígito verificador ([#11](https://github.com/edusouza/nfse-emissor-go/issues/11)).
- O cache de municípios, também usado pelo `nfse danfse`, aceitava entradas adulteradas sem conferir código e UF ([#11](https://github.com/edusouza/nfse-emissor-go/issues/11)).
- A conferência do `emitir` e do `enviar` comparava o CNPJ do *common name*, e não o da extensão, que é o que a Sefin compara ([#13](https://github.com/edusouza/nfse-emissor-go/issues/13)).
- O `nfse.yaml` gerado pelo `onboard` podia sair ilegível com um caractere que o YAML não aceita, vindo do cadastro ou do certificado ([#14](https://github.com/edusouza/nfse-emissor-go/issues/14)).

## [0.8.0] - 2026-09-29

O DANFSe volta, agora gerado aqui: a NT 008 suspendeu a API do governo em 03/08/2026 e passou a geração para quem emite.
Ver [ADR 0011](docs/decisoes/0011-bibliotecas-de-pdf-e-qr-code.md) e [ADR 0012](docs/decisoes/0012-municipio-por-consulta.md).

### Adicionado

- `nfse danfse <arquivo-da-nfse.xml>` desenha o PDF do DANFSe a partir do XML autorizado, conforme a NT 008 v1.02, em uma página A4 ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)).
- QR Code da consulta pública desenhado como vetor ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)).
- Tarja "NFS-e SEM VALIDADE JURÍDICA" quando `tpAmb = 2`, como a NT exige ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)).
- Informações complementares na ordem da NT, sempre com os totais aproximados de tributos da Lei 12.741/2012 ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)).
- Canhoto de recebimento, que `--sem-canhoto` omite ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)).
- Marcas d'água `CANCELADA` e `SUBSTITUÍDA`, por `--cancelada` e `--substituida` ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)).
- Blocos de prestador, tomador, destinatário e intermediário, com a frase da NT para quem não foi identificado ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)).
- Dados de quem emitiu vêm do grupo `emit` da NFS-e quando a DPS não os traz ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)).
- Local da prestação e município de incidência no formato "Município / UF / País" ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)).
- Nome do município por consulta ao IBGE, com cache em disco, `--sem-rede` e sem nunca impedir o documento ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)) ([ADR 0012](docs/decisoes/0012-municipio-por-consulta.md)).
- O `danfse` confere a assinatura do governo e avisa quando ela falta ou não confere ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)).
- Aviso quando um texto tem caracteres fora da fonte do PDF, em vez de sumirem em silêncio ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)).
- Dependências `go-pdf/fpdf` e `boombuler/barcode`, ambas sem `cgo` ([ADR 0011](docs/decisoes/0011-bibliotecas-de-pdf-e-qr-code.md)).
- Consultas à Sefin tentam de novo, até 3 vezes, quando a conexão nem chega a abrir; cada tentativa é anunciada ([ADR 0005](docs/decisoes/0005-contrato-da-sefin-verificado.md)).

### Modificado

- Abrir a conexão com a Sefin tem prazo próprio de 10 s, em vez do padrão do sistema operacional ([ADR 0005](docs/decisoes/0005-contrato-da-sefin-verificado.md)).
- Uma falha de conexão diz se a requisição chegou ao governo; no `emitir --enviar`, mostra o `nfse enviar` que manda a mesma DPS ([ADR 0005](docs/decisoes/0005-contrato-da-sefin-verificado.md)).
- A consulta de existência (`consultar --dps --existe`) manda `Accept: application/json`, como as outras ([ADR 0005](docs/decisoes/0005-contrato-da-sefin-verificado.md)).

### Corrigido

- O `danfse` só recusava um PDF já existente depois de consultar o IBGE e desenhar o documento ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)).
- A consulta ao IBGE esperava o prazo de novo a cada bloco quando o serviço estava fora ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)).
- O cache de municípios podia ficar pela metade numa interrupção; agora é gravado de forma atômica ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)).
- A descrição do serviço sumia pela direita da página; agora quebra em linhas dentro do quadro ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)).
- PIS/COFINS retidos com `tpRetPisCofins` 3 a 9 saíam como débito próprio e fora do total retido ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)).
- Quebras de linha e tabulações do XML caíam em campos de uma linha só ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)).
- Um valor maior que o schema permite estourava o inteiro e imprimia um total errado ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)).
- XML maior que 5 MB passa a ser recusado ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)).

## [0.7.0] - 2026-09-21

Fecha a substituição de NFS-e; o `nfse danfse` pela API do governo foi removido antes do lançamento, quando a NT 008 a suspendeu.
Ver [ADR 0010](docs/decisoes/0010-substituicao-e-danfse.md).

### Adicionado

- `nfse emitir --substitui <chave> --motivo <nome>` emite uma nota que substitui outra; `--motivo-texto` acrescenta a descrição ([ADR 0010](docs/decisoes/0010-substituicao-e-danfse.md)).
- Motivos de substituição com nomes próprios (`saiu-do-simples`, `recusada-pelo-tomador` etc.), para não se confundirem com os do `cancelar` ([ADR 0010](docs/decisoes/0010-substituicao-e-danfse.md)).
- Pacote de schemas v1.01 em `docs/schemas/`: a 1.01 só acrescenta o grupo opcional `IBSCBS`, e a DPS 1.00 continua válida ([#21](https://github.com/edusouza/nfse-emissor-go/issues/21)).
- NT 008 v1.02 em `docs/notas-tecnicas/`, com o leiaute do DANFSe ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)).

### Modificado

- A lista nacional de serviços embutida passou para a v1.01: 338 códigos, com `141403`, `141404` e `200102` novos ([#10](https://github.com/edusouza/nfse-emissor-go/issues/10)).
- A validação do `subst` confere o tipo da chave e o motivo pela enumeração do XSD, não só a presença ([ADR 0010](docs/decisoes/0010-substituicao-e-danfse.md)).
- O ANEXO I de regras em `docs/anexos/` passou de 322 para 441 códigos de rejeição ([#21](https://github.com/edusouza/nfse-emissor-go/issues/21)).

### Removido

- `nfse danfse` pela API do Ambiente de Dados Nacional, suspensa pela NT 008 antes do lançamento ([#22](https://github.com/edusouza/nfse-emissor-go/issues/22)) ([ADR 0010](docs/decisoes/0010-substituicao-e-danfse.md)).

## [0.6.0] - 2026-09-21

O emissor monta a própria configuração a partir do certificado A1 e do cadastro público de CNPJ, e ajuda a achar o código de serviço (`cTribNac`).
Ver [ADR 0007](docs/decisoes/0007-preenchimento-da-configuracao.md) e [ADR 0009](docs/decisoes/0009-lista-de-servicos-embutida.md).

### Adicionado

- `nfse onboard` grava um `nfse.yaml` preenchido com o que o certificado e o cadastro público informam, e lista o que falta ([ADR 0007](docs/decisoes/0007-preenchimento-da-configuracao.md)).
- `--cnpj`, `--sem-rede` e `--fonte` no `onboard`; o arquivo registra no cabeçalho de onde veio cada valor ([ADR 0007](docs/decisoes/0007-preenchimento-da-configuracao.md)).
- Uma falha na consulta ao cadastro não interrompe o `onboard`: o que falta vai para a lista de pendências ([ADR 0007](docs/decisoes/0007-preenchimento-da-configuracao.md)).
- Lista nacional de serviços (LC 116/2003) embutida no binário, gerada do `ANEXO_B` ([#10](https://github.com/edusouza/nfse-emissor-go/issues/10)) ([ADR 0009](docs/decisoes/0009-lista-de-servicos-embutida.md)).
- `nfse servico buscar <termo>` procura o código pela descrição, sem acento e com peso por raridade das palavras ([#10](https://github.com/edusouza/nfse-emissor-go/issues/10)).
- `nfse servico ver <codigo>` e `nfse servico listar [item]` ([#10](https://github.com/edusouza/nfse-emissor-go/issues/10)).
- `nfse onboard --servico <codigo>`, conferido contra a lista, com a descrição oficial como comentário ([#10](https://github.com/edusouza/nfse-emissor-go/issues/10)).
- Sugestões de `cTribNac` a partir do CNAE do cadastro, gravadas comentadas no `nfse.yaml`: são palpites, e o campo fica vazio ([#10](https://github.com/edusouza/nfse-emissor-go/issues/10)) ([ADR 0009](docs/decisoes/0009-lista-de-servicos-embutida.md)).

### Modificado

- O erro de configuração ausente sugere `nfse onboard` antes de `nfse config init` ([ADR 0007](docs/decisoes/0007-preenchimento-da-configuracao.md)).
- `nfse config check` diz o que o código de serviço significa e avisa, sem recusar, quando ele não está na lista embutida ([#10](https://github.com/edusouza/nfse-emissor-go/issues/10)).
- A lista de pendências do `onboard` não cita o código de serviço quando ele foi informado, e quebra linha no terminal ([#10](https://github.com/edusouza/nfse-emissor-go/issues/10)).
- O README ficou com o que o emissor faz; o histórico de versões e defeitos está neste arquivo e nas ADRs.

## [0.5.2] - 2026-09-18

A primeira versão que emitiu uma NFS-e de verdade: emitir, enviar, consultar e cancelar, contra a Sefin Nacional em produção restrita.
Ver [ADR 0008](docs/decisoes/0008-digest-sem-namespace.md).

### Adicionado

- Testes de canonicalização ancorados numa implementação externa (libxml2), e não no próprio pacote ([ADR 0008](docs/decisoes/0008-digest-sem-namespace.md)).

### Modificado

- `nfse consultar` sobrescreve a nota já gravada, porque a consulta é idempotente; `--sobrescrever` sai dele.

### Corrigido

- O `totTrib` era escolhido pelo percentual configurado, e não pelo regime; a Sefin recusava com E0710 e E0712.
- A razão social do prestador ia na DPS quando o próprio prestador emite; a Sefin recusava com E0121.
- O tipo de inscrição federal estava invertido no identificador da DPS; a Sefin recusava com E0004.
- O digest da assinatura era calculado sem a declaração de namespace; toda DPS assinada antes desta versão é inválida (E0714) ([ADR 0008](docs/decisoes/0008-digest-sem-namespace.md)).
- `nfse consultar --dps` imprimia o identificador em branco, porque o serviço real não o devolve como o swagger diz.
- `nfse consultar` recusava consultar a mesma nota duas vezes, com uma mensagem que falava em `--numero`.

## [0.5.1] - 2026-09-18

`emitir` e `enviar` passam a recusar, antes de assinar e antes de abrir a conexão, um certificado que não é do prestador.
Atenção: se a emissão parar em E0120, esvazie `prestador.inscricao_municipal`, que é opcional.

### Adicionado

- `xmlsigner.CertificateInfo.SubjectCNPJ` e `SubjectHolderName`, que leem o titular `RAZÃO SOCIAL:CNPJ` do certificado ICP-Brasil, com dígitos verificadores conferidos.

### Modificado

- Um `403` sem envelope da Sefin cita primeiro a hipótese do certificado, e proxy e firewall em segundo lugar.

### Corrigido

- Um certificado de outro CNPJ só falhava na Sefin, com um `403` que mandava procurar proxy e firewall; o `emitir` ainda gravava uma DPS assinada que nunca seria aceita.

## [0.5.0] - 2026-09-18

Primeira versão publicada do emissor em linha de comando: emitir, enviar, consultar e cancelar, em um binário único, sem `cgo` e sem serviço externo.
Ver [ADR 0001](docs/decisoes/0001-cli-em-vez-de-api.md).

### Adicionado

- CLI `nfse`, instalável com `go install github.com/edusouza/nfse-emissor-go/cmd/nfse@latest`.
- `nfse versao`, com versão, plataforma e versão do Go.
- `nfse cert info` inspeciona um certificado A1 e avisa quando faltam menos de 30 dias para o vencimento.
- Senha do certificado por `--senha`, `NFSE_CERT_SENHA` ou prompt interativo, nessa ordem.
- `nfse config init` gera um `nfse.yaml` comentado; `nfse config check` confere se está completo e coerente.
- `nfse emitir` monta, valida e assina a DPS a partir de `padroes`, de `--yaml` e das flags; `--sem-assinar` gera o XML sem assinatura.
- `nfse emitir --enviar` transmite a DPS à Sefin e grava a NFS-e autorizada; rejeições trazem todos os motivos de uma vez ([#3](https://github.com/edusouza/nfse-emissor-go/issues/3)).
- `nfse enviar <arquivo.xml>` transmite, byte a byte, uma DPS já assinada ([#3](https://github.com/edusouza/nfse-emissor-go/issues/3)).
- `nfse consultar <chave-de-acesso>` e `nfse consultar --dps <id>`, com `--existe` ([#3](https://github.com/edusouza/nfse-emissor-go/issues/3)).
- `nfse cancelar <chave-de-acesso>`, com os motivos `erro-emissao`, `nao-prestado` e `outros`.
- Emitir e cancelar em produção pedem confirmação no terminal; `--confirmar` dispensa a pergunta em scripts.
- Numeração automática da DPS em `.nfse-estado.json`; `--numero` virou opcional ([#8](https://github.com/edusouza/nfse-emissor-go/issues/8)).
- `nfse numero ver` e `nfse numero definir` ([#8](https://github.com/edusouza/nfse-emissor-go/issues/8)).
- Validação da alíquota do ISS antes de assinar, pelas regras E0595, E0600, E0621 e E0625 ([#5](https://github.com/edusouza/nfse-emissor-go/issues/5)) ([ADR 0006](docs/decisoes/0006-validacao-de-aliquota-do-iss.md)).
- `--retencao` e `padroes.valores.retencao_issqn`, que alimentam `tpRetISSQN` ([#5](https://github.com/edusouza/nfse-emissor-go/issues/5)) ([ADR 0006](docs/decisoes/0006-validacao-de-aliquota-do-iss.md)).
- `prestador.regime_apuracao`, que alimenta `regApTribSN`.
- Validação de CNPJ e CPF na configuração e nos dados da nota.
- `nfse emitir` recusa sobrescrever um arquivo existente; `--sobrescrever` permite a substituição deliberada.
- Campos desconhecidos no YAML são recusados.
- O ambiente reportado pelo governo na resposta é exibido.
- Uma negativa por sigilo fiscal (403) é explicada, e um 403 sem corpo cita a hipótese de proxy ou firewall.
- Consultar exige só o ambiente e o certificado, não uma configuração completa de emissão.
- `SignDocument` assina qualquer documento do sistema nacional, não só a DPS.
- Exemplo executável em `exemplos/`, em shell e PowerShell, com certificado descartável.
- `docs/convenio-municipal.md`, sobre como consultar o convênio de um município (E0635 e E0640).
- Especificações OpenAPI oficiais em `docs/api/` ([#3](https://github.com/edusouza/nfse-emissor-go/issues/3)).
- Registro de decisões de arquitetura em `docs/decisoes/`.
- Teste que confere as chaves de acesso citadas na documentação.
- CI com formatação, `go mod tidy`, `go vet`, build, testes com detector de corrida e cobertura.

### Modificado

- O projeto deixou de ser uma API REST e passou a ser um CLI ([ADR 0001](docs/decisoes/0001-cli-em-vez-de-api.md)).
- Módulo Go renomeado para `github.com/edusouza/nfse-emissor-go`, e o código saiu de `src/` para a raiz.
- Dependências diretas reduzidas de 12 para 5, e a versão mínima do Go passou a 1.26.
- O validador foi renomeado para `StructuralValidator` e perdeu o parâmetro `schemaDir`, que era ignorado ([#4](https://github.com/edusouza/nfse-emissor-go/issues/4)).
- Testes dependentes de relógio são pulados com `-short`.

### Removido

- Cliente SOAP da Sefin, que seguia um contrato inventado, e seus testes ([#3](https://github.com/edusouza/nfse-emissor-go/issues/3)).
- API REST, worker assíncrono, MongoDB, Redis, webhooks, chave de API, *rate limiting*, métricas e `docker-compose.yml` ([ADR 0001](docs/decisoes/0001-cli-em-vez-de-api.md)).
- O `internal/config` de variáveis de ambiente de servidor e as entidades de persistência.
- O que não era validação da chave de acesso em `internal/domain/query`.

### Corrigido

- O cliente da Sefin falava SOAP; a API é REST/JSON, com `basePath` `/SefinNacional` ([#3](https://github.com/edusouza/nfse-emissor-go/issues/3)) ([ADR 0005](docs/decisoes/0005-contrato-da-sefin-verificado.md)).
- A conexão com a Sefin falhava com `tls: no renegotiation`, porque o governo pede o certificado numa renegociação ([#3](https://github.com/edusouza/nfse-emissor-go/issues/3)).
- A assinatura digital nunca verificava: reindentação depois de assinar, namespace perdido e verificação exigindo a chave privada ([ADR 0004](docs/decisoes/0004-assinatura-que-nao-verificava.md)).
- O XML da DPS tinha sete divergências em relação ao `DPS_v1.00.xsd` ([ADR 0003](docs/decisoes/0003-xml-conforme-o-xsd.md)).
- O validador estrutural procurava `cTribNac`, `cLocPrest` e `vServPrest` em caminhos inexistentes e exigia `subst` ([#4](https://github.com/edusouza/nfse-emissor-go/issues/4)).
- O `verAplic` estourava os 20 caracteres de `TSVerAplic` em toda DPS.
- A validação da chave de acesso exigia um prefixo `NFSe` inexistente.
- O README documentava uma chave de acesso com 47 dígitos.
- Certificados A1 no formato do OpenSSL 3 (AES-256, MAC SHA-256) não eram lidos ([ADR 0002](docs/decisoes/0002-parser-pkcs12.md)).
- A cadeia de certificados intermediários do PFX era descartada.

## [1.0.0] - 2026-01-08

Versão da API REST, anterior à mudança para CLI, mantida como registro histórico; o código está no histórico do git.

### Adicionado

- API REST para emissão de NFS-e com processamento assíncrono, chave de API, *rate limiting*, webhooks e métricas Prometheus.
- Assinatura XMLDSig (RSA-SHA256, exc-c14n) e leitura de certificados A1.
- Montagem do XML da DPS, cálculo de valores e tradução de códigos de rejeição.

[Não lançado]: https://github.com/edusouza/nfse-emissor-go/compare/v0.9.0...HEAD
[0.9.0]: https://github.com/edusouza/nfse-emissor-go/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/edusouza/nfse-emissor-go/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/edusouza/nfse-emissor-go/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/edusouza/nfse-emissor-go/compare/v0.5.2...v0.6.0
[0.5.2]: https://github.com/edusouza/nfse-emissor-go/compare/v0.5.1...v0.5.2
[0.5.1]: https://github.com/edusouza/nfse-emissor-go/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/edusouza/nfse-emissor-go/releases/tag/v0.5.0
