# Troubleshooting

## `unknown revision .../v1.1.0`

A tag ainda não existe no GitHub. Use `@main` antes da publicação ou publique a tag antes de instalar por versão.

## `gosvc` funciona no checkout, mas depois do `cd` aponta para outra versão ou não existe

Há dois modos distintos. Se a CLI foi instalada no `PATH`, use `gosvc ...` em qualquer diretório. Se você apenas compilou o source checkout com `make build`, use `./bin/gosvc ...` enquanto estiver em `gosvc/` e, após entrar no projeto irmão, use `../gosvc/bin/gosvc ...`. O shell não troca automaticamente `gosvc` pelo binário local.

## Projeto criado dentro de `gosvc`

Versões atuais criam o projeto como diretório irmão quando `gosvc new` é executado dentro do checkout. Use exatamente o `Next: cd ...` impresso pela CLI. Remova diretórios antigos aninhados se eles puderem causar confusão.

## Linter mostra caminho antigo

Projetos atuais isolam o cache em `.cache/golangci-lint` e invalidam o cache quando a raiz muda. Em projetos antigos, limpe uma vez com `./bin/golangci-lint cache clean` ou regenere/atualize o Makefile.

## Primeiro `make verify` altera `go.mod/go.sum`

Use `make bootstrap` primeiro. Localmente, `make verify` pode reconciliar e informar drift; CI usa `make verify-strict`/`STRICT_GIT_DRIFT=1` para exigir zero drift rastreado.

## `govulncheck` acusa standard library Go 1.25 antiga

Projetos com banco declaram `runtime.go.toolchain: go1.25.12`. Garanta `GOTOOLCHAIN=auto` ou instale um Go igual/mais novo. Não ignore o scanner.

## Porta 8080 ocupada

```bash
lsof -iTCP:8080 -sTCP:LISTEN
```

Finalize o processo anterior ou altere `HTTP_PORT`.

## `BLOCKED` na certificação

`BLOCKED` significa que o gate real não conseguiu executar por ausência de pré-requisito/infraestrutura. Não equivale a PASS nem a FAIL funcional.

## Certification tries a generator that the project does not use

v17 certification is capability-driven. `bare`/`worker` record OpenAPI/sqlc as `SKIPPED (not applicable)`, and GORM projects skip `sqlc-real`. If an older binary still attempts these checks, rebuild/update gosvc before re-running certification.

## `project.yaml` schema 1 vs manifest schema 4

They are different contracts. `project.yaml` is user configuration; `.gosvc/manifest.json` is internal generator state. See `docs/CONTRACTS.md`.
