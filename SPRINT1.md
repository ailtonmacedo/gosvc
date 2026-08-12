# Sprint 1 — Issues 1 a 6

## Entregue

- Estrutura inicial do módulo Go.
- Executável em `cmd/gosvc`.
- Ajuda global da CLI.
- Comando `version` e flag `--version`.
- Injeção de versão, commit e data por `ldflags`.
- Tratamento central de erros e exit codes.
- Flag global `--debug`.
- Modelo inicial do `project.yaml`.
- Defaults seguros para arquitetura, HTTP, deploy e cobertura.
- Validação agregada de regras semânticas.
- Rejeição de campos desconhecidos.
- Comando funcional `validate-config`.
- Contrato inicial do comando `new`.
- Testes unitários.
- Makefile para build e quality checks.

## Decisão temporária sobre YAML

O ambiente usado para montar esta entrega não possuía acesso ao proxy de módulos Go. Para manter o scaffold compilável e testado, foi criado um parser estrito e dependency-free para o subset inicial do `project.yaml`.

Ele suporta:

- mappings aninhados;
- strings;
- inteiros;
- booleanos;
- durações representadas como strings, como `10s` e `1m`.

Ele rejeita explicitamente:

- listas;
- anchors e aliases;
- tags;
- valores multilinha;
- chaves duplicadas;
- campos desconhecidos;
- indentação com tabs.

A função pública `project.Load` mantém essa implementação encapsulada. Portanto, o parser poderá ser substituído por uma biblioteca YAML completa sem alterar a CLI.

## Validação executada

```bash
go mod tidy
go test ./...
go vet ./...
go build ./...
make build VERSION=0.1.0
./bin/gosvc version
./bin/gosvc validate-config --print ./examples/project.yaml
./bin/gosvc new --help
```

## Próxima sprint

Issues 7 a 14:

- sistema de presets;
- representação de artefatos;
- templates embutidos;
- escrita atômica;
- manifesto `.gosvc/manifest.json`;
- proteção de arquivos;
- idempotência;
- implementação funcional de `gosvc new`.
