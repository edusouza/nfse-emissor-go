# 0020 — Binários publicados com proveniência assinada

**Status:** Aceita
**Data:** 2026-10-02

## Contexto

Até aqui, o `nfse` era distribuído só pelo código-fonte:
`go install github.com/edusouza/nfse-emissor-go/cmd/nfse@vX.Y.Z`. A integridade
vinha do checksum database do Go (`sum.golang.org`): quem instala recebe
exatamente o código da tag, verificado.

Isso exige o Go instalado, o que afasta justamente o público do projeto —
prestadores do Simples Nacional que querem emitir a própria nota, não
programar. O passo seguinte natural é publicar binários prontos.

Um binário pronto tem um problema que o `go install` não tem: **ninguém sabe
de onde ele veio.** E este binário recebe o certificado A1 e a senha — a
chave que assina em nome da empresa. Um binário adulterado na release, ou
construído na máquina de alguém a partir de um código que não é o da tag,
seria o ataque mais barato contra todos os usuários de uma vez. A
[issue #48](https://github.com/edusouza/nfse-emissor-go/issues/48) registrou
isso como lacuna no [SECURITY.md](../../SECURITY.md).

## Decisão

**Os binários são construídos só pela CI, a partir de uma tag, e publicados
com proveniência assinada.** O workflow `.github/workflows/release.yml` roda
quando uma tag `vX.Y.Z` é enviada e:

1. confere que a tag segue o formato de versão e que o commit está no `master`;
2. roda os testes;
3. constrói para Linux, macOS e Windows, em `amd64` e `arm64`, com
   `CGO_ENABLED=0` e `-trimpath`, gravando a versão no binário;
4. gera o `SHA256SUMS` e um SBOM CycloneDX do módulo;
5. gera a **proveniência SLSA** e a atestação do SBOM de cada binário, com
   `actions/attest-build-provenance` e `actions/attest-sbom`;
6. publica tudo na release da tag.

**A assinatura é do Sigstore, sem chave.** O workflow se identifica por OIDC,
o Sigstore emite um certificado de curta duração para aquela execução e
registra a assinatura no log público de transparência. Não existe chave
privada de assinatura para guardar, rotacionar ou vazar — e a assinatura diz
*qual workflow, de qual repositório, em qual commit* construiu o binário.

Conferir é um comando:

```bash
gh attestation verify nfse_v1.0.0_linux_amd64 --repo edusouza/nfse-emissor-go
```

**`go install` continua sendo o caminho principal.** Os binários são para
quem não tem o Go; quem tem continua com a verificação do checksum database.

## Alternativas consideradas

- **Assinar com uma chave GPG ou cosign do mantenedor.** Exige guardar uma
  chave privada de longa duração, que vira o novo alvo, e não prova onde o
  binário foi construído — só quem o assinou.
- **GoReleaser.** Faz tudo isto e mais, mas é uma dependência de terceiros no
  caminho da publicação, com configuração própria. O que precisamos cabe em
  um laço de `go build` e duas ações oficiais do GitHub.
- **Não publicar binários.** Mantém a superfície menor, mas deixa de fora
  quem mais precisa do projeto. A lacuna era a ausência de verificação, não a
  existência de binários.

## Consequências

- **O workflow tem permissão de escrita** (`contents: write`) para criar a
  release, além de `id-token: write` e `attestations: write`. Elas ficam no
  job, não no workflow, e só esse workflow as tem. O job roda no environment
  `release`, onde o mantenedor pode exigir aprovação antes de publicar.
- **Quem envia uma tag publica uma versão.** Proteger as tags `v*` (Settings →
  Rules) impede que alguém sem permissão de manutenção crie uma.
- **A conferência depende do usuário.** A proveniência só protege quem a
  confere. O README e as notas de cada release mostram o comando.
- O SBOM é gerado pelo `cyclonedx-gomod`, chamado por `go run` em versão fixa
  e verificado pelo checksum database — não por uma ação de terceiros.
