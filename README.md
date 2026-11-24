# PDF Converter — estrutura inicial

Conversor de PDF em PNG, com possibilidade de executar em lote, feito em Go com servidor HTTP local e interface HTML/CSS/JavaScript.

## Requisitos

- Go 1.22 ou superior.
- Navegador instalado.

## Executar

No terminal, dentro da pasta do projeto:

```powershell
go run .
```

O programa inicia um servidor em `127.0.0.1` usando uma porta livre e tenta abrir o navegador automaticamente.

Para compilar:

```powershell
go build -o pdfconverter.exe .
```

No Linux:

```bash
go build -o pdfconverter .
```