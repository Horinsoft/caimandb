# Hoja de ruta: motor por bloques — estado actual y priorización

Este documento responde a la lista de características pedida, comparándola
con lo que **ya existe** en este repo (`caimandb_pro_v2`) y proponiendo un
orden de trabajo realista para el resto. Es deliberadamente honesto sobre
el alcance: la lista original describe, en conjunto, un motor distribuido
de nivel "producción" comparable a proyectos como CockroachDB o
FoundationDB en ambición. Eso no se construye en una sola sesión, ni de
forma segura sin un compilador Go a mano para verificar cada cambio —
este entorno no tuvo red ni toolchain disponibles. Lo que sigue es un plan
de trabajo, no una implementación completa.

## 0. Lo que ya está resuelto en este pase

Ver `docs/corruption-fixes-2026-07.md`: se corrigieron 4 bugs reales de
pérdida/corrupción de datos en `wal.go` y `block_repair.go` (marcador
`.clean` obsoleto, confirmación de escritura antes de persistir, y
`RepairBlock` que no reparaba nada). Es la parte de "que los datos no se
corrompan" que se pidió explícitamente, y es la que más urgía porque
afecta la garantía básica de durabilidad del motor tal como está hoy.

## 1. Estado por bloque: independencia de BadgerDB, WAL, caché, índices, stats, compresión

| Recurso | Estado actual | Nota |
|---|---|---|
| BadgerDB | **Ya es independiente por bloque** (`pool.openBlock(db, block)` abre/gestiona una instancia BadgerDB propia por bloque) | — |
| Índices secundarios | **Ya es independiente por bloque** (`__index/` por bloque) | — |
| Compresión | Config global (`CompressionZstd` etc.), aplicada por documento/entrada, no hay política *distinta* por bloque | Falta: permitir que cada bloque elija su algoritmo/nivel |
| Caché | Global de dos niveles (L1 documentos, L2 índices), no por bloque | Falta: caché con clave que incluya bloque + límites de memoria por bloque |
| WAL | **Global, un solo WAL para todo el engine** (`engine.wal`, en `__system/wal`) | Es el ítem más grande pendiente de esta lista |
| Estadísticas | Métricas globales (Prometheus nativo vía `metrics.go`), no desglosadas por bloque | Falta: labels por bloque en las métricas existentes |

### Por qué el WAL por bloque es el trabajo más grande, y cómo abordarlo

Pasar de "un WAL global" a "un WAL por bloque" no es solo instanciar N
WALs — implica decidir:

1. **Orden de recovery entre bloques.** Hoy el recovery es una sola
   secuencia global; con WAL por bloque, cada bloque se recupera de forma
   independiente y en paralelo, pero las operaciones que tocan varios
   bloques a la vez (transacciones multi-bloque, joins, `relate`) deben
   seguir siendo atómicas — normalmente se resuelve con un WAL de
   coordinación (2PC-like) por encima de los WALs de bloque, no
   eliminando la coordinación central del todo.
2. **Costo de recursos.** Cada WAL activo hoy mantiene un goroutine
   (`writeLoop`), un `*os.File` abierto, un `bufio.Writer` y un
   `BufferPool` de hasta 128MB. Multiplicado por miles de bloques, eso es
   inviable tal cual — hace falta un pool de WALs con activación
   perezosa (lazy) para bloques fríos, similar a lo que ya existe para
   BadgerDB (`dbPool`).
3. **Migración de datos existentes.** Los despliegues que ya tengan un
   WAL global necesitan una ruta de migración (drenar el WAL global,
   luego empezar a escribir en WALs por bloque) para no perder el
   historial no aplicado durante el cambio.

Orden sugerido: (a) extraer `wal.go` en algo instanciable por bloque sin
romper la API actual de `engine.wal.Write(...)`, (b) añadir un
`walPool` con lazy-open igual que `dbPool`, (c) resolver la
coordinación multi-bloque, (d) migración. Cada paso necesita compilar y
correr las pruebas de `test/` antes de pasar al siguiente.

## 2–12: mapeo del resto de la lista

| Sección pedida | Estado | Prioridad sugerida |
|---|---|---|
| Particionado (Nodos→Shards→Blocks), split/merge, balanceo | Ya existe: `shard_manager.go` (hashing consistente, auto-split/merge, auto-scaling predictivo) | Madurar con benchmarks reales antes de tocar |
| Coordinador distribuido | Ya existe: Raft (`cluster.go`, `raft_fsm.go`, `dist_query.go`) | — |
| Compresión adaptativa Zstd/LZ4, dictionary/delta encoding, dedupe | Zstd ya integrado (`compression.go`); LZ4, dictionary encoding, delta encoding y deduplicación de documentos **no existen** | Media — dictionary encoding da el mayor ahorro para JSON repetitivo |
| `sync.Pool` para buffers | Ya existe (`buffer_pool.go`) | — |
| Arena allocators, zero-copy, mmap | No existen; Go no tiene arenas nativas fuera del paquete experimental `arena` (retirado) — normalmente se logra con `sync.Pool` + reducir allocs, no con arenas reales | Baja/investigación |
| Caché multinivel (Block→Shard→Nodo), lazy loading | Caché L1/L2 ya existe pero no tiene esa jerarquía exacta; `LazyIndexLoading` ya existe como config | Media |
| SIMD (AVX2/SSE) | No existe; en Go requiere cgo o assembly, alto riesgo/mantenimiento | Baja — medir primero si el cuello de botella real lo justifica |
| Thread pools, paralelización por bloque | `worker_pool.go` ya existe como base | Extender por bloque |
| Lock striping | `locks.go` ya existe (`lockManager`) — verificar si ya sigue granularidad por bloque o es más grueso | Revisar antes de rediseñar |
| ART, Bloom Filters, Roaring Bitmaps | Índices secundarios existen (`index_secondary.go`) pero no como ART/Bloom/Roaring específicamente | Alta si el objetivo es rendimiento de lectura — impacto medible y acotado |
| Learned indexes | No existe; es investigación activa en la industria, no una feature estándar | Investigación, no roadmap de producto |
| Compactación, snapshots incrementales | `ops_compact.go`, `ops_backup.go` existen; snapshots incrementales específicamente no confirmados | Revisar `ops_backup.go` antes de asumir que falta |
| Protocolo binario (FlatBuffers/Cap'n Proto) | Hoy es JSON (`encoding/json`) en WAL y HTTP | Alto impacto en throughput pero es un cambio de formato de datos que rompe compatibilidad — requiere migración versionada |
| TLS, cifrado en reposo, roles/permisos, auditoría | JWT + Argon2id + rate limiting + risk engine + auditoría **ya existen** (`auth_jwt.go`, `risk_engine.go`, `audit.go`); cifrado en reposo no confirmado | Verificar si BadgerDB está corriendo con `WithEncryptionKey` |
| Observabilidad (Prometheus, dashboard, estado por bloque) | Métricas Prometheus nativas ya integradas (`metrics.go`), sin dependencias externas (VictoriaMetrics eliminado) | Falta granularidad por bloque, no la base |
| Benchmarks | `test/` existe como plantilla pero **sin suite real** (confirmado en `docs/known-limitations.md`) | Alta — sin esto no se puede validar ninguna optimización de esta lista con datos reales |

## Orden de trabajo recomendado

1. **Benchmarks reales primero.** Sin una suite de benchmarks reproducible
   (inserciones/seg, consultas/seg, latencia p50/p99, uso de CPU/RAM/disco,
   tiempo de recuperación), cualquier optimización de las secciones 2–9 es
   un cambio a ciegas — no hay forma de confirmar que ayuda ni de detectar
   una regresión.
2. **WAL por bloque** (sección 1), porque es la pieza de arquitectura que
   más se pidió explícitamente y de la que más depende el resto (caché e
   índices por bloque son más simples una vez el bloque es una unidad
   de vida propia).
3. **ART / Bloom / Roaring Bitmaps** para índices — impacto de lectura
   medible, alcance acotado, no requiere cambiar el formato de
   almacenamiento existente.
4. **Dictionary encoding** para JSON — buen ratio esfuerzo/beneficio en
   compresión antes de tocar SIMD o arenas.
5. Todo lo marcado "Baja / investigación" (SIMD, arena allocators,
   learned indexes) al final, y solo si los benchmarks del paso 1
   muestran que son realmente el cuello de botella.

Cada paso de esta lista debería terminar con `go build ./... && go vet
./...` y la suite de benchmarks del paso 1 antes de pasar al siguiente —
igual que recomienda `docs/known-limitations.md` para la separación de
paquetes.
