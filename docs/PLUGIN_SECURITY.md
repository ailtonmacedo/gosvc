# Segurança de plugins

`gosvc` possui dois modos de execução para plugins externos.

## 1. Native — confiável

Plugins nativos continuam compatíveis com o protocolo existente, mas executam com permissões do usuário. O framework valida checksum, entrypoint, timeout, tamanho de saída e usa um snapshot temporário do projeto; isso **não é uma sandbox de sistema operacional**.

Riscos residuais incluem acesso a filesystem fora do snapshot, rede, sockets locais, credenciais do usuário e outros recursos permitidos pelo SO.

## 2. Docker — isolado

Plugins `schema_version: 3` podem declarar `execution.mode: docker`. A imagem deve ser fixada por digest SHA-256:

```json
{
  "schema_version": 3,
  "protocol_version": 1,
  "name": "audit",
  "version": "1.0.0",
  "description": "Sandboxed audit plugin",
  "minimum_gosvc_version": "1.1.0",
  "capabilities": ["validation", "artifacts"],
  "execution": {
    "mode": "docker",
    "docker": {
      "image": "ghcr.io/acme/gosvc-audit@sha256:<64-hex>",
      "command": ["/plugin"],
      "network": false
    }
  }
}
```

O runtime aplica por padrão:

- filesystem do container read-only;
- projeto montado read-only em `/workspace/project`;
- `--cap-drop ALL`;
- `no-new-privileges`;
- usuário não-root `65532:65532`;
- limites de CPU, memória e PIDs;
- `/tmp` temporário com `noexec,nosuid`;
- rede `none` por padrão;
- somente o protocolo JSON via stdin/stdout pode contribuir artefatos.

## Política de execução

Para exigir sandbox Docker e rejeitar plugins nativos:

```bash
gosvc plugins run audit --project . --require-sandbox --dry-run
```

Se o manifesto pedir rede, a execução falha até que o operador aprove explicitamente:

```bash
gosvc plugins run audit --project . --require-sandbox --allow-network
```

`--allow-network` é uma concessão de confiança. O default continua sem rede.

## Limites do isolamento

Docker reduz significativamente a superfície do plugin, mas não transforma código não confiável em risco zero. A segurança também depende do daemon/runtime Docker, kernel, imagem utilizada e configuração do host. Use imagens mínimas, assinadas/provenientes quando possível, e sempre fixe digest imutável.
