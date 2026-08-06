# CaimanDB

Motor de datos orientado a documentos con sharding, clustering basado
en Raft, WAL, transacciones ACID y un lenguaje de comandos tipo SQL
(NQL).

## Estructura del repositorio

```
caimandb/
├── cmd/
│   └── caimandb/              # punto de entrada (func main), delega todo a internal/caimandb.Run()
├── internal/
│   └── caimandb/               # motor, servidor y CLI (paquete Go único — ver docs/known-limitations.md)
│       └── parse/            # tokenizer de la sintaxis NQL (único subpaquete separado hoy)
├── configs/
│   ├── caimandb.conf.example   # plantilla de configuración (JSON)
│   └── caimandb.conf           # config activa; se crea aquí sola en el primer arranque
├── deployments/
│   └── docker/
│       ├── Dockerfile
│       └── docker-compose.yml
├── docs/
│   ├── architecture.md       # mapa completo del código y del almacenamiento en disco
│   ├── configuration.md      # archivo de config + variables de entorno
│   ├── nql-reference.md      # referencia de comandos NQL
│   ├── known-limitations.md  # por qué internal/caimandb sigue siendo un solo paquete
│   └── api/
│       └── http-api.md       # endpoints de los dos servidores HTTP
├── examples/
│   └── quickstart.md
├── scripts/
│   ├── build.sh
│   ├── test.sh
│   └── run-dev.sh
├── test/
│   ├── integration/
│   ├── fixtures/
│   └── README.md
├── .github/workflows/ci.yml
├── CHANGELOG.md
├── CONTRIBUTING.md
├── LICENSE
├── Makefile
└── go.mod
```

Ver [`docs/architecture.md`](docs/architecture.md) para el detalle de
qué hace cada archivo dentro de `internal/caimandb`.

## Compilar y ejecutar

```bash
go build ./cmd/caimandb
./caimandb
```

o con las utilidades incluidas:

```bash
make build   # ./scripts/build.sh -> bin/caimandb
make run     # ./scripts/run-dev.sh (modo desarrollo, ./data-dev)
make test    # go build + go vet + go test
make docker  # build de la imagen Docker
```

Guía paso a paso: [`examples/quickstart.md`](examples/quickstart.md).

## Configuración

`configs/caimandb.conf` (JSON), relativo al directorio de trabajo, con
overrides por variable de entorno (`CAIMANDB_*`). Si no existe, CaimanDB
lo crea solo (y crea la carpeta `configs/` si hace falta) con los
valores por defecto. Detalle completo en
[`docs/configuration.md`](docs/configuration.md); plantilla en
[`configs/caimandb.conf.example`](configs/caimandb.conf.example).

## Por qué `internal/caimandb` no se dividió (casi) en más paquetes Go

Es la carpeta con más archivos (69) y la razón por la que a primera
vista el repo puede parecer "plano". Se investigó en profundidad
partirla en subpaquetes (`internal/engine`, `internal/storage`,
`internal/cluster`...), pero el análisis estático mostró un
acoplamiento real y profundo: más de 40 archivos acceden directamente
a campos no exportados del `Engine` central, y varios subsistemas
(cluster, HTTP, transacciones) guardan una referencia de vuelta al
propio `Engine`, lo que generaría ciclos de imports si se separan sin
más. Hacerlo bien exige exportar buena parte del estado interno y
verificar cada punto de uso con un compilador — algo que no fue
posible confirmar en este entorno (sin Go instalado ni red).

La única excepción es `internal/caimandb/parse`: el tokenizer de la
sintaxis NQL (`tokenize`, ahora `parse.Tokenize`) no tocaba ningún
tipo del motor (`Engine`, `Document`, `Config`, `Session`, `Filter`,
`Transaction`), así que se pudo mover de verdad sin arriesgar nada.
Es exactamente el tipo de archivo "hoja" que describe la ruta
recomendada en `docs/known-limitations.md` para seguir avanzando.

Detalle completo, con la lista exacta de qué se investigó y una ruta
recomendada para hacerlo con un compilador a mano:
[`docs/known-limitations.md`](docs/known-limitations.md).

Todo lo demás en este repositorio (la reorganización de carpetas,
`docs/`, `deployments/`, `scripts/`, `configs/`, `examples/`, `test/`,
CI) no toca ni un archivo `.go` de `internal/caimandb`, así que no
introduce ningún riesgo nuevo sobre el código que ya compilaba.

## Historial de cambios

Ver [`CHANGELOG.md`](CHANGELOG.md) — incluye tanto este pase de
reorganización como el pase anterior que arregló los errores de
compilación originales (declaraciones duplicadas, handlers NQL
faltantes, imports sin usar).

## Verificación pendiente

**Este entorno no tuvo acceso a un compilador de Go ni a red.**
Antes de confiar en este código, en una máquina con Go 1.22+:

```bash
go build ./...
go vet ./...
go mod tidy   # y confirma que go.sum queda consistente
```

Si algo falla, es el primer sitio por dónde seguir.
