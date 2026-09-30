# 0016 — Site de documentação no GitHub Pages

**Status:** Aceita
**Data:** 2026-09-30

## Contexto

O README chegou a 666 linhas servindo três leitores diferentes: quem quer
emitir a própria nota, quem quer automatizar a emissão num script, e quem
integra com o Sistema Nacional e procura o que este projeto aprendeu a duras
penas. O conteúdo mais raro — as ADRs, o que cada rejeição significa — estava
lá, mas não era encontrável por quem não abrisse o repositório.

O GitHub Pages já estava habilitado no repositório, no modo "via workflow", sem
nenhum workflow que publicasse nada.

Ao planejar a página de rejeições, apareceu um problema maior que o site. O
catálogo em `internal/domain/emission/errors.go` tem 38 códigos de três
dígitos (`E001` "CNPJ nao encontrado", `E042`...) com descrição e ação
sugerida em inglês. **Nenhum deles existe.** O ANEXO I oficial
(`docs/anexos/anexo_i-sefin_adn-dps_nfse-snnfse-v1-01-20260209.xlsx`) usa
quatro dígitos, de `E0001` a `E1638`. O catálogo é da época da API REST
([ADR 0001](0001-cli-em-vez-de-api.md)) e nenhum código do CLI o usa. O
próprio README mostrava uma rejeição inventada (`[E001] Municipio nao
conveniado`), que contradizia o catálogo.

## Decisão

**Um site estático em pt-BR, com a fonte em `site/`,** publicado pelo workflow
`pages.yml` a cada mudança no `master`. Nos pull requests ele só é construído,
em modo estrito: um link quebrado reprova a mudança.

**O gerador é o Zensical, lendo um `mkdocs.yml`.** A primeira escolha era o
Material for MkDocs, mas ele entra em modo de manutenção em 05/11/2026: a 9.7
é a última versão com recursos novos, e depois só vêm correções de segurança
por um ano. O Zensical é o sucessor, da mesma equipe, e lê o mesmo arquivo de
configuração, mas está na versão 0.0.x. Daí a regra: **o `mkdocs.yml` só usa o
que o Material também entende.** Se o Zensical quebrar, voltar ao Material é
trocar uma linha do `site/requirements.txt`. O Hugo manteria as ferramentas só
em Go, mas pediria tema e configuração próprios, sem essa saída. O Python que o
Zensical traz roda só na CI e na máquina de quem mexe no site; nada dele entra
no binário.

**O que vem do repositório é gerado, não copiado.** `internal/sitegen` escreve
três tipos de página antes de cada construção: os códigos de rejeição, as ADRs e
o changelog. Os links relativos são reescritos: o que é página do site vira
link entre páginas, o resto aponta para o GitHub. Nada disso é commitado. A
`lista.csv` de serviços ([ADR 0009](0009-lista-de-servicos-embutida.md)) é
commitada porque vai embutida no binário; estas páginas não precisam estar no
repositório, e uma cópia que não existe não tem como ficar desatualizada.

**Os códigos de rejeição vêm do ANEXO I, não de `errors.go`.** A página traz o
texto do governo: a mensagem, a regra, o campo do XML e o nível da regra
(1, leiaute; 2, regra geral; 3, depende do município). Entram as regras da
recepção (o certificado de transmissão) e as que o anexo marca como executadas
na recepção de uma DPS pela Sefin (coluna K = `V`), 426 códigos. Ficam de fora
as que só valem para notas que os municípios compartilham com o ADN, que um
emissor nunca recebe. O leitor da planilha segue o de
`internal/domain/servico/gerar_lista.go`: `archive/zip` e `encoding/xml`, sem
biblioteca de planilha. Ele confere os cabeçalhos e se recusa a gerar se o
leiaute do anexo mudar, e espalha as células mescladas, que é como o anexo
atribui um caminho de XML a várias regras.

**As regras que o `nfse` confere antes de assinar são marcadas a partir do
código.** O gerador procura `a Sefin rejeita: E0000` nas mensagens de
`internal/domain/validation`. Uma validação nova que cite o código aparece na
página sem ninguém lembrar. Uma mensagem que cite um código ausente do anexo
impede a geração.

**O que não se publica:**

- `docs/markdown/` e `docs/nfse-nacional/`, os manuais do governo. São cerca de
  40 MB, e uma cópia republicada envelhece. O site aponta para a fonte.
- `specs/`, que descreve a API REST abandonada.
- O catálogo de `errors.go`.

## Consequências

- **Rejeições inventadas não chegam ao site.** O exemplo de rejeição do guia de
  envio é o `E0714` real, registrado na [ADR 0008](0008-digest-sem-namespace.md).
- **As páginas escritas à mão passam pelos testes do README.** Os testes de
  `internal/docs`, que conferem chaves de acesso e códigos de serviço citados
  na documentação, agora leem também `site/conteudo/`. As páginas geradas são
  reconhecidas pelo comentário que o `sitegen` põe no topo e ficam de fora.
- **O site e o README repetem conteúdo por enquanto.** O README deve encolher
  para a apresentação, a instalação e o caminho para o site, mas só depois da
  primeira publicação confirmada: antes disso, o README é a única cópia que
  certamente está no ar.
- **O endereço depende do site de usuário.** O site sai em
  `edusouza.github.io/nfse-emissor-go/`, o endereço padrão de um site de
  projeto. Isso só funciona enquanto o repositório `edusouza.github.io` não
  tiver domínio personalizado: se tiver, o GitHub redireciona todos os sites
  de projeto para esse domínio. Quando esta decisão foi tomada, ele apontava
  para `blog.eduardosouza.net`, um nome que não existia no DNS, e o domínio
  precisou ser removido de lá. Um domínio exclusivo para o `nfse` continua
  possível, com um arquivo `CNAME` e o `site_url` do `mkdocs.yml`.
- **O catálogo de `errors.go` continua no código,** morto e errado. Removê-lo é
  a [issue #37](https://github.com/edusouza/nfse-emissor-go/issues/37).
- **O Zensical é 0.0.x.** A versão fica fixada em `site/requirements.txt`, e
  atualizá-la é uma decisão, não um efeito colateral.
