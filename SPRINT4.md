# Sprint 4 — OpenAPI e geração de CRUD

## Objetivo

Transformar o `postgres-api` em um gerador de recursos completos orientado por
contrato, mantendo idempotência, proteção de arquivos e migrations estáveis.

## Escopo entregue

A entrega cobre as Issues 29 a 36 do planejamento.

## Issue 29 — Feature OpenAPI

O preset `postgres-api` agora gera:

```text
api/openapi.yaml
api/oapi-codegen.yaml
internal/generated/openapi/spec.gen.go
```

O contrato é reconstruído a partir do catálogo de recursos registrado em:

```text
.gosvc/resources.json
```

Cada recurso adiciona:

- endpoint de listagem;
- endpoint de criação;
- endpoint de consulta por ID;
- endpoint de atualização;
- endpoint de exclusão;
- schemas de entrada;
- schemas de saída;
- schema de listagem;
- respostas de erro conhecidas.

## Integração com oapi-codegen

Configuração gerada:

```yaml
package: openapi
output: internal/generated/openapi/oapi.gen.go

generate:
  models: true
  chi-server: true
  strict-server: true

output-options:
  skip-prune: true
```

Comandos:

```bash
make tools
make generate
```

O Makefile fixa o `oapi-codegen` em `v2.4.1`.

## Issue 30 — Validação de requisições

O router utiliza:

```text
github.com/oapi-codegen/nethttp-middleware
```

A validação ocorre dentro do grupo das rotas de API e verifica o contrato antes
do handler.

Configuração aplicada:

- `OapiRequestValidatorWithOptions`;
- validação de servidores desabilitada no middleware;
- servidores removidos do contrato carregado;
- resposta JSON padronizada;
- `request_id` incluído no erro;
- erro interno do validador não exposto ao cliente.

Exemplo de resposta:

```json
{
  "code": "openapi_validation_failed",
  "message": "Request does not match the OpenAPI contract.",
  "request_id": "..."
}
```

## Issue 31 — Comando `add resource`

Exemplo:

```bash
gosvc add resource product \
  --fields 'id:uuid,name:string,price:decimal,active:bool,released_at:datetime' \
  --crud \
  --project ./catalog-service
```

Opções:

```text
--project
--dry-run
--force
```

Regras:

- disponível inicialmente somente para `postgres-api`;
- `--crud` é obrigatório;
- `id` é obrigatório;
- `id` deve usar `int64` ou `uuid`;
- nomes seguem `snake_case` minúsculo;
- campos duplicados são rejeitados;
- recurso existente com a mesma definição é idempotente;
- recurso existente com definição diferente produz conflito.

Tipos suportados:

```text
uuid
string
int64
integer
decimal
bool
datetime
```

`integer` e `decimal` são normalizados para `int64` nesta versão.

## Issue 32 — Domínio e ports

Para um recurso `product`, são gerados:

```text
internal/domain/product.go
internal/domain/product_errors.go
internal/ports/product_repository.go
```

Características:

- entidade sem dependência de Chi ou pgx;
- sentinel error `ErrProductNotFound`;
- repository definido como interface;
- `context.Context` em todas as operações;
- identificadores UUID representados por `uuid.UUID`.

## Issue 33 — Casos de uso CRUD

Arquivo:

```text
internal/application/product_service.go
```

Operações:

```text
Create
GetByID
List
Update
Delete
```

O service depende da porta de repository, não da implementação PostgreSQL.
Erros são propagados com contexto por meio de `%w`.

## Issue 34 — Persistência, queries e migrations

Arquivos gerados:

```text
internal/infrastructure/persistence/postgres/product_repository.go
internal/generated/sqlc/products.sql.go
db/queries/products.sql
db/migrations/000002_create_products.up.sql
db/migrations/000002_create_products.down.sql
```

Práticas aplicadas:

- placeholders PostgreSQL;
- `context.Context`;
- `rows.Close()`;
- `rows.Err()`;
- tradução de `pgx.ErrNoRows`;
- código inicial compatível com o contrato esperado do sqlc;
- `CREATE EXTENSION IF NOT EXISTS pgcrypto` para IDs UUID;
- `gen_random_uuid()` como default de chave primária UUID.

### Compatibilidade UUID com sqlc

O `sqlc.yaml` usa diretórios para considerar todos os recursos:

```yaml
schema: db/migrations
queries: db/queries
```

Também possui override explícito:

```yaml
overrides:
  - db_type: uuid
    go_type:
      import: github.com/google/uuid
      type: UUID
```

Isso mantém o mesmo tipo entre domínio, ports, handlers, repositories e código
regenerado pelo sqlc.

## Migrations estáveis

Cada recurso possui um número de migration persistido no registro:

```json
{
  "name": "product",
  "plural": "products",
  "migration": 2
}
```

Cenário validado:

```text
order   → 000001
product → 000002
account → 000003
```

A ordenação alfabética do catálogo não altera números já atribuídos.
Migrations negativas ou duplicadas são rejeitadas.

## Issue 35 — Handlers e rotas CRUD

Arquivos:

```text
internal/infrastructure/http/product_handler.go
internal/infrastructure/http/product_handler_test.go
internal/bootstrap/resources.gen.go
```

Rotas:

```text
GET    /products
POST   /products
GET    /products/{id}
PUT    /products/{id}
DELETE /products/{id}
```

Características:

- handlers dependem de interfaces de caso de uso;
- JSON inválido retorna `400`;
- ID `int64` é validado por `strconv.ParseInt`;
- ID UUID é validado por `uuid.Parse`;
- domínio não recebe conceitos HTTP;
- respostas usam nomes JSON em `snake_case`;
- erros não encontrados retornam `404`;
- erros desconhecidos retornam mensagem genérica `500`.

## Issue 36 — ReDoc

Endpoints gerados:

```text
GET /docs
GET /openapi.yaml
```

`/docs` entrega uma página ReDoc que consome o contrato publicado pela própria
aplicação.

## Geração atômica e idempotência

O comando `add resource` reutiliza o mesmo pipeline do comando `new`:

```text
Render
Validate
Plan
Detect conflicts
Stage
Apply atomically
Update manifest
```

Segunda execução com a mesma definição:

```text
Resource product already exists; no changes required.
```

Arquivos do usuário continuam protegidos e arquivos controlados pelo gerador
continuam validados por checksum.

## CI gerado

No preset `postgres-api`, o workflow executa:

```bash
make tools
make generate
go test ./...
```

O bloco não é gerado no `minimal-api`, pois esse preset não possui os targets de
geração externa.

Nesta sprint, a pipeline valida que os geradores executam e que o projeto
continua compilando. A comparação byte a byte do output real dos geradores com
o snapshot inicial ainda não é aplicada, porque ela exige consolidar a
estratégia de ownership dos arquivos regenerados.

## Testes adicionados e atualizados

- parsing do comando `add resource`;
- geração CRUD completa;
- idempotência do recurso;
- rejeição de ID string;
- normalização de decimal;
- UUID em domínio, repository e handlers;
- override UUID do sqlc;
- schema e queries por diretório;
- migrations UUID;
- numeração estável de migrations;
- rejeição de migration negativa;
- geração do contrato OpenAPI;
- geração de ReDoc;
- middleware de validação;
- compilação do projeto com múltiplos recursos;
- CI específico para cada preset.

## Validação executada

```bash
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
go build ./...
make verify
```

Também foram validados manualmente:

- criação do `catalog-service`;
- adição do recurso UUID `product`;
- adição posterior do recurso `account`;
- migrations `000001`, `000002` e `000003` preservadas;
- segunda adição de `product` sem alterações;
- contrato OpenAPI parseado com seis paths;
- registro de recursos com migrations estáveis;
- configuração sqlc com override UUID.

## Limitações conhecidas

### Adapter strict-server

O `oapi-codegen` gera models, interface Chi e interface strict-server, mas as
rotas em runtime ainda são registradas diretamente pelos handlers CRUD. A
próxima evolução deve implementar o adapter strict para ligar os tipos gerados
aos casos de uso e adicionar testes de contrato sobre esse caminho.

### Geradores externos

O ambiente desta entrega não permite baixar módulos Go nem executar Docker e
PostgreSQL. Portanto, não foi possível executar ao vivo:

```text
make tools
make generate
make migrate-up
docker compose up
```

A suíte compila os projetos gerados com stubs locais compatíveis com as APIs
consumidas. Uma execução completa em ambiente conectado continua obrigatória
antes de publicar um release.

### Pluralização

A pluralização atual cobre regras simples em inglês. Nomes irregulares deverão
receber uma opção explícita `--plural` em uma versão futura.

### Decimal

`decimal` ainda é normalizado para `int64`. Não há suporte a escala ou precisão
arbitrária nesta versão.
