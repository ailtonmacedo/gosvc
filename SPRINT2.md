# Sprint 2 — Issues 7 a 14

## Objetivo

Transformar a CLI da Sprint 1 em um gerador funcional e seguro de projetos.

## Entregue

### Issue 7 — Sistema de presets

- Registry interno de presets.
- Presets `minimal-api` e `postgres-api`.
- Features declaradas por preset.
- Erro informativo para preset desconhecido.

### Issue 8 — Representação de artefatos

- `Artifact` com path, conteúdo, permissão e ownership.
- Ownership `generated` e `user`.
- Bloqueio de paths absolutos, `../`, paths não normalizados e backslashes.

### Issue 9 — Templates

- Templates embutidos no binário com `go:embed`.
- Renderização por `text/template`.
- `missingkey=error`.
- Formatação automática de arquivos Go com `go/format`.
- Templates condicionais por preset.

### Issue 10 — Escrita atômica

- Geração em diretório temporário no mesmo filesystem.
- Cópia segura do projeto existente para staging.
- Aplicação das mudanças no staging.
- Troca por rename com backup e rollback básico.
- Rejeição de symlinks durante a cópia e no destino.

### Issue 11 — Manifesto interno

- `.gosvc/manifest.json`.
- Versão do framework.
- Versão do schema.
- Preset e features.
- Ownership e checksum SHA-256 de cada artefato.
- JSON determinístico e ordenado.

### Issue 12 — Proteção de arquivos

- Arquivos `user` modificados são preservados e apresentados como `PROTECT`.
- Arquivos `generated` modificados causam conflito.
- `--force` permite sobrescrita explícita dos artefatos conflitantes.
- Diretórios não vazios sem manifesto não são assumidos pelo gerador.

### Issue 13 — Idempotência

- Segunda execução com a mesma configuração não altera o projeto.
- Arquivos inalterados são apresentados como `SKIP`.
- Saída final: `No changes required.`
- Testes automatizados cobrem a idempotência.

### Issue 14 — Comando `gosvc new`

- Geração funcional.
- Flags antes ou depois do nome do projeto.
- `--module`.
- `--preset`.
- `--output`.
- `--dry-run`.
- `--force`.
- Resumo das operações `CREATE`, `UPDATE`, `SKIP` e `PROTECT`.

## Estrutura gerada

```text
order-service/
├── .gosvc/
│   └── manifest.json
├── cmd/
│   └── api/
│       └── main.go
├── internal/
│   ├── application/
│   │   └── doc.go
│   ├── bootstrap/
│   │   └── doc.go
│   ├── config/
│   │   └── doc.go
│   ├── domain/
│   │   └── doc.go
│   ├── infrastructure/
│   │   └── doc.go
│   └── ports/
│       └── doc.go
├── .gitignore
├── Makefile
├── README.md
├── go.mod
└── project.yaml
```

O preset `postgres-api` adiciona:

```text
db/
├── migrations/
│   └── README.md
└── queries/
    └── README.md
```

Esses diretórios são preparatórios. A integração funcional com pgxpool,
`golang-migrate` e sqlc pertence à Sprint 3.

## Testes implementados

- resolução e isolamento dos presets;
- serialização determinística do manifesto;
- validação de paths de artefatos;
- geração de projeto compilável;
- execução de `go test ./...` no projeto gerado;
- idempotência da segunda geração;
- proteção de README alterado;
- conflito em Makefile gerado alterado;
- recuperação com `--force`;
- dry run sem escrita;
- layout do preset PostgreSQL;
- rejeição de diretório desconhecido não vazio;
- parsing de flags depois do nome do projeto;
- execução do comando `new` pela CLI.

## Validação executada

```bash
go mod tidy
go test ./...
go vet ./...
go build ./...
make verify
make build VERSION=0.2.0
```

O projeto gerado também foi validado com:

```bash
go test ./...
go vet ./...
go build ./...
```

## Limitações conscientes

- O projeto gerado ainda não inicia um servidor Chi.
- O preset PostgreSQL ainda não instala pgxpool.
- Migrations e sqlc ainda não estão funcionais.
- OpenAPI, Docker e CI ainda não são gerados.
- O parser YAML continua limitado ao schema atual.
- O swap atômico depende de origem e destino estarem no mesmo filesystem.

## Próxima entrega

Sprint 3, correspondente às Issues 15 a 21:

- estrutura Clean Architecture funcional;
- bootstrap explícito;
- servidor HTTP com Chi;
- middlewares básicos;
- health checks;
- graceful shutdown;
- Dockerfile;
- pipeline CI básico.
