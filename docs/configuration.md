# Configuración

CaimanDB se configura por dos vías, que se combinan en este orden:

1. **Archivo `configs/caimandb.conf`** (JSON), relativo al directorio
   de trabajo. Si no existe, se crea automáticamente al arrancar con
   los valores por defecto, creando también la carpeta `configs/` si
   hace falta (ver `internal/caimandb/app.go` y
   `internal/caimandb/defaults.go`). Una plantilla comentada está en
   [`configs/caimandb.conf.example`](../configs/caimandb.conf.example)
   — cópiala como `configs/caimandb.conf` para personalizarla.
2. **Variables de entorno**, que sobrescriben lo que haya en el
   archivo (`internal/caimandb/defaults.go`).

## Variables de entorno

| Variable | Descripción | Por defecto |
|---|---|---|
| `CAIMANDB_DATA` | Directorio raíz de datos | `./data` |
| `CAIMANDB_CACHE` | Caché L1 (MB) | `2048` |
| `CAIMANDB_L2_CACHE` | Entradas de índice en caché L2 | `4096` |
| `CAIMANDB_WORKERS` | Goroutines de trabajo | auto |
| `CAIMANDB_LOG_LEVEL` | `debug`\|`info`\|`warn`\|`error` | `info` |
| `CAIMANDB_METRICS` | Puerto de métricas | `9090` |
| `CAIMANDB_NODE_ID` | Identificador de nodo | auto |
| `CAIMANDB_RAFT_PORT` | Puerto Raft | `2335` |
| `CAIMANDB_RAFT_BIND` | Dirección de bind de Raft | `0.0.0.0` |
| `CAIMANDB_QUERY_PORT` | Puerto del servidor de consultas | `1555` |
| `CAIMANDB_ADMIN_PORT` | Puerto de la API de administración | `1556` |
| `CAIMANDB_SHARD_COUNT` | Número de shards | `16` |
| `CAIMANDB_REPLICA_COUNT` | Factor de replicación | `3` |
| `CAIMANDB_QUERY_USER` / `CAIMANDB_QUERY_PASSWORD` | Credenciales | — |
| `CAIMANDB_FAST_STARTUP` | Arranque rápido (sin replay de WAL) | `true` |
| `CAIMANDB_COMPRESS_LARGE` | Comprimir documentos grandes | `true` |
| `CAIMANDB_MAX_DOC_SIZE` | Tamaño máx. de documento (bytes) | `10485760` |
| `CAIMANDB_JWT_SECRET` | Secreto JWT | autogenerado |
| `CAIMANDB_TLS_ENABLED` | Activar TLS | `false` |
| `CAIMANDB_TLS_CERT_FILE` | Certificado TLS | — |
| `CAIMANDB_STORAGE_AI_ENABLED` | Activar sizing adaptativo de Badger por bloque (Storage AI) | `true` |
| `CAIMANDB_STORAGE_AI_RAM_FRACTION` | Fracción (0,1] de la RAM detectada que Storage AI puede reservar en conjunto | `0.5` |
| `CAIMANDB_STORAGE_AI_MAX_BUDGET_MB` | Tope absoluto en MB (anula la fracción; útil en contenedores) | `0` (sin tope explícito) |

## Bloques de configuración principales

- **Core**: `data_root`, `cache_mb`, `l2_cache_mb`, `workers`.
- **Red**: `raft_port`, `raft_bind`, `query_port`, `admin_port`.
- **Sharding**: `shard_count`, `replica_count`, `max_shards_per_node`,
  `auto_merge_enabled`, `auto_split_enabled`, `predictive_scaling`.
- **Seguridad**: `jwt_secret`, `token_expiry`, `max_login_attempts`,
  `lockout_duration`, `tls_enabled`.
- **FLEX-COLUMN** (motor columnar, bloque `flex_column`): caché
  columnar, detección de campos calientes, vistas materializadas,
  compresión (`zstd` por defecto).

Ver la estructura completa en
`internal/caimandb/config.go` (struct `Config`) y
`internal/caimandb/defaults.go` (valores por defecto + overrides por
entorno).
