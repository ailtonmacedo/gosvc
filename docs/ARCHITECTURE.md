# Generated Architecture

O código gerado segue Clean Architecture em camadas:

```text
HTTP / PostgreSQL / Kafka / Redis
             │
             ▼
      infrastructure
             │
             ▼
          ports
             │
             ▼
       application
             │
             ▼
          domain
```

Dependências do núcleo não apontam para infraestrutura. `gosvc check architecture --project .` valida essa regra.

Os exemplos abaixo são trechos do `postgres-api` gerado pelo baseline atual.

## Domain

```go
type Order struct {
    ID         int64
    Status     string
    TotalCents int64
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

## Port

```go
type OrderRepository interface {
    Create(context.Context, domain.Order) (domain.Order, error)
    GetByID(context.Context, int64) (domain.Order, error)
    List(context.Context, int32, int32) ([]domain.Order, error)
    Update(context.Context, domain.Order) (domain.Order, error)
    Delete(context.Context, int64) error
}
```

## Application service

```go
type OrderService struct {
    repository ports.OrderRepository
}

func (s *OrderService) Create(
    ctx context.Context,
    status string,
    totalCents int64,
) (domain.Order, error) {
    order, err := domain.NewOrder(status, totalCents)
    if err != nil {
        return domain.Order{}, err
    }
    return s.repository.Create(ctx, order)
}
```

## PostgreSQL adapter

```go
var _ ports.OrderRepository = (*OrderRepository)(nil)

type OrderRepository struct {
    queries *dbgen.Queries
}

func (r *OrderRepository) GetByID(ctx context.Context, id int64) (domain.Order, error) {
    order, err := r.queries.GetOrder(ctx, id)
    if errors.Is(err, pgx.ErrNoRows) {
        return domain.Order{}, domain.ErrOrderNotFound
    }
    if err != nil {
        return domain.Order{}, fmt.Errorf("select order %d: %w", id, err)
    }
    return toDomain(order), nil
}
```

O objetivo não é esconder Go atrás do framework. O projeto gerado continua sendo Go comum e não depende do runtime do `gosvc`.
