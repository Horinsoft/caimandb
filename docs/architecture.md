# Arquitectura de CaimanDB

CaimanDB es un motor de datos orientado a documentos con sharding,
clustering basado en Raft, un Write-Ahead Log (WAL), transacciones
ACID y un lenguaje de comandos tipo SQL (NQL).

## Diseño del binario

```
cmd/caimandb/main.go   → punto de entrada (func main), delega todo a internal/caimandb.Run()
internal/caimandb/      → toda la lógica del motor, servidor y CLI (un solo paquete Go)
```

`internal/caimandb` se mantiene como **un único paquete** a propósito.
El tipo central `Engine` (definido en `engine_main.go`) es referenciado
directamente —incluyendo campos no exportados como `pool`, `wal`,
`l1Cache`, `shardMgr`, `lockManager`, etc.— desde más de 40 archivos
(`ops_*.go`, `cmd_*.go`, `raft_fsm.go`, `transaction.go`...). Separar
eso en varios paquetes de Go exigiría exportar buena parte del estado
interno del motor, un cambio mecánico pero de alto riesgo que en un
entorno sin compilador (como el usado para preparar este repo) no se
puede verificar con seguridad. Ver [`known-limitations.md`](./known-limitations.md).

En cambio, el código dentro de `internal/caimandb` ya está agrupado por
prefijo de archivo, que es la convención habitual en proyectos Go
grandes de un solo paquete:

| Prefijo / archivo | Responsabilidad |
|---|---|
| `engine_core.go`, `engine_main.go`, `app.go` | Núcleo del motor, ciclo de vida, arranque (`Run()`) |
| `ops_*.go` | Operaciones de bajo nivel sobre el `Engine` (CRUD, backup, compact, búsqueda, agregación...) |
| `cmd_*.go` | Manejadores de comandos NQL (INSERT, FIND, CREATE, VIEW, DROP, RENAME, TX...) |
| `dsl_parser.go`, `tokenizer.go`, `query_filter.go`, `query_optimizer.go` | Parsing y optimización de consultas NQL |
| `badger_pool.go`, `wal.go`, `buffer_pool.go`, `compression.go`, `storage_external.go`, `directory.go`, `keys.go` | Almacenamiento persistente (BadgerDB, WAL, buffers, compresión) |
| `cache.go` | Caché L1 (documentos) y L2 (índices) |
| `cluster.go`, `shard_manager.go`, `raft_fsm.go`, `raft_logstore.go`, `dist_query.go` | Clustering Raft, sharding y consultas distribuidas |
| `transaction.go`, `locks.go` | Transacciones ACID y locking |
| `flexcolumn.go`, `nested_fields.go`, `index_secondary.go` | Motor columnar (FLEX-COLUMN) e índices secundarios |
| `http_admin.go`, `http_query.go` | Servidores HTTP (API de administración y de consultas) |
| `auth_jwt.go`, `users_auth.go`, `ratelimit.go`, `risk_engine.go`, `audit.go` | Autenticación, rate limiting, motor de riesgo, auditoría |
| `metrics.go`, `logging.go` | Observabilidad (métricas Prometheus nativas, sin dependencias externas) |
| `config.go`, `defaults.go`, `constants.go` | Configuración |
| `document.go`, `doc.go` | Modelo de documento |
| `block_repair.go`, `worker_pool.go`, `misc_utils.go`, `sort_utils.go`, `help.go` | Utilidades varias |

## Flujo de arranque

`cmd/caimandb/main.go` → `caimandb.Run()` (`app.go`):

1. Carga `configs/caimandb.conf` si existe (o crea uno ahí, creando la
   carpeta si hace falta, con los valores por defecto — ver
   `configs/caimandb.conf.example`).
2. Inicializa el `Engine`: pool de BadgerDB, WAL, cachés L1/L2,
   buffer pool, gestor de directorios, rate limiter, auditoría.
3. Si `auto_cluster` está activo, arranca Raft y el descubrimiento de
   nodos (`cluster.go`).
4. Levanta los servidores HTTP de consultas y administración
   (`http_query.go`, `http_admin.go`) en los puertos configurados.
5. Entra en el REPL interactivo para comandos NQL.

## Almacenamiento en disco

```
./data/
├── <db_name>/                 <- Directorio de base de datos
│   ├── __meta/                <- Metadatos (db.json)
│   ├── <block_name>/          <- Directorio de bloque (equivalente a "colección")
│   │   ├── __data/            <- Datos BadgerDB (documentos)
│   │   ├── __index/            <- Índices secundarios
│   │   └── __meta/            <- Metadatos del bloque
│   └── ...
├── __users/                    <- Autenticación (Argon2id)
├── __raft/                     <- Estado del clúster Raft
├── __shards/                   <- Datos de gestión de shards
├── __cluster/                  <- Información de nodos del clúster
├── __system/wal/                <- Write-Ahead Log
└── __external/                  <- Documentos grandes (>5MB) fuera de BadgerDB
```

## Subsistemas destacados

- **Transacciones ACID**: `BEGIN` / `COMMIT` / `ROLLBACK` con niveles
  de aislamiento `read_committed`, `repeatable_read` (por defecto) y
  `serializable` (`transaction.go`).
- **Sharding y clustering**: hashing consistente, auto-splitting/
  merging, auto-scaling predictivo y ejecución de consultas
  distribuidas con "shard pruning" (`shard_manager.go`, `dist_query.go`).
- **FLEX-COLUMN**: motor columnar opcional con detección automática
  de campos "calientes" y vistas materializadas (`flexcolumn.go`).
- **Seguridad**: hashing Argon2id, JWT, rate limiting, bloqueo de
  cuentas, auditoría en JSON y un motor de riesgo que puede bloquear
  operaciones sospechosas (`risk_engine.go`).

Para el detalle completo de comandos NQL ver
[`docs/nql-reference.md`](./nql-reference.md); para los endpoints
HTTP ver [`docs/api/http-api.md`](./api/http-api.md).
