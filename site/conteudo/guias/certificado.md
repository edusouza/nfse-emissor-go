# O certificado e a senha

O `nfse` assina cada declaração com o seu certificado **A1**: um arquivo
`.pfx` ou `.p12` emitido por uma Autoridade Certificadora da ICP-Brasil.

## Verificar o certificado

Antes de emitir qualquer coisa, confirme que o certificado está legível e
dentro da validade:

```bash
export NFSE_CERT_SENHA='sua-senha'
nfse cert info --arquivo certificado.pfx
```

```
Titular                 EMPRESA EXEMPLO LTDA:12345678000195
Emissor                 AC CERTISIGN RFB G5
Numero de serie         4A3B2C1D...
Valido de               10/03/2026 09:14
Valido ate              10/03/2027 09:14
Dias restantes          173
Tamanho da chave        2048 bits
Certificados na cadeia  2

Certificado apto a assinar uma DPS.
```

O comando sai com código diferente de zero se o certificado não puder assinar
uma DPS — útil para usar em script.

## A senha

Há três formas de informá-la, nesta ordem de precedência:

1. `--senha` — direto na linha de comando;
2. `NFSE_CERT_SENHA` — variável de ambiente;
3. prompt interativo, quando nenhuma das anteriores é usada e há um terminal.

!!! warning "Prefira a variável de ambiente ou o prompt"
    Argumentos de linha de comando ficam visíveis para qualquer processo que
    consiga ler a lista de processos do sistema, e costumam ficar gravados no
    histórico do shell.
