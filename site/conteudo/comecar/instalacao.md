# Instalação

```bash
go install github.com/edusouza/nfse-emissor-go/cmd/nfse@latest
```

Para fixar uma versão, troque `@latest` pela tag desejada; as versões estão no
[changelog](../changelog.md). O `nfse versao` mostra o que está instalado — e é
o mesmo identificador que vai no `verAplic` de cada declaração.

Ou compilando a partir do código:

```bash
git clone https://github.com/edusouza/nfse-emissor-go.git
cd nfse-emissor-go
go build -o nfse ./cmd/nfse
```

Requer Go 1.26 ou superior. O resultado é um binário único, sem `cgo` e sem
dependência de serviço externo.

Todos os comandos aceitam `--help`. A lista completa está em
[comandos](../referencia/comandos.md).

## Próximo passo

- Sem um certificado A1 ainda: [experimente com um descartável](experimentar.md).
- Com o certificado: [configure o emissor](configurar.md).
