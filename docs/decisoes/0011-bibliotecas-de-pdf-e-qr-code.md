# 0011 — Bibliotecas de PDF e QR Code para gerar o DANFSe

**Status:** Aceita
**Data:** 2026-09-22

## Contexto

A [ADR 0010](0010-substituicao-e-danfse.md) tirou o `nfse danfse` da v0.7.0: a
API de geração do documento foi suspensa em 03/08/2026 pela
[NT 008 v1.02](../notas-tecnicas/nt-008-se-cgnfse-danfse-20260714-v1-02.pdf),
que passou a especificação para quem emite. Gerar o DANFSe aqui é a
[issue #22](https://github.com/edusouza/nfse-emissor-go/issues/22), e começa por
uma decisão que o CLAUDE.md e a [ADR 0001](0001-cli-em-vez-de-api.md) pedem para
evitar: **dependência nova num binário que lida com certificado digital**.

A NT não descreve um relatório, descreve um formulário. Em 26 páginas ela fixa:

- página **A4 retrato, uma única página**, margens entre 0,15 cm e 0,20 cm;
- **coordenadas absolutas em centímetros** para cada campo — o QR Code, por
  exemplo, fica em X 17,48 cm / Y 1,67 cm, com 1,52 cm de lado;
- linhas divisórias de **0,5 ponto** e borda de página de **1 ponto**;
- **sombreamento cinza a 5%** no cabeçalho, nos títulos de bloco e em dois
  campos;
- a expressão “NFS-e SEM VALIDADE JURÍDICA” em **vermelho sólido**, 9 pontos,
  quando `tpAmb = 2`;
- marca d'água **na diagonal**, mínimo de 50 pontos, cinza K35, para nota
  cancelada ou substituída;
- fontes **Arial** (títulos) e **Microsoft Sans Serif** (conteúdo), de 6 a 9
  pontos.

Ou seja: posicionamento absoluto, retângulos preenchidos, texto rotacionado,
cor e um código bidimensional. Nada de fluxo de texto, nada de tabelas
automáticas.

## Decisão

Duas dependências, ambas MIT e **sem `cgo`**:

| Módulo | Versão | Para quê |
|---|---|---|
| [`github.com/go-pdf/fpdf`](https://github.com/go-pdf/fpdf) | v0.9.0 | desenhar o PDF |
| [`github.com/boombuler/barcode`](https://github.com/boombuler/barcode) | v1.1.0 | **codificar** o QR Code |

**As duas foram exercitadas antes de serem escolhidas**, não lidas. Um programa
de 70 linhas desenhou, em coordenadas da própria NT, o que a especificação tem
de mais incomum: borda de 1 pt, bloco com fundo cinza 5%, rótulo de 6 pt e
conteúdo de 7 pt, a tarja vermelha de 9 pt, acentuação (`ação, ç, ã, é`), o QR
Code de 1,52 cm e a marca d'água diagonal de 50 pt em K35. O PDF saiu com 4.489
bytes e foi conferido a olho, renderizado em imagem.

**O `fpdf` trabalha em centímetros**, que é a unidade da NT. `NewCustom` com
`UnitStr: "cm"` faz cada número da tabela do item 2.4.5 ir direto para o código,
sem conversão intermediária — conversão é onde erro de leiaute se esconde.

**O QR Code é desenhado como vetor, não como imagem.** A biblioteca entra só
como *codificador*: devolve a matriz de módulos, e cada módulo vira um retângulo
preenchido no PDF. Isso evita o codificador de PNG e a incorporação de imagem,
imprime nítido em qualquer resolução, e reduz a superfície de que dependemos a
uma função — `qr.Encode` — e a um acesso de matriz.

**A fonte é a Helvetica do PDF, e isso é um desvio declarado.** Arial e
Microsoft Sans Serif são proprietárias: não podem ser redistribuídas dentro do
binário. A Helvetica é uma das 14 fontes que todo leitor de PDF tem, tem as
**mesmas métricas da Arial** (mesmas larguras de caractere, mesmo texto no mesmo
espaço) e não custa um byte de arquivo. O desvio está aqui registrado e é o
primeiro item a rever quando houver um DANFSe oficial da mesma nota para
comparar.

**Acentuação:** as fontes internas do PDF usam cp1252, e o `fpdf` traz o
tradutor (`UnicodeTranslatorFromDescriptor`). Testado com `ação, ç, ã, é`.

## Consequências

- **O binário cresce ~2,3 MB** (14,4 MB → ~16,7 MB, medido: “olá mundo” em Go
  são 2,21 MB; o mesmo com as duas bibliotecas, 4,49 MB). A maior parte é a
  tabela de métricas das fontes internas que o `fpdf` embute.
- **O grafo de dependências cresce dois módulos, não seis.** O `go.mod` do
  `fpdf` cita `gofpdi`, `golang.org/x/image`, `pdf417` e `pkg/errors`, mas
  nenhum é usado pelo pacote raiz: `go mod tidy` no projeto de teste gravou
  **só os dois** módulos, e `go list -deps` linkou quatro pacotes
  (`fpdf`, `barcode`, `barcode/qr`, `barcode/utils`).
- **O cross-compile continua de pé** — nenhuma das duas usa `cgo`.
- **O `fpdf` é um fork mantido do `jung-kurt/gofpdf`**, que está arquivado desde
  2021. É um risco real, e a mitigação é o tamanho da superfície que usamos:
  `AddPage`, `SetFont`, `Text`, `Rect`, `SetFillColor`, `SetTextColor`,
  `SetLineWidth`, `TransformRotate` e `Output`. Se o fork morrer, isso é
  substituível — inclusive por um gerador próprio, que foi considerado abaixo.
- **O `boombuler/barcode` traz 42 arquivos** de dezenas de simbologias, mas o
  Go linka só o que é importado: `barcode/qr` e seu utilitário.
- **A conferência do leiaute é visual, e precisa continuar sendo.** Teste
  automatizado prova que um campo foi escrito na coordenada certa; não prova que
  o documento está legível nem que se parece com o Anexo I. A verificação final
  é renderizar e olhar, e comparar com um DANFSe do portal para a mesma nota.

## Alternativas consideradas

**Escrever o PDF na mão, sem dependência nenhuma.** Tentador: um DANFSe usa
fontes internas, retângulos, linhas e texto — não precisa de compressão, nem de
transparência, nem de fontes incorporadas. Seriam umas 600 linhas de tabela de
referência cruzada, *content stream*, escape de texto e métricas de fonte, mais
o encaixe do QR Code. O que se ganha é uma dependência a menos; o que se paga é
um formato binário para manter e depurar dentro de um projeto cujo problema
difícil é outro — a nota fiscal. **E o QR Code continuaria sendo dependência**:
implementar Reed-Solomon e o mascaramento na mão é justamente o tipo de código
que erra em silêncio. Fica registrado como saída caso o `fpdf` seja abandonado.

**`github.com/signintech/gopdf`.** Puro Go e ativo, mas trabalha com fontes TTF
incorporadas — não usa as 14 internas. Exigiria embutir uma fonte no binário
(a Liberation Sans, licenciada em OFL, é metricamente compatível com a Arial),
uns 700 KB por peso usado, e resolveria um problema que a Helvetica já resolve
de graça. Continua sendo a resposta se a comparação com o documento oficial
mostrar que as métricas importam mais do que eu suponho.

**`github.com/johnfercher/maroto`.** É uma camada de grade *por cima* do
`fpdf`. Grade é o oposto do que a NT pede: ela dá coordenadas absolutas, campo a
campo. Seria uma dependência a mais para traduzir coordenada em grade e voltar.

**`github.com/skip2/go-qrcode`.** Zero dependências também, e API mais direta
(devolve PNG). Mas está sem tag desde sempre — a versão é um pseudo-número de
2020 — e sem commits desde então. Entre duas bibliotecas de porte parecido, a
que tem versão publicada e manutenção recente ganha.

**Continuar mandando o usuário ao portal.** É o que o README diz hoje, e
continua valendo enquanto o gerador não existir. Mas é a situação que a issue
#22 descreve como problema: o governo transferiu a geração para quem emite, e um
emissor que não emite o documento auxiliar entrega meio caminho.

## Aprendizado

A parte difícil da escolha não foi comparar READMEs — foi descobrir que o
`go.mod` de uma biblioteca **não é a conta das dependências que ela impõe**. O
`fpdf` declara cinco módulos e, pelo pacote raiz, não usa nenhum: quem paga a
conta é quem importa os subpacotes. Ler o `go.mod` teria custado quatro
dependências a mais no projeto, por engano; rodar `go mod tidy` e `go list
-deps` num programa que faz o que o nosso vai fazer respondeu em um minuto.

É a mesma regra das ADRs [0003](0003-xml-conforme-o-xsd.md),
[0006](0006-validacao-de-aliquota-do-iss.md) e
[0009](0009-lista-de-servicos-embutida.md), aplicada a dependência em vez de
especificação: confrontar a suposição com o artefato que pode desmenti-la.
