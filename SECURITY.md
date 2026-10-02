# Política de segurança

O `nfse` assina documentos fiscais com o certificado digital da empresa e os
entrega à Receita. Uma falha aqui pode ter consequências de verdade: a chave
privada de um e-CNPJ assina em nome da empresa, e uma NFS-e emitida por engano
tem valor fiscal e precisa ser cancelada. Por isso, relatos de segurança passam
na frente de qualquer outro trabalho no projeto.

Este documento explica como relatar uma vulnerabilidade, o que esperar depois
do relato, o que o projeto protege e de quem, e as regras que o código segue
para que os problemas conhecidos não aconteçam.

## Sumário

- [Versões suportadas](#versões-suportadas)
- [Como relatar uma vulnerabilidade](#como-relatar-uma-vulnerabilidade)
- [O que acontece depois do relato](#o-que-acontece-depois-do-relato)
- [Escopo](#escopo)
- [Porto seguro para pesquisa](#porto-seguro-para-pesquisa)
- [Modelo de ameaças](#modelo-de-ameaças)
- [Como o projeto evita cada problema](#como-o-projeto-evita-cada-problema)
- [Regras para quem contribui](#regras-para-quem-contribui)
- [Recomendações para quem usa](#recomendações-para-quem-usa)
- [O que ainda não temos](#o-que-ainda-não-temos)
- [Histórico de avisos](#histórico-de-avisos)

## Versões suportadas

O projeto está em `0.x` (veja a nota no [CHANGELOG](CHANGELOG.md)). Até a
`1.0.0`, **só a versão mais recente recebe correções de segurança**: a correção
sai numa nova versão de patch ou minor, nunca num backport.

| Versão                    | Recebe correções de segurança |
|---------------------------|-------------------------------|
| `0.9.x` (a mais recente)  | Sim                           |
| `< 0.9`                   | Não — atualize                |
| `1.0.0` do CHANGELOG (API REST antiga, nunca publicada) | Não |

Depois da `1.0.0`, a última minor de cada major ainda suportada recebe
correções. A tabela é atualizada a cada versão.

Para atualizar:

```bash
go install github.com/edusouza/nfse-emissor-go/cmd/nfse@latest
nfse versao
```

## Como relatar uma vulnerabilidade

> **Não abra uma issue, pull request ou discussão pública** para relatar uma
> vulnerabilidade. Um relato público dá a quem quer explorar a falha o mesmo
> tempo que dá a quem precisa corrigi-la.

Use o **relato privado de vulnerabilidades do GitHub**:

1. Acesse a aba [**Security**](https://github.com/edusouza/nfse-emissor-go/security)
   do repositório.
2. Clique em **Report a vulnerability**
   ([link direto](https://github.com/edusouza/nfse-emissor-go/security/advisories/new)).
3. Preencha o formulário. Só você e os mantenedores veem o conteúdo.

O relato privado gera um *security advisory* rascunho, em que a correção pode
ser discutida, testada num fork privado e, ao fim, publicada com crédito a quem
encontrou — inclusive com pedido de CVE.

Se o formulário não estiver disponível, abra uma issue pública **sem nenhum
detalhe técnico**, só pedindo um canal privado de contato. Um mantenedor
responde por lá.

### O que incluir

Quanto mais concreto o relato, mais rápida a correção:

- a versão afetada (`nfse versao`), o sistema operacional e a versão do Go;
- o componente: comando (`emitir`, `enviar`, `cancelar`, `danfse`...) ou
  pacote (`internal/infrastructure/xmlsigner`, `.../sefin`...);
- o tipo de falha e o impacto que você enxerga — vazamento da chave, emissão
  indevida, execução de código, negação de serviço;
- os passos para reproduzir, de preferência com uma prova de conceito mínima;
- se você já sabe, uma sugestão de correção.

### O que **não** incluir

- **Certificado real, senha real ou chave privada** — nem sua, nem de ninguém.
  Gere um certificado de teste: os testes do projeto mostram como fazê-lo em
  memória (`internal/infrastructure/xmlsigner/certificate_format_test.go`).
- **Dados pessoais de terceiros**: CPF, nome ou endereço de tomadores, XMLs de
  NFS-e reais. Anonimize antes. Isso é exigência da LGPD, não preferência.
- Chaves de acesso de notas reais de outras pessoas.

## O que acontece depois do relato

O projeto é mantido por poucas pessoas, em tempo parcial. Os prazos abaixo são
compromissos de melhor esforço, contados em dias corridos a partir do relato:

| Etapa | Prazo |
|---|---|
| Confirmação de recebimento | até **3 dias** |
| Avaliação inicial: reprodução, severidade e escopo | até **7 dias** |
| Correção publicada — severidade **crítica** | até **7 dias** após a confirmação |
| Correção publicada — severidade **alta** | até **30 dias** |
| Correção publicada — severidade **média** | até **90 dias** |
| Correção publicada — severidade **baixa** | na próxima versão planejada |

A severidade é calculada com o [CVSS v4.0](https://www.first.org/cvss/v4.0/),
ajustada pelo contexto do projeto: qualquer falha que exponha a chave privada
ou a senha do certificado, ou que faça o `nfse` assinar ou transmitir algo que
o usuário não pediu, é tratada como **crítica**, seja qual for a nota.

Durante o processo você recebe notícias pelo menos a cada 14 dias, mesmo que a
notícia seja "ainda estamos nisso".

### Divulgação coordenada

- A vulnerabilidade só é divulgada depois que a correção estiver publicada, ou
  em **90 dias** a contar do relato, o que vier primeiro. O prazo pode ser
  estendido em comum acordo se a correção depender de terceiros — por exemplo,
  de uma biblioteca ou do próprio Sistema Nacional NFS-e.
- Se a falha estiver sendo explorada ativamente, a divulgação pode ser
  antecipada, para que os usuários possam se proteger.
- A divulgação é feita por um
  [GitHub Security Advisory](https://github.com/edusouza/nfse-emissor-go/security/advisories)
  e por uma entrada na seção **Segurança** do [CHANGELOG](CHANGELOG.md), que
  diz qual versão corrige e o que o usuário precisa fazer.
- Quem relatou recebe crédito no advisory, a menos que prefira o anonimato.

### Falhas nas dependências ou no governo

- **Biblioteca de terceiros:** se a falha está numa dependência (`etree`,
  `go-pkcs12`, `cobra`, `fpdf`...), relate também ao projeto dela. Aqui,
  avaliamos se o `nfse` é afetado — se o código vulnerável é de fato alcançado
  — e atualizamos ou contornamos.
- **Sistema Nacional NFS-e, BrasilAPI ou IBGE:** falhas nesses serviços estão
  fora do escopo deste projeto. Relate ao responsável pelo serviço. Se a falha
  afetar a forma como o `nfse` os usa, avise aqui também.

## Escopo

**Dentro do escopo:**

- o binário `nfse` e todo o código de `cmd/`, `internal/` e `pkg/`;
- a leitura do certificado A1, a assinatura XMLDSig e a canonicalização;
- a comunicação com a Sefin Nacional, a BrasilAPI e o IBGE, do lado do cliente;
- o tratamento de arquivos de entrada (`nfse.yaml`, `--yaml`, XMLs) e de saída;
- o DANFSe gerado em PDF;
- os workflows de `.github/workflows/` e o gerador do site (`internal/sitegen`).

**Fora do escopo:**

- vulnerabilidades nos serviços do governo, da BrasilAPI ou do IBGE em si;
- ataques que exigem uma máquina já comprometida — quem tem acesso de leitura à
  sua memória ou como seu usuário já pode ler o `.pfx` e capturar a senha
  diretamente;
- o uso de `--senha` na linha de comando ficar visível na lista de processos:
  é um risco conhecido e documentado, e o CLI orienta a usar
  `NFSE_CERT_SENHA` ou o prompt;
- relatórios de ferramentas automáticas sem demonstração de impacto real;
- engenharia social contra mantenedores ou usuários;
- negação de serviço contra os serviços públicos.

## Porto seguro para pesquisa

Pesquisa feita de boa-fé, dentro desta política, é bem-vinda. Não tomaremos
nenhuma medida contra quem:

- testar **só contra a própria instalação** e com **o próprio certificado**,
  ou com certificados de teste gerados para isso;
- usar **apenas o ambiente `producao-restrita`** para qualquer teste que
  envolva a Sefin — o ambiente de produção emite notas com valor fiscal;
- não acessar, alterar ou guardar dados de outras pessoas além do mínimo
  necessário para demonstrar a falha;
- não fizer testes de carga ou de negação de serviço contra os serviços
  públicos;
- der tempo razoável para a correção antes de divulgar, como descrito acima.

Esta política não autoriza nenhum teste contra a infraestrutura do governo.
O Sistema Nacional NFS-e tem regras próprias; respeite-as.

## Modelo de ameaças

### O que protegemos

| Ativo | Por que importa |
|---|---|
| **Chave privada do certificado A1** | Assina em nome da empresa: documentos fiscais e qualquer outro documento eletrônico com validade jurídica (MP 2.200-2/2001). Vazou, alguém se passa pela empresa até o certificado ser revogado. |
| **Senha do certificado** | Junto com o `.pfx`, é a chave privada. |
| **Integridade da DPS** | O que é assinado e transmitido vira NFS-e com valor fiscal. Valor, alíquota ou tomador trocados geram imposto errado e uma nota que precisa ser cancelada. |
| **Unicidade da emissão** | Uma segunda transmissão da mesma intenção pode gerar uma segunda nota. |
| **Dados pessoais** | CPF, nome e endereço de tomadores nos XMLs e PDFs — dados protegidos pela LGPD. |
| **Integridade do binário** | Um `nfse` adulterado tem acesso a tudo o que está acima. |

### De quem protegemos

| Adversário | Exemplo do que tenta |
|---|---|
| Rede hostil | Interceptar ou alterar o tráfego com a Sefin (Wi-Fi público, proxy malicioso). |
| Serviço remoto com defeito ou comprometido | Devolver uma resposta enorme, malformada ou feita para explorar o leitor de XML/JSON. |
| Arquivo de entrada malicioso | Um XML ou YAML recebido de terceiros, feito para travar ou enganar o `nfse`. |
| Cadeia de suprimentos | Uma dependência ou ação do GitHub adulterada que roube a chave no build ou em tempo de execução. |
| Outro usuário da mesma máquina | Ler a senha na lista de processos ou os XMLs no diretório de saída. |
| O próprio usuário, por engano | Commitar o `.pfx`, emitir em produção achando que é teste, reenviar uma nota já emitida. |

### Fronteiras de confiança

```
 usuário ──(flags, nfse.yaml, --yaml)──▶ nfse ──(mTLS)──▶ Sefin Nacional
                                          │  ├──(HTTPS)──▶ BrasilAPI (só no onboard)
   .pfx + senha ─────────────────────────▶│  └──(HTTPS)──▶ IBGE (municípios, com cache)
                                          ▼
                         diretório de saída (XML, PDF, estado)
```

Tudo o que atravessa uma seta é tratado como **não confiável** até ser
validado — inclusive as respostas do governo.

## Como o projeto evita cada problema

Cada item abaixo diz o risco e o que o código faz a respeito. As referências
apontam para o código, para que você possa conferir.

### Chave privada e certificado

| Risco | Como evitamos |
|---|---|
| Chave ou senha em log, erro ou saída do terminal | Nenhum caminho do código registra a senha, a chave ou o conteúdo do PFX. As mensagens de erro descrevem o problema sem repetir o segredo. Regra explícita no [CLAUDE.md](CLAUDE.md) e verificada em revisão. |
| Chave gravada em disco | A chave só existe em memória, durante a execução. O `nfse` nunca a exporta, converte ou guarda em cache. |
| Senha em arquivo de configuração | O `nfse.yaml` **não tem** campo de senha, de propósito (`internal/config/config.go`, tipo `Certificado`). |
| `.pfx` commitado por engano | O `.gitignore` bloqueia `*.pfx`, `*.p12`, `*.pem`, `*.key`, `*.crt`, `.env` e `credentials.json`. Os testes geram certificados em memória — não há certificado nenhum no repositório. |
| Parser de PKCS#12 incompleto ou abandonado | `software.sslmate.com/src/go-pkcs12`, mantido e com suporte a AES-256 + SHA-256, no lugar do congelado `golang.org/x/crypto/pkcs12` ([ADR 0002](docs/decisoes/0002-parser-pkcs12.md)). |
| Assinar com certificado vencido ou impróprio | Antes de assinar, o certificado é validado: validade, `KeyUsage` de assinatura digital e `ExtKeyUsage` (`internal/infrastructure/xmlsigner/validate.go`). O `nfse cert info` mostra o diagnóstico e avisa 30 dias antes do vencimento. |
| CNPJ errado lido do certificado | O CNPJ vem do `otherName` ICP-Brasil (OID `2.16.76.1.3.3`), o mesmo campo que a Sefin lê. Um certificado ambíguo — duas entradas, entrada inválida — é recusado em vez de cair para o nome comum. Os OIDs vizinhos, que trazem CPF e data de nascimento do responsável, **não são lidos** (`internal/infrastructure/xmlsigner/icpbrasil.go`). |

### Senha do certificado

| Risco | Como evitamos |
|---|---|
| Senha visível na lista de processos (`ps`, `/proc`) | A ordem de precedência é `--senha`, `NFSE_CERT_SENHA`, prompt. A ajuda do comando e o README orientam a não usar `--senha`. |
| Senha ecoada no terminal | O prompt usa `term.ReadPassword`, sem eco (`internal/cli/password.go`). |
| Senha pedida sem terminal, travando um script | Sem terminal e sem as outras fontes, o comando falha com uma mensagem que diz como informá-la, em vez de esperar para sempre. |

### Comunicação com a Sefin Nacional

| Risco | Como evitamos |
|---|---|
| Interceptação ou adulteração (MITM) | TLS 1.2 ou superior, com o certificado do servidor verificado contra as CAs do sistema. **`InsecureSkipVerify` nunca é usado** no código de produção (`internal/infrastructure/sefin/client.go`). |
| Renegociação TLS insegura (CVE-2009-3555) | A Sefin pede o certificado do cliente por renegociação; o `nfse` a aceita, mas o Go só renegocia com servidores que anunciam a renegociação segura da RFC 5746. Em TLS 1.3 a renegociação não existe. A justificativa está no próprio código. |
| Identidade do emissor | Por TLS mútuo com o mesmo A1 que assina a DPS. Não há token, chave de API ou senha de portal para vazar. |
| **Emissão duplicada** | A emissão e o registro de eventos **nunca são repetidos automaticamente**: uma requisição que falhou depois de enviada pode ter sido processada. Só consultas, que são idempotentes, são repetidas, e só quando a conexão nem chegou a abrir. A numeração é controlada localmente, e um arquivo de DPS existente nunca é sobrescrito sem `--sobrescrever` (`O_EXCL`). |
| Emitir em produção por engano | O ambiente padrão é `producao-restrita`, em todo lugar: config ausente, campo vazio ou cliente sem ambiente nunca caem em produção. |
| Conexão que nunca responde | Tempo limite por requisição (60 s) e para abrir a conexão (10 s). |

### Dados que vêm de fora (rede e arquivos)

| Risco | Como evitamos |
|---|---|
| Resposta gigante esgotando a memória | Toda leitura de resposta HTTP tem limite (`io.LimitReader`): 8 MiB na Sefin, limites próprios na BrasilAPI e no IBGE. |
| Bomba de descompressão (gzip) | O XML descompactado da Sefin é limitado a 32 MiB; passar disso é erro, não truncamento silencioso (`internal/infrastructure/sefin/payload.go`). |
| XXE e expansão de entidades ("billion laughs") | O XML é lido com `encoding/xml` e `etree`, que não resolvem entidades externas nem expandem entidades declaradas em DTD. Nenhum XML dispara acesso a arquivo ou rede. |
| XML de entrada enorme no `danfse` | Limite de 5 MiB na leitura (`internal/cli/danfse.go`). |
| DPS malformada chegando a ser assinada | A DPS é validada contra os **XSDs oficiais v1.01, embutidos no binário**, e contra as regras de negócio, **antes** de o certificado ser usado ([ADR 0014](docs/decisoes/0014-validacao-pelo-xsd.md)). O validador recusa construções de XSD que não conhece em vez de ignorá-las. |
| Assinatura que não verifica ou que cobre outra coisa | Canonicalização exc-c14n testada contra casos reais ([ADR 0004](docs/decisoes/0004-assinatura-que-nao-verificava.md), [ADR 0008](docs/decisoes/0008-digest-sem-namespace.md)), RSA-SHA256, e testes de ida e volta que assinam e verificam. |
| Injeção no `nfse.yaml` gerado pelo `onboard` | Os dados da BrasilAPI (razão social, por exemplo) vão para o YAML como strings entre aspas duplas, com escape, e caracteres não imprimíveis são tratados (`internal/config/onboard.go`). |
| Arquivo de entrada malicioso derrubando o processo | `gopkg.in/yaml.v3` na versão `3.0.1`, que já inclui a correção do CVE-2022-28948 (pânico com YAML malformado). Um `panic` provocado por entrada é tratado como vulnerabilidade. |

### Linguagem e código

| Prática | Por quê |
|---|---|
| **Sem `cgo`** | Go puro mantém as garantias de segurança de memória do Go e evita a superfície de bibliotecas C. Também preserva o cross-compile. |
| **Sem `unsafe`**, sem `os/exec`, sem `math/rand` para nada sensível | Nenhum código que entra no binário importa `unsafe` ou executa programas externos (só um teste de gerador usa `os/exec`). A aleatoriedade criptográfica vem de `crypto/rand`. |
| Criptografia só da biblioteca padrão | `crypto/rsa`, `crypto/sha256`, `crypto/tls`, `crypto/x509`. Nenhum algoritmo implementado à mão. |
| `go test -race` na CI | Detecta condições de corrida. |
| `go vet` e `gofmt` na CI | Pegam erros comuns e mantêm o código revisável. |
| `go mod tidy` conferido na CI | Um `go.mod` ou `go.sum` alterado sem explicação faz a CI falhar. |
| Erros sempre com contexto (`%w`) | O usuário recebe uma mensagem que diz o que fazer, e o erro original não se perde. |
| Toolchain do Go atualizada | O projeto exige Go 1.26+, uma versão suportada pelo time do Go. Versões de correção do Go (`1.26.x`) devem ser adotadas logo que saem, porque trazem correções em `crypto/tls`, `net/http` e `encoding/xml`. |

### Cadeia de suprimentos

| Risco | Como evitamos |
|---|---|
| Dependências demais | Sete dependências diretas, cada uma justificada — em ADR quando é nova num binário que lida com certificado ([ADR 0002](docs/decisoes/0002-parser-pkcs12.md), [ADR 0011](docs/decisoes/0011-bibliotecas-de-pdf-e-qr-code.md)). Dependência nova exige justificativa no PR. |
| Módulo adulterado | O `go.sum` fixa o hash de cada módulo, e o Go confere contra o [checksum database](https://sum.golang.org) público. Quem instala com `go install ...@versão` recebe exatamente o código publicado. |
| Dados de referência adulterados | XSDs e a lista de serviços são embutidos no binário a partir dos arquivos oficiais versionados em `docs/`; as tabelas de rejeição são geradas dos anexos oficiais com `go generate`, nunca escritas à mão. |
| Workflow do GitHub com permissão demais | Os workflows declaram permissões mínimas (`contents: read`) e só elevam no job que precisa: `pages: write` no deploy do site, `id-token: write` no workflow do assistente de código. Nenhum usa `pull_request_target`, então código de um fork nunca roda com segredos. |
| Dados pessoais em log público de CI | O teste de contrato roda na CI **sem** o CNPJ de um MEI real, porque a razão social de um MEI traz nome e CPF de uma pessoa (`.github/workflows/contrato.yml`). |

### Privacidade

- **Sem telemetria.** O `nfse` não coleta nem envia dado de uso a ninguém.
- Os únicos destinos de rede são a Sefin Nacional (emissão, consulta,
  cancelamento), a BrasilAPI (só no `onboard`, e só com o CNPJ do prestador) e
  o IBGE (lista pública de municípios, sem dado do usuário).
- O cache do IBGE fica no diretório de cache do usuário e só contém dados
  públicos. Ele é gravado de forma atômica (arquivo temporário + `rename`).

## Regras para quem contribui

Todo pull request é revisado também sob este ponto de vista. Antes de abrir o
seu, confira:

- [ ] **Nenhum segredo no código, nos testes ou nos exemplos.** Certificados de
      teste são gerados em memória. Nunca use `git add -f` para forçar um
      `.pfx`, `.p12`, `.pem` ou `.key`.
- [ ] **Nenhum log, erro ou `fmt.Print` com a senha, a chave privada, o
      conteúdo do PFX** ou dados pessoais completos de tomadores.
- [ ] **Toda leitura de dado externo tem limite de tamanho** (`io.LimitReader`)
      e toda requisição tem tempo limite.
- [ ] **Nada de `InsecureSkipVerify`**, de baixar a versão mínima do TLS ou de
      aceitar algoritmos fracos (SHA-1, RSA < 2048) para "fazer funcionar".
- [ ] **Operações não idempotentes não são repetidas automaticamente.**
      Emissão e eventos falham alto; quem decide o que fazer é o usuário.
- [ ] **O padrão seguro continua sendo o padrão.** Ambiente restrito, não
      sobrescrever arquivos, validar antes de assinar.
- [ ] **Sem `cgo`, sem `unsafe`, sem `os/exec` no código do binário.** Se for
      mesmo necessário, é decisão para um ADR.
- [ ] **Dependência nova só com justificativa** no PR — e um ADR, se ela entra
      no caminho do certificado, da assinatura ou da rede. Prefira a biblioteca
      padrão.
- [ ] **`go test -race ./...`, `go vet ./...` e `gofmt -l`** passam localmente.
- [ ] Mudanças em workflows mantêm as permissões mínimas e não introduzem
      `pull_request_target` nem interpolação de dados do evento
      (`${{ github.event... }}`) dentro de `run:`.
- [ ] Mudanças com impacto de segurança entram no CHANGELOG, na seção
      **Segurança**.

Se o seu PR corrige uma vulnerabilidade **ainda não divulgada**, não o abra no
repositório público: use o fork privado do advisory, como descrito em
[Como relatar](#como-relatar-uma-vulnerabilidade).

## Recomendações para quem usa

O `nfse` faz a parte dele; o resto depende de como ele é usado.

**Proteja o certificado.**

```bash
chmod 600 empresa.pfx          # só você lê o arquivo
```

Guarde o `.pfx` fora de pastas sincronizadas ou compartilhadas e fora de
repositórios git. Faça um backup cifrado.

**Informe a senha sem deixá-la no histórico.** `export NFSE_CERT_SENHA=...`
digitado no terminal fica no histórico do shell. Prefira o prompt interativo,
ou leia a senha sem eco:

```bash
read -rs NFSE_CERT_SENHA && export NFSE_CERT_SENHA
```

Em automações (CI, cron), use o cofre de segredos da plataforma — nunca a
senha em texto no script.

**Proteja o diretório de saída.** Os XMLs e PDFs trazem dados pessoais de
tomadores. Hoje eles são gravados com permissão `0644` (leitura para todos os
usuários da máquina; a correção é a [#46](https://github.com/edusouza/nfse-emissor-go/issues/46)).
Numa máquina compartilhada, restrinja o diretório:

```bash
chmod 700 notas/
```

**Teste em `producao-restrita`.** Só mude `ambiente` para `producao` quando o
fluxo estiver conferido.

**Instale versões fixas e confira o que está instalado.**

```bash
go install github.com/edusouza/nfse-emissor-go/cmd/nfse@v0.9.0
nfse versao
go version -m "$(command -v nfse)"   # mostra o módulo e as dependências embutidas
```

Não desligue a verificação de checksums do Go para este módulo: nada de
`GOSUMDB=off`, nem `GONOSUMDB`, `GOPRIVATE` ou `GOINSECURE` cobrindo
`github.com/edusouza/*`.

**Se o certificado vazou** — o `.pfx` e a senha, ou a máquina onde estavam foi
comprometida:

1. **Peça a revogação do certificado à Autoridade Certificadora** que o emitiu,
   imediatamente. Enquanto não for revogado, quem tem o arquivo e a senha
   assina em nome da empresa.
2. Confira no [Portal Nacional da NFS-e](https://www.nfse.gov.br) se há notas
   emitidas que você não reconhece, e cancele-as.
3. Emita um certificado novo e troque a senha.
4. Se a suspeita envolver o `nfse`, relate conforme
   [Como relatar](#como-relatar-uma-vulnerabilidade).

**Sobre o `nfse danfse`:** o PDF é gerado a partir do XML que você fornece, e a
assinatura desse XML não é conferida. O DANFSe não prova que a nota existe —
quem prova é a consulta pela chave de acesso no portal ou com `nfse consultar`.
Não gere DANFSe a partir de XMLs recebidos de terceiros esperando que isso os
autentique.

## O que ainda não temos

Transparência sobre o que falta faz parte da política. Cada lacuna abaixo tem
uma issue, e **vamos corrigir todas**, nesta ordem de prioridade. Quando uma
issue fecha, o item sai desta lista e entra na tabela de controles
correspondente acima.

| # | Lacuna | Risco enquanto não for corrigida | Issue |
|---|---|---|---|
| 1 | **`govulncheck` na CI** | Uma vulnerabilidade conhecida no Go ou numa dependência, em código que o `nfse` de fato chama, passa sem aviso. | [#43](https://github.com/edusouza/nfse-emissor-go/issues/43) |
| 2 | **Atualização automática de dependências** (Dependabot), para módulos Go e ações do GitHub | Uma correção de segurança publicada numa dependência demora a chegar aqui. | [#44](https://github.com/edusouza/nfse-emissor-go/issues/44) |
| 3 | **Ações do GitHub fixadas por hash de commit**, não por tag (`@v4`) | Uma tag movida num repositório comprometido muda o que roda na CI, com acesso aos segredos. | [#45](https://github.com/edusouza/nfse-emissor-go/issues/45) |
| 4 | **Arquivos de saída com permissão `0600`** | XMLs e PDFs com dados pessoais de tomadores ficam legíveis para os outros usuários da máquina. | [#46](https://github.com/edusouza/nfse-emissor-go/issues/46) |
| 5 | **Análise estática de segurança** (CodeQL ou `gosec`) e o [OpenSSF Scorecard](https://securityscorecards.dev) | Padrões inseguros dependem só da revisão humana para serem pegos. | [#47](https://github.com/edusouza/nfse-emissor-go/issues/47) |
| 6 | **Binários pré-compilados assinados**, com proveniência [SLSA](https://slsa.dev) | Só passa a valer se o projeto distribuir binários. Hoje a distribuição é só pelo código-fonte (`go install`), cuja integridade é garantida pelo checksum database do Go. | [#48](https://github.com/edusouza/nfse-emissor-go/issues/48) |

## Histórico de avisos

Nenhuma vulnerabilidade foi divulgada até agora. Os avisos publicados ficam em
[Security Advisories](https://github.com/edusouza/nfse-emissor-go/security/advisories)
e na seção **Segurança** de cada versão no [CHANGELOG](CHANGELOG.md).

---

Obrigado por ajudar a manter o `nfse` e seus usuários seguros.
