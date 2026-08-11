# Sprint 7 — Redis, Kafka, Outbox e Kubernetes

## Objetivo

Evoluir o `gosvc` para gerar serviços distribuídos com publicação assíncrona
confiável, idempotência, execução separada de workers e manifests Kubernetes.

## Novo preset

```bash
gosvc new order-events \
  --module github.com/acme/order-events \
  --preset event-driven-api
```

O preset `event-driven-api` inclui tudo do `production-api` e adiciona:

- Redis;
- Kafka por meio de franz-go;
- Transactional Outbox;
- worker separado da API;
- idempotência de consumidores;
- retry com backoff;
- dead-letter topic;
- Docker Compose para o ambiente distribuído;
- imagem de migration separada;
- manifests Kubernetes.

## Configuração declarativa

Foram adicionadas as seções:

```yaml
cache:
  enabled: true
  provider: redis
  address: localhost:6379
  db: 0

messaging:
  enabled: true
  provider: kafka
  brokers: localhost:9092
  topic_prefix: events
  consumer_group: service-workers
  max_retries: 5
  retry_backoff: 1s
  dlq_suffix: .dlq

outbox:
  enabled: true
  poll_interval: 1s
  batch_size: 100
  max_attempts: 10

deployment:
  kubernetes: true
  namespace: order-events
  replicas: 2
```

A validação impede a criação do preset quando Redis, Kafka, Outbox ou
Kubernetes estão desabilitados ou possuem valores inválidos.

## Transactional Outbox

O projeto gerado contém:

```text
internal/domain/event.go
internal/ports/events.go
internal/application/outbox_worker.go
internal/infrastructure/persistence/postgres/outbox_repository.go
db/migrations/900100_create_outbox.up.sql
db/migrations/900100_create_outbox.down.sql
```

O repository aceita uma abstração implementada tanto por `pgxpool.Pool` quanto
por `pgx.Tx`. Isso permite inserir a alteração de negócio e o evento Outbox na
mesma transação PostgreSQL.

Exemplo conceitual:

```go
err := postgres.WithinTransaction(ctx, pool, func(tx pgx.Tx) error {
    orderRepository := postgres.NewOrderRepository(sqlc.New(tx))
    outboxRepository := postgres.NewOutboxRepository(tx)

    if err := orderRepository.Create(ctx, order); err != nil {
        return err
    }

    return outboxRepository.Enqueue(ctx, event)
})
```

O worker usa `FOR UPDATE SKIP LOCKED` para permitir múltiplas réplicas sem que
duas instâncias processem o mesmo lote simultaneamente.

## Retry e Dead Letter

O worker:

1. busca um lote de eventos pendentes;
2. incrementa a quantidade de tentativas;
3. publica no tópico Kafka;
4. marca o evento como publicado;
5. registra falhas transitórias;
6. envia para `<topic>.dlq` ao alcançar o limite de tentativas;
7. marca falhas terminais.

A configuração é controlada por:

```text
OUTBOX_POLL_INTERVAL
OUTBOX_BATCH_SIZE
OUTBOX_MAX_ATTEMPTS
KAFKA_MAX_RETRIES
KAFKA_RETRY_BACKOFF
KAFKA_DLQ_SUFFIX
```

## Kafka

Foram gerados:

```text
internal/infrastructure/messaging/kafka/publisher.go
internal/infrastructure/messaging/kafka/consumer.go
```

O publisher usa produção síncrona para que o Outbox somente marque um evento
como publicado depois da confirmação do broker.

O consumer:

- utiliza consumer group;
- cria uma chave de idempotência por tópico, partição e offset;
- executa retries limitados;
- envia mensagens permanentemente inválidas ao tópico DLQ;
- realiza commit somente após sucesso ou publicação na DLQ.

## Redis e idempotência

Foi gerado:

```text
internal/infrastructure/cache/redis/store.go
```

O adapter implementa a porta `IdempotencyStore` usando `SET NX` com TTL.

A chave recebe o prefixo:

```text
idempotency:
```

Redis é validado com `PING` durante a inicialização do adapter.

## Worker separado

Foram adicionados:

```text
cmd/worker/main.go
internal/bootstrap/worker.go
```

Execução:

```bash
make run-worker
```

O worker possui lifecycle próprio, signal handling, logger e conexões com
PostgreSQL e Kafka.

## Docker

A imagem principal contém dois binários:

```text
/app/order-events
/app/order-events-worker
```

O Docker Compose inclui:

```text
postgres
redis
kafka
api
worker
prometheus
otel-collector
```

Também foi gerado:

```text
Dockerfile.migrate
```

Isso evita colocar a CLI de migrations dentro da imagem de runtime da API.

## Kubernetes

Arquivos gerados:

```text
deployments/k8s/00-namespace.yaml
deployments/k8s/10-configmap.yaml
deployments/k8s/11-secret.example
deployments/k8s/20-api-deployment.yaml
deployments/k8s/21-worker-deployment.yaml
deployments/k8s/30-service.yaml
deployments/k8s/40-migration-job.yaml
deployments/k8s/50-network-policy.yaml
```

Os Deployments incluem:

- usuário não-root;
- seccomp `RuntimeDefault`;
- capabilities removidas;
- root filesystem somente leitura;
- requests e limits;
- liveness e readiness probes na API;
- réplicas independentes para API e worker.

O arquivo Secret é gerado sem extensão YAML para não ser aplicado
acidentalmente por `kubectl apply -f deployments/k8s`.

Antes do deploy:

```bash
cp deployments/k8s/11-secret.example deployments/k8s/11-secret.yaml
```

O Secret real é ignorado pelo Git e pelo Docker build context.

Comandos:

```bash
make k8s-validate
make k8s-apply
```

Os manifests assumem PostgreSQL, Redis, Kafka e OpenTelemetry externos ou
gerenciados. O framework não tenta instalar stateful dependencies dentro do
mesmo deployment da aplicação.

## Diagnóstico

O `gosvc doctor` passa a exigir `kubectl` no preset `event-driven-api`.

A saída também continua verificando:

- versão mínima do Go;
- Docker e Docker Compose;
- sqlc;
- oapi-codegen;
- golang-migrate;
- golangci-lint;
- govulncheck.

## Testes adicionados

Foram adicionados testes para:

- defaults e validação do preset distribuído;
- composição das features do preset;
- geração dos adapters Redis e Kafka;
- geração do worker e da migration Outbox;
- geração dos manifests Kubernetes;
- publicação normal de eventos;
- falha transitória de publicação;
- roteamento para DLQ;
- validação da configuração do worker;
- compilação do projeto gerado com stubs compatíveis;
- coverage gate mínimo de 80%;
- idempotência da regeneração.

## Validações executadas

```bash
gofmt -w .
go test ./...
go test -race ./...
go vet ./...
go build ./...
make verify
```

Também foram executados no projeto gerado:

```bash
gosvc validate --project .
gosvc check architecture --project .
gosvc verify --project . --static
```

Todos os arquivos YAML gerados foram parseados com sucesso.

## Limitações do ambiente

O ambiente local desta sessão não disponibiliza:

- Docker;
- PostgreSQL;
- Redis;
- Kafka;
- kubectl conectado a um cluster;
- acesso ao proxy público de módulos Go.

Por isso, os testes de compilação do projeto gerado utilizam stubs locais
compatíveis com pgx, Redis, franz-go, JWT, Prometheus e OpenTelemetry.

A execução real de containers, migrations, publicação Kafka e manifests
Kubernetes deve ser validada no CI conectado ou em um ambiente de staging.

## Próxima evolução sugerida

A próxima sprint pode implementar:

- `gosvc upgrade`;
- migrations do schema do manifesto;
- relatório de conflitos de upgrade;
- sistema de plugins;
- geração de consumidores a partir de contratos de evento;
- AsyncAPI;
- Schema Registry;
- políticas de compatibilidade de eventos.
