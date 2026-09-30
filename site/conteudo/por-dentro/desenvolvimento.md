# Desenvolvimento

```bash
go build -o nfse ./cmd/nfse   # compilar
go test -short ./...          # rápida (~2s), pulando os testes dependentes de relógio
go test ./...                 # suíte completa (~75s: inclui testes de backoff reais)
go vet ./...
gofmt -l ./cmd ./internal ./pkg
```

## Testes de contrato

Os clientes da BrasilAPI e do IBGE foram escritos a partir da documentação
desses serviços, e os testes de unidade usam servidores locais. Os testes de
contrato conferem as respostas reais e ficam desligados por padrão; a CI os
roda quando os clientes mudam, toda semana e sob demanda:

```bash
NFSE_TESTE_CONTRATO=1 go test -run Contrato -v ./internal/infrastructure/brasilapi/ ./internal/infrastructure/ibge/

# para conferir também as opções pelo MEI e pelo Simples de um MEI de verdade
NFSE_TESTE_CONTRATO=1 NFSE_TESTE_CNPJ_MEI=<cnpj> go test -run Contrato -v ./internal/infrastructure/brasilapi/
```

## Este site

As páginas escritas à mão estão em `site/conteudo/`. As que vêm do repositório
— os [códigos de rejeição](../referencia/rejeicoes.md), as
[decisões](../decisoes/index.md) e o [changelog](../changelog.md) — são geradas
por `internal/sitegen` e não ficam no repositório. Por quê:
[ADR 0016](../decisoes/0016-site-de-documentacao.md).

```bash
go run ./internal/sitegen                 # gera as páginas derivadas
python -m pip install -r site/requirements.txt
cd site
zensical serve                            # prévia em http://localhost:8000
zensical build --strict                   # o que a CI roda
```

O site é publicado pelo workflow `pages.yml` a cada mudança no `master`. Nos
pull requests, ele só é construído, com `--strict`: um link quebrado reprova a
mudança.

Os exemplos das páginas passam pelos mesmos testes do README
(`internal/docs`): uma chave de acesso ou um código de serviço que o `nfse`
recusaria reprova a suíte.
