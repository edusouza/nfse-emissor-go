# Comandos

| Comando | O que faz | Guia |
|---------|-----------|------|
| `nfse onboard` | cria o `nfse.yaml` já preenchido, a partir do certificado | [Configurar](../comecar/configurar.md) |
| `nfse config init` / `check` | cria um `nfse.yaml` em branco e confere o que está preenchido | [Configurar](../comecar/configurar.md) |
| `nfse servico buscar` / `ver` / `listar` | acha o código do serviço (`cTribNac`) na lista nacional | [Achar o código do serviço](../guias/codigo-do-servico.md) |
| `nfse cert info` | inspeciona o certificado A1 | [O certificado e a senha](../guias/certificado.md) |
| `nfse emitir` | monta, valida, assina e — com `--enviar` — transmite | [Emitir uma nota](../guias/emitir.md) |
| `nfse enviar <arquivo.xml>` | transmite uma DPS que já foi gerada e assinada | [Enviar à Sefin Nacional](../guias/enviar.md) |
| `nfse consultar <chave>` | busca a NFS-e, ou a chave a partir do identificador da DPS | [Consultar uma nota](../guias/consultar.md) |
| `nfse danfse <arquivo.xml>` | gera o DANFSe em PDF a partir do XML da nota | [Entregar o PDF ao cliente](../guias/danfse.md) |
| `nfse cancelar <chave>` | registra o evento de cancelamento | [Cancelar uma nota](../guias/cancelar.md) |
| `nfse numero ver` / `definir` | consulta e ajusta o contador da série | [A numeração](../guias/numeracao.md) |
| `nfse versao` | mostra a versão instalada, a mesma que vai no `verAplic` | [Instalação](../comecar/instalacao.md) |

Todos aceitam `--help`, que lista as opções de cada um.

## Variáveis de ambiente

| Variável | Para quê |
|---|---|
| `NFSE_CERT_SENHA` | a senha do certificado; preferível a `--senha`, que fica visível na lista de processos |
