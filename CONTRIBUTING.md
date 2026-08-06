# Contribuir a CaimanDB

## Antes de nada

Este repositorio se preparó/reorganizó en un entorno sin toolchain de
Go ni acceso a red, así que **nada aquí se compiló realmente**. Antes
de tu primer cambio:

```bash
go build ./...
go vet ./...
```

y reporta cualquier error — es lo primero que hay que arreglar.

## Estructura

Ver [`docs/architecture.md`](docs/architecture.md) para el mapa
completo del código. Resumen rápido:

- `cmd/caimandb/` — punto de entrada, no debería crecer.
- `internal/caimandb/` — todo el motor (un solo paquete Go, ver
  [`docs/known-limitations.md`](docs/known-limitations.md) para el
  porqué). Agrupa tu archivo nuevo con el prefijo que le corresponda
  (`ops_`, `cmd_`, `http_`, etc.) en vez de crear una carpeta nueva.
- `docs/` — documentación.
- `deployments/` — Docker/compose.
- `scripts/` — build/test/run.
- `test/` — integración y fixtures.

## Estilo

- `gofmt -l .` no debería devolver nada antes de un PR.
- Sigue la convención de prefijos de archivo ya existente en
  `internal/caimandb/`.
- Los comandos NQL nuevos van en `cmd_*.go` + su entrada en
  `dsl_parser.go` + documentación en `docs/nql-reference.md` y en el
  texto de `HELP` (`help.go`).

## Pull requests

1. Rama descriptiva (`fix/...`, `feat/...`).
2. `go build ./... && go vet ./... && go test ./...` en verde.
3. Actualiza la documentación afectada (`docs/`, `README.md`,
   `CHANGELOG.md`).
