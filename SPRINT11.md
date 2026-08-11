# Sprint 11 — Publicação Real e Package Managers

## Objetivo

Fechar o fluxo entre a release candidate produzida pela Sprint 10 e uma publicação real em um repositório GitHub definitivo, sem deixar module paths, imports, installers ou metadados dependentes de substituições manuais frágeis.

## Entregas

### `gosvc release prepare`

Novo comando para migrar a identidade do repositório:

```bash
gosvc release prepare \
  --project . \
  --repository ailtonmacedo/gosvc \
  --dry-run

gosvc release prepare \
  --project . \
  --repository ailtonmacedo/gosvc
```

O comando:

- valida `owner/name`;
- lê o module path atual em `go.mod`;
- atualiza o module path e imports internos;
- atualiza Makefile, workflows e documentação;
- substitui links legados `ailtonmacedo/gosvc`;
- ignora `.git`, `dist`, `bin`, `vendor` e diretórios de IDE;
- prepara todos os arquivos temporários antes da troca;
- mantém backups durante a aplicação e executa rollback em falha;
- suporta dry-run;
- é idempotente.

Uma migração real do código completo foi executada para:

```text
github.com/ailtonmacedo/gosvc → github.com/acme/gosvc
```

Resultado:

- 32 arquivos atualizados;
- `go test ./...` aprovado no módulo migrado;
- release preflight aprovado;
- segunda execução sem alterações.

### Identidade de repositório no preflight

`gosvc release check` agora valida em conjunto:

- versão semântica;
- module path;
- repositório informado;
- remote `origin`, quando disponível;
- documentação e arquivos obrigatórios;
- completions.

### Installers renderizados

Os installers da release recebem o repositório definitivo durante o snapshot. O uso normal não exige mais definir `GOSVC_REPOSITORY`:

```bash
./install.sh 1.0.0
```

```powershell
.\install.ps1 1.0.0
```

Foi adicionado suporte a mirrors locais:

```bash
GOSVC_RELEASE_BASE_URL=http://127.0.0.1:18080 \
GOSVC_INSTALL_DIR="$PWD/tmp-bin" \
./install.sh 1.0.0
```

### Homebrew

Cada snapshot passa a produzir:

```text
gosvc.rb
```

A fórmula contém URLs e hashes reais para:

- macOS amd64;
- macOS arm64;
- Linux amd64;
- Linux arm64.

Também instala completions para Bash, Zsh e Fish.

### Scoop

Cada snapshot passa a produzir:

```text
gosvc.json
```

O manifesto contém artefatos e hashes reais para:

- Windows amd64;
- Windows arm64.

### `gosvc release verify`

Novo comando:

```bash
gosvc release verify --dist dist
```

Valida offline:

- schema e identidade do release manifest;
- tamanho e SHA-256 de cada asset;
- conteúdo do `checksums.txt`;
- estrutura dos seis archives;
- presença do binário esperado;
- repositório incorporado nos installers;
- ausência de placeholders;
- Homebrew formula e hashes;
- Scoop manifest;
- execução do binário correspondente ao host.

Para ambientes de cross-validation:

```bash
gosvc release verify --dist dist --skip-exec
```

### Manifest schema v2 da release

`release-manifest.json` agora registra também:

```json
{
  "schema_version": 2,
  "module": "github.com/ailtonmacedo/gosvc",
  "repository": "ailtonmacedo/gosvc"
}
```

### Workflow de release

O workflow agora:

1. executa quality gates;
2. executa o preflight;
3. gera os assets determinísticos;
4. executa `gosvc release verify`;
5. inicia mirror HTTP local;
6. instala o binário pelo `install.sh` renderizado;
7. executa o binário instalado;
8. gera provenance e attestation do SBOM;
9. publica a GitHub Release.

## Reprodutibilidade

Dois snapshots foram produzidos com:

```bash
SOURCE_DATE_EPOCH=1700000000
```

Todos os hashes permaneceram idênticos, incluindo:

- seis archives;
- installers;
- completions;
- Homebrew formula;
- Scoop manifest;
- SBOM;
- release manifest;
- checksums.

## Smoke test do installer

O `install.sh` foi testado contra um servidor HTTP local:

```text
gosvc 1.0.0 installed at /tmp/gosvc-install-smoke/gosvc
gosvc version 1.0.0
commit: unknown
built: 2023-11-14T22:13:20Z
```

## Validações executadas

```bash
make verify
go test -race ./...
sh -n scripts/install.sh
sh -n dist/install.sh
ruby -c dist/gosvc.rb
gosvc release check --version 1.0.0 --repository acme/gosvc --allow-placeholder
gosvc release verify --dist dist
```

Também foram parseados:

- workflows YAML;
- Dependabot YAML;
- Scoop JSON;
- release manifest JSON;
- SBOM SPDX JSON.

## Limitações do ambiente

- Nenhuma publicação foi feita no GitHub porque o repositório definitivo não foi informado.
- Homebrew e Scoop não estavam disponíveis para instalação real dos manifests.
- `pwsh` não estava disponível; o installer PowerShell foi gerado e inspecionado, mas não executado.
- O installer POSIX, os archives e o binário Linux foram executados de ponta a ponta.

## Estado final

A base está pronta para substituir o placeholder por um repositório real e publicar a tag `v1.0.0` sem edições manuais dispersas.
