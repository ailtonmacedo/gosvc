# Sprint 3 — HTTP, Docker e PostgreSQL funcional

## Objetivo

Transformar os presets da Sprint 2 em aplicações executáveis, incluindo o
servidor HTTP e a infraestrutura PostgreSQL do `postgres-api`.

## Escopo entregue

A entrega consolida as Issues 15 a 28 do planejamento.

## Issue 15 — Clean Architecture funcional

- `internal/domain` para entidades e erros.
- `internal/application` para casos de uso.
- `internal/ports` para contratos.
- `internal/infrastructure` para adapters.
- `internal/bootstrap` para composição explícita.
- `internal/config` para configuração de runtime.
- `main.go` limitado à inicialização e tratamento do erro final.

## Issue 16 — Servidor HTTP com Chi

- `http.Server` configurado explicitamente.
- Read timeout.
- Write timeout.
- Idle timeout.
- Shutdown timeout.
- Porta por variável de ambiente.
- Limite configurável do body.

## Issue 17 — Middlewares HTTP

- Request ID.
- Real IP.
- Recoverer.
- Timeout.
- Limite de body.
- Headers de segurança.
- Content-Type JSON consistente nos health checks.

## Issue 18 — Health checks

```text
GET /health/live
GET /health/ready
```

No preset PostgreSQL, readiness executa `pool.Ping(ctx)`.

## Issue 19 — Graceful shutdown

- Captura de SIGINT.
- Captura de SIGTERM.
- `http.Server.Shutdown` com timeout.
- Fechamento do pool PostgreSQL.
- Combinação de erros independentes com `errors.Join`.

## Issue 20 — Docker

- Multi-stage build.
- Build com `CGO_ENABLED=0`.
- `-trimpath`.
- Version, commit e build time por `ldflags`.
- Runtime distroless.
- Usuário não-root.
- `.dockerignore`.

## Issue 21 — CI básico

Workflow gerado em:

```text
.github/workflows/ci.yaml
```

Etapas:

- gofmt;
- go mod tidy;
- go vet;
- go test;
- go build;
- docker build.

## Issue 22 — Feature PostgreSQL

O preset `postgres-api` declara:

```text
postgres
migrations
sqlc
docker-compose
```

O `project.yaml` passa a aceitar:

```yaml
database:
  enabled: true
  engine: postgres
  driver: pgx
  pool: pgxpool
  migrations: golang-migrate
  code_generation: sqlc
```

## Issue 23 — Pool PostgreSQL

- `pgxpool.ParseConfig`.
- Max e min connections.
- Max connection lifetime.
- Max idle time.
- Health-check period.
- Ping durante inicialização.
- Fechamento durante shutdown.

Variáveis geradas:

```text
DATABASE_URL
DATABASE_POOL_MAX_CONNS
DATABASE_POOL_MIN_CONNS
DATABASE_POOL_MAX_CONN_LIFETIME
DATABASE_POOL_MAX_CONN_IDLE_TIME
DATABASE_HEALTH_CHECK_PERIOD
DATABASE_QUERY_TIMEOUT
```

## Issue 24 — Migrations

```text
db/migrations/
├── 000001_create_orders.up.sql
└── 000001_create_orders.down.sql
```

Comandos:

```bash
make migrate-up
make migrate-down
make migrate-version
```

As migrations não são executadas automaticamente pela API.

## Issue 25 — sqlc

```text
sqlc.yaml
db/queries/orders.sql
internal/generated/sqlc/
├── db.go
├── models.go
└── orders.sql.go
```

Configuração:

```yaml
sql_package: pgx/v5
```

O baseline gerado corresponde ao contrato inicial do sqlc e pode ser
regenerado com:

```bash
make tools
make generate
```

## Issue 26 — Repository PostgreSQL

CRUD de persistência para `Order`:

- Create;
- GetByID;
- List;
- Update;
- Delete.

Práticas aplicadas:

- context em todas as operações;
- placeholders `$1`, `$2` e `$3`;
- `rows.Close()`;
- `rows.Err()`;
- `pgx.ErrNoRows` traduzido para `domain.ErrOrderNotFound`;
- contexto adicionado ao propagar erros;
- adapter validado contra `ports.OrderRepository`.

## Suporte a transações

Foi incluído `WithinTransaction` com:

- Begin;
- Commit;
- Rollback em erro;
- rollback antes de repassar panic;
- `errors.Join` para preservar falha original e falha de rollback.

## Issue 27 — Docker Compose

Serviços:

```text
api
postgres
```

O PostgreSQL possui:

- volume persistente;
- health check com `pg_isready`;
- dependência da API condicionada ao estado saudável.

## Issue 28 — Preset `postgres-api`

O comando:

```bash
gosvc new order-service \
  --module github.com/acme/order-service \
  --preset postgres-api
```

gera uma aplicação com HTTP, Docker, PostgreSQL, migrations e sqlc.

## Estrutura principal gerada

```text
order-service/
├── .github/workflows/ci.yaml
├── .gosvc/manifest.json
├── cmd/api/main.go
├── db/
│   ├── migrations/
│   └── queries/
├── internal/
│   ├── application/
│   ├── bootstrap/
│   ├── config/
│   ├── domain/
│   ├── generated/sqlc/
│   ├── infrastructure/
│   │   ├── http/
│   │   └── persistence/postgres/
│   └── ports/
├── tests/integration/
├── .dockerignore
├── .env.example
├── compose.yaml
├── Dockerfile
├── Makefile
├── project.yaml
├── sqlc.yaml
└── go.mod
```

## Testes adicionados

- defaults específicos do preset PostgreSQL;
- rejeição de `database.enabled: false` no `postgres-api`;
- features operacionais do preset;
- presença de migrations, queries, sqlc, pool e Compose;
- compilação do projeto `minimal-api` gerado;
- compilação do projeto `postgres-api` gerado;
- execução dos testes HTTP gerados;
- idempotência;
- ownership e conflitos mantidos.

## Estratégia de compilação no ambiente

O ambiente usado para esta entrega não possui acesso ao Go module proxy.
Por isso, os testes de compilação dos projetos gerados usam módulos locais que
reproduzem somente as APIs de Chi e pgx consumidas pelos templates.

Isso valida:

- imports;
- tipos;
- assinaturas;
- composição;
- testes HTTP;
- compatibilidade interna entre arquivos gerados.

Não substitui a validação em um ambiente conectado.

## Validação executada

```bash
gofmt -w .
go test ./...
go vet ./...
go build ./...
```

Também foram validados manualmente:

```bash
gosvc new order-service --module github.com/acme/order-service --preset postgres-api
gosvc validate-config --print order-service/project.yaml
gosvc new order-service --module github.com/acme/order-service --preset postgres-api
```

A segunda geração retorna apenas `SKIP` e `No changes required.`

## Limitações conscientes

- Não foi possível baixar os módulos reais no ambiente de execução.
- Docker e Docker Compose não estão instalados no ambiente.
- Não havia servidor PostgreSQL disponível para executar migrations.
- O binário real do sqlc não pôde ser executado.
- O CRUD HTTP/OpenAPI pertence à próxima entrega.
- `DATABASE_QUERY_TIMEOUT` já é carregado, mas será aplicado aos casos de uso e
  repositories quando o CRUD for conectado.

## Próxima entrega

Sprint 4:

- contrato OpenAPI;
- oapi-codegen;
- validação de requests;
- comando `gosvc add resource`;
- CRUD HTTP de Order;
- ReDoc;
- testes de contrato.
