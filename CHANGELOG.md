# Changelog

Todas as mudanças notáveis neste projeto serão documentadas neste arquivo.

O formato é baseado em [Keep a Changelog](https://keepachangelog.com/pt-BR/1.0.0/),
e este projeto adere ao [Versionamento Semântico](https://semver.org/lang/pt-BR/).

## [Não lançado]

### Adicionado

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
