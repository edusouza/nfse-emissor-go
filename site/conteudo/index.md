# nfse

Emissor de NFS-e (Nota Fiscal de Serviço eletrônica) em linha de comando, para o
**Sistema Nacional NFS-e**.

Voltado a prestadores de serviço do Simples Nacional — MEI, ME e EPP — que
querem emitir as próprias notas a partir do terminal ou de um script, sem
depender de portal web.

!!! warning "Ainda não emite com valor fiscal"
    Tudo o que está descrito aqui foi testado contra a Sefin Nacional em
    **produção restrita**: o ambiente real do governo, onde as notas não têm
    valor fiscal. Emitir em produção usa o mesmo caminho técnico, mas ainda não
    foi confirmado. É o que falta para a versão `1.0.0`.

```bash
go install github.com/edusouza/nfse-emissor-go/cmd/nfse@latest
```

## O que funciona

O ciclo completo de uma nota — configurar, achar o código do serviço, montar,
validar, assinar, transmitir, consultar, substituir e cancelar — exercitado
ponta a ponta contra a Sefin Nacional em produção restrita. E o **DANFSe**, o
documento auxiliar que se entrega ao cliente, desenhado aqui conforme a NT 008.

Configurar a partir do certificado, achar o código do serviço, emitir e gerar o
PDF:

```bash
nfse onboard --certificado certificado.pfx
nfse servico buscar suporte tecnico
nfse emitir --valor 1500 --descricao "Consultoria - agosto/2026" --enviar
nfse danfse notas/41069022212345678000195000000000000126081234567890-nfse.xml
```

## O que ainda não

- **Emissão em produção**, com valor fiscal. É o mesmo caminho técnico; o que
  muda é a consequência de errar.
- As regras que dependem do convênio do município com o Sistema Nacional —
  veja [o que a validação local cobre](referencia/validacao.md).

## Por onde começar

- **Ainda sem certificado?** [Experimente em 2 minutos](comecar/experimentar.md),
  com um certificado descartável gerado na hora.
- **Com o A1 em mãos:** [configure em um comando](comecar/configurar.md) e
  [emita a primeira nota](comecar/primeira-nota.md).
- **Recebeu uma rejeição?** Procure o código em
  [códigos de rejeição](referencia/rejeicoes.md) — a lista oficial, com a regra
  que cada um aplica.
- **Quer entender por que o código é como é?** As
  [decisões de arquitetura](decisoes/index.md) contam o problema, o que foi
  decidido e o que se aprendeu, inclusive os defeitos que só a emissão real
  revelou.

O código está no [GitHub](https://github.com/edusouza/nfse-emissor-go), sob a
licença MIT.
