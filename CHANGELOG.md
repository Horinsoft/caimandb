# Changelog

## [Unreleased] — WAL: evita doble aplicación de entradas tras un segundo crash

- **Problema corregido:** después de recuperar el WAL en el arranque
  (`RecoverWAL`), el motor solo podaba los segmentos viejos
  (`PruneToLastSegment`) pero dejaba el segmento *activo* —el mismo que
  acababa de leer y aplicar— como destino de las escrituras nuevas. Si
  el proceso volvía a caer sin un `Close()` limpio de por medio, el
  siguiente arranque releía ese segmento y reaplicaba las mismas
  entradas una segunda vez; inofensivo para un `insert` (idempotente
  por ID) pero corrompe un `update` no idempotente (`$inc`, etc).
- **Fix:** nuevo `WAL.RotateAndPruneFresh()`
  (`internal/caimandb/wal/wal.go`), usado por `RecoverWAL`
  (`internal/caimandb/wal_recovery.go`) en vez de
  `PruneToLastSegment()` directo: rota a un segmento nuevo y vacío
  antes de podar, así ninguna entrada ya aplicada queda en disco para
  ser releída. Ver `docs/corruption-fixes-2026-07.md` (sección 6) para
  el detalle completo.
- **No se tocó:** el resto del pipeline de arranque/interrupción/limpieza
  ya estaba bien cubierto de antes — marcador `.clean` invalidado al
  arrancar (punto 1 del doc de julio), *graceful shutdown* con
  `SIGINT`/`SIGTERM`/`SIGHUP` y `http.Server.Shutdown` con timeout,
  *worker pool* adaptativo que auto-termina goroutines ociosas
  (`internal/caimandb/turbo/pool.go`), y un `tx_cleanup_loop` dedicado
  que aborta y purga transacciones abandonadas/expiradas
  (`internal/caimandb/transaction.go`). Este cambio revisó todo ese
  camino sin tocarlo porque ya funcionaba correctamente; el único bug
  real encontrado fue el de arriba.

## [Unreleased] — Storage AI: sizing adaptativo de BadgerDB por bloque

- **Problema corregido:** todo bloque (nuevo o no) se abría con las
  mismas opciones fijas de BadgerDB (`storage/constants.go`), así que un
  bloque casi vacío ocupaba en disco/RAM lo mismo que uno con cientos de
  miles de documentos (~49MB reportado). Con muchos bloques pequeños eso
  se vuelve un problema real de espacio y memoria.
- **`internal/caimandb/storage/adaptive.go` (nuevo):** 4 tiers de
  configuración de Badger (`micro`/`small`/`standard`/`large`). El tier
  de un bloque se elige, la primera vez que se abre en el proceso, según
  lo que ya hay en disco para ese bloque (vacío → `micro`) y se ajusta
  contra un presupuesto de RAM compartido entre todos los bloques
  abiertos a la vez (por defecto 50% de la RAM detectada vía
  `/proc/meminfo`, configurable).
- **`storage/badger_pool.go`:** `DBPool` ahora hace esta elección al
  abrir cada bloque (`OpenDataPath`/`OpenBlock`, sin cambiar su firma
  pública, así que ninguno de los ~20 call sites existentes se tocó) y
  libera el presupuesto correspondiente al cerrar. Nuevo
  `DBPool.AdaptiveStats()` para observabilidad (conteo de bloques por
  tier, presupuesto usado/total).
- **Config nueva:** `storage_ai_enabled` (default `true`),
  `storage_ai_ram_fraction` (default `0.5`), `storage_ai_max_budget_mb`
  (default `0` = sin tope explícito), con sus variables de entorno
  `CAIMANDB_STORAGE_AI_*`. `storage_ai_enabled=false` reproduce el
  comportamiento fijo original exacto.
- **Qué NO hace todavía, a propósito:** no reajusta en caliente un
  bloque que ya está sirviendo tráfico (solo se clasifica al abrir). Ver
  [`docs/storage-ai-adaptive-sizing.md`](docs/storage-ai-adaptive-sizing.md)
  para el análisis completo de por qué (lecturas en `ops_find.go` no
  toman el lock de escritura, así que cerrar/reabrir el handle de Badger
  bajo tráfico de lectura concurrente no es seguro sin más) y la ruta
  recomendada para implementarlo con un compilador a mano.

## [Unreleased] — `SHOW AUTORELATIONS`: `WHERE` y `ORDER BY` con dirección

- **`WHERE <expresión>`** (`cmd_show_autorelations.go`): sinónimo de
  `FILTER` con la misma sintaxis de condición que usan `FIND`/`SEARCH`/
  `VIEW`, sobre `doc_id`, `user_id`, `access_count`, `relevance`,
  `last_seen`, `first_seen`. `FILTER` se mantiene por compatibilidad.
- **`ORDER BY ... ASC|DESC`**: `ORDER BY` ahora acepta una dirección
  explícita opcional, y se amplían los campos ordenables de
  `DEGREE|ID|NAME` a también `ACCESS_COUNT`, `RELEVANCE`, `LAST_SEEN` y
  `FIRST_SEEN`. Sin `ASC`/`DESC`, cada campo conserva su dirección por
  defecto de siempre (descendente para `DEGREE`/`ACCESS_COUNT`/
  `RELEVANCE`/`LAST_SEEN`/`FIRST_SEEN`, ascendente para `ID`/`NAME`).
- Ejemplo combinado: `SHOW AUTORELATIONS products FROM p145 DEPTH 6
  DIRECTION BOTH WHERE relevance >= 0.75 ORDER BY ACCESS_COUNT DESC
  LIMIT 100 FORMAT TREE STATS VERBOSE;`

## [Unreleased] — Auto-relaciones temporales basadas en patrones de acceso

- **`relations_auto.go`, nuevo `AutoRelationManager`**: cuando un mismo
  usuario lee repetidamente el mismo documento (por defecto, 5 lecturas
  en 10 minutos), CaimanDB crea automáticamente una auto-relación
  (self-relation) entre ese usuario y el documento — sin necesidad de
  un `RELATE` explícito. Cada auto-relación guarda `access_count`,
  `last_seen`, una puntuación de relevancia (`relevance`, combinando
  frecuencia y recencia) y una pequeña muestra de metadatos clave del
  documento (`key_metadata`, p. ej. `name`/`title`).
- **Son datos temporales por diseño**: cada nuevo acceso extiende su
  expiración (`auto_relation_ttl`, 24h por defecto); si el par
  usuario-documento deja de accederse, la relación expira sola y un
  barrido de fondo (`autoRelationCleanupLoop`, cada 5 min) la elimina.
  No es un dato duradero — es una señal viva de "qué es relevante
  ahora mismo".
- **`FIND` y `GET <id>` alimentan el detector automáticamente**
  (`cmd_find.go`): cada documento leído por una sesión se reporta al
  `AutoRelationManager` con el usuario autenticado de esa sesión.
- **`SHOW AUTORELATIONS <block>`** (`cmd_show_autorelations.go`,
  `cmd_show_autorelations_render.go`, `relations_auto_graph.go`): comando
  completo estilo consulta de grafo sobre ese conjunto de auto-relaciones
  (bipartito y dirigido: `usuario -> documento leído`), con modificadores
  `FROM`, `TO`, `DEPTH`, `DIRECTION IN|OUT|BOTH`, `FORMAT
  TABLE|TREE|GRAPH|JSON`, `ORDER BY DEGREE|ID|NAME`, `LIMIT`, `OFFSET`,
  `FILTER <expresión>` (reutiliza el mismo evaluador WHERE que `FIND`),
  `STATS`, `SUMMARY`, `PATHS` (árbol del recorrido BFS desde `FROM`),
  `ORPHANS` (pares aislados), `CYCLES` (detectados con union-find) y
  `BROKEN` (relaciones cuyo documento fue borrado).
- Nuevas opciones de configuración: `auto_relations_enabled`,
  `auto_relation_threshold`, `auto_relation_window`,
  `auto_relation_ttl`.

## [Unreleased] — Consola web (ui/) real con panel de estado, conectada a datos en vivo del nodo

- **Consola `ui/` reescrita** con el aspecto de un cliente de base de
  datos tipo Beekeeper Studio: rail de iconos con dos vistas (Editor de
  consultas / Dashboard), pestañas de consulta reales, barra lateral
  con las bases de datos y bloques del nodo, y panel de estado con
  KPIs y gráficas. Todo el contenido sale de peticiones HTTP reales al
  propio nodo — no hay datos de ejemplo ni `Math.random` en el cliente.
- **`GET /entities`** (`http_query.go`), nuevo endpoint del servidor de
  consultas: lista bases de datos (`Engine.ShowDBs`) y, para la base
  activa, sus bloques con conteo real de documentos y tamaño
  (`Engine.DescribeBlock`) — lo que alimenta la barra lateral y el
  gráfico de documentos por bloque.
- **`POST /query` ahora también devuelve `rows`/`columns`** para
  comandos `FIND`/`GET`: una grilla de documentos estructurada
  (re-ejecutando la misma lectura contra el motor), además del texto
  de resultado que ya devolvía todo comando. Así el cliente puede
  pintar una tabla real en vez de parsear la salida de texto pensada
  para terminal.
- **`L1Cache.Stats()` expone bytes crudos** (`used_bytes`, `max_bytes`,
  `hit_ratio_pct`) junto a las cadenas ya formateadas, para que el
  dashboard pueda calcular un porcentaje de uso de caché real sin
  parsear strings tipo "128.00 MB".

## [Unreleased] — Motor de alto rendimiento: caché con sharding, WAL con group-commit y change streams en tiempo real

- **Caché L1/L2 con sharding (32 particiones)** (`cache.go`). Antes cada
  caché tenía **un único mutex global** protegiendo todo el mapa + lista
  LRU, así que cualquier lectura/escritura serializaba a todos los demás
  hilos — el clásico cuello de botella bajo carga concurrente sostenida.
  Ahora cada caché se parte en 32 shards independientes (hash `fnv32a`
  ya usado por `shardedLockManager`), cada uno con su propio mutex, mapa
  y lista LRU/presupuesto de bytes, así que la contención cae de "global"
  a "1/32 de las claves". De paso se corrigió una **carrera de datos
  real**: `L2IndexCache.get()` tomaba `RLock` pero mutaba campos
  compartidos (`LastUsed`, `Frequency`) sin lock exclusivo. También se
  reemplazó el acceso directo a `l1Cache.mu`/`l1Cache.items` en
  `RenameDB` (que además tenía un **deadlock latente**: tomaba `Lock()`
  y luego llamaba a `del()`, que vuelve a tomar el mismo mutex) por el
  nuevo método `L1Cache.DeleteByPrefix`.
- **WAL con I/O en buffer + fsync configurable (group commit)**
  (`wal.go`). El WAL nunca hacía `fsync` — los datos quedaban en el
  buffer del SO sin garantía real de durabilidad pese a llamarse
  "Write-Ahead Log". Además cada entrada se escribía con
  `json.Encoder.Encode` directo contra el `*os.File`, sin buffer (un
  syscall por entrada). Ahora cada segmento usa un `bufio.Writer`, los
  batches se codifican en memoria (reutilizando el buffer pool, que
  antes se pedía pero nunca se usaba) para emitir **un solo `Write()`
  por batch**, y hay una política de sync configurable
  (`wal_sync_policy`: `always` / `interval` [default, group commit] /
  `off`) que acota la ventana máxima de pérdida de datos sin pagar un
  fsync por operación.
- **`BufferPool` sin condición de carrera** (`buffer_pool.go`): los
  contadores eran `int64` planos mutados desde múltiples goroutines sin
  atomics.
- **Change streams en tiempo real** (`changestream.go`): nuevo
  `ChangeBus` (pub/sub en proceso) que publica eventos de
  insert/update/delete conforme ocurren, con `Publish()` no bloqueante
  (un suscriptor lento nunca frena el camino de escritura — su evento se
  descarta y se cuenta en `caimandb_change_events_dropped_total`).
  Expuesto vía SSE en el nuevo endpoint `GET /watch?db=...&block=...`
  (`http_query.go`), con la conexión eximida del `WriteTimeout` global
  del servidor (que si no cortaría el stream a los 30s) y heartbeat cada
  15s para mantenerla viva a través de proxies/balanceadores.


- **Nuevo comando `EXPLAIN FIND ...` / `EXPLAIN SEARCH ...`**
  (`cmd_explain.go`). Estilo "EXPLAIN ANALYZE": construye la consulta con
  las mismas funciones que usa `FIND`/`SEARCH`
  (`buildFindQuery`/`buildSearchQuery`, extraídas de `cmd_find.go` para
  que EXPLAIN nunca pueda desincronizarse de lo que el comando real
  hace), la ejecuta de verdad, y reporta lo medido: filas escaneadas,
  filas encontradas, qué acceso se usó realmente (`Actual Access`), qué
  índice habría elegido el optimizador para un top-level de igualdades/IN
  (`Planned`, vía `QueryOptimizer.AnalyzeQuery`) -- y el árbol `WHERE`
  parseado (`parse.Expr.String()`), útil ahora que soporta paréntesis y
  precedencia. No inventa cifras que no midió (nada de "memoria usada"
  ni "costo" ficticios).
- **`SHOW DBS` y `SHOW BLOCKS` ahora aceptan uno o más nombres**
  (`cmd_show_size.go`): `SHOW DBS`, `SHOW DBS nombre`, `SHOW DBS nombre1
  nombre2`; igual para `SHOW BLOCKS [<db>] [nombre1 nombre2 ...]`. Si
  algún nombre no existe, se reporta en una línea "Not found: ..." en
  vez de fallar todo el comando. `SHOW DBS` además ahora incluye el
  tamaño en disco de cada base (antes solo mostraba blocks/docs) y un
  total agregado al final; `SHOW BLOCKS` ya mostraba tamaño por bloque y
  ahora también agrega un total.
- `docs/known-limitations.md` y `docs/nql-reference.md`: sin cambios en
  este pase (ver el bloque anterior para el estado del AST); `help.go`
  documenta la sintaxis nueva de `EXPLAIN` y `SHOW DBS/BLOCKS`.
- **Alcance de este pase, explícito:** solo se implementó `EXPLAIN` y las
  mejoras de `SHOW` pedidas. La visión más amplia de NQL discutida
  (`USING`/`EXPAND` para relaciones sin estado global de `RELATE`,
  `GROUP BY` dentro de `FIND`, funciones de agregación inline
  (`COUNT()`/`SUM()`/...), `DISTINCT`, `PAGE`/`SIZE`, `FIND ... IDS`,
  `FIND ... COUNT`, `FIND ... CACHE`, `ANALYZE FIND ...`) es un rediseño
  bastante más grande del lenguaje y no se tocó en este pase para no
  arriesgar cambios extensos sin poder compilar; queda pendiente si se
  quiere abordar por partes.
- **Sin verificar con `go build`/`go vet`** -- mismo aviso que el resto
  de este changelog: sin Go ni red en este entorno. Revisado a mano con
  atención especial a: que `buildFindQuery`/`buildSearchQuery` extraídas
  de `cmd_find.go` preserven el comportamiento exacto de
  `handleFind`/`handleSearch` (son el mismo código, solo movido a una
  función parametrizada por el índice de inicio), que no queden imports
  sin usar tras mover código entre archivos (`time` en
  `cmd_create_show.go`), y que ningún nombre de función se repita.
  Confírmalo con `go build ./... && go vet ./... && go test ./...`
  antes de desplegar.

## [Unreleased] — AST real para WHERE (paréntesis, precedencia, NOT)

- Nuevo `internal/caimandb/parse/ast.go`: un parser recursivo-descendente
  que compila una cláusula `WHERE` a un árbol de expresión (`Expr`)
  real -- `KindCondition` (hoja) / `KindAnd` / `KindOr` / `KindNot` --
  en vez de la lista plana `[]Filter` con un `Logic` por elemento que
  se evaluaba estrictamente de izquierda a derecha. Soporta:
  - Paréntesis para agrupar (`(a = 1 OR b = 2) AND c = 3`).
  - Precedencia estándar `AND` > `OR` (antes no existía: la única
    semántica posible era "de izquierda a derecha", así que
    `a=1 OR b=2 AND c=3` no se podía escribir con su significado
    habitual).
  - `NOT` delante de una condición o de un grupo completo.
  - Los mismos operadores y misma sintaxis de valores de siempre
    (comillas, arrays/objetos JSON, `BETWEEN x AND y`, `IS [NOT] NULL`,
    operadores de dos palabras) -- se preservó a propósito, incluida
    una peculiaridad del parser original (un valor entre comillas
    todavía pasa por la misma coerción numérica/booleana que uno sin
    comillas).
- `internal/caimandb/parse/tokenizer.go`: `(` y `)` ahora siempre se
  emiten como tokens propios (antes solo se reconocían corchetes `[]`
  y llaves `{}`), para que la agrupación del AST funcione sin exigir
  espacios exactos alrededor de los paréntesis.
- Conectado a `FIND` y `SEARCH` (`cmd_find.go`): nuevo
  `parseWhereClause` en `cmd_filters_util.go` construye el árbol y,
  además, una lista plana *best-effort* (solo cuando el árbol es una
  cadena pura de `AND` de nivel superior) para no tocar el
  planificador de índices existente (`QueryOptimizer.AnalyzeQuery`).
  `Query` (`ops_find.go`) tiene un campo nuevo `Where *parse.Expr`;
  `matchesQuery`/`evalExpr` (`query_filter.go`) evalúan el árbol
  cuando está presente y si no caen al `matchesFilters` de siempre --
  por diseño, todo lo demás que construye `[]Filter` directamente
  (`JOIN`, transacciones, `VIEW`, admin) sigue exactamente igual que
  antes de este cambio.
- `docs/nql-reference.md` documenta la sintaxis nueva con ejemplos;
  `docs/known-limitations.md` documenta qué comandos usan ya el AST y
  cuáles siguen en el parser plano, con una ruta de migración.
- **Sin verificar con `go build`/`go vet`** -- este entorno tampoco
  tuvo acceso a un compilador de Go ni a red (mismo aviso que el resto
  de este changelog). Revisado a mano con cuidado especial en los
  puntos de integración (firma de `Query`, los dos call-sites de
  `matchesFilters`, y que ningún literal `Query{...}` existente use
  campos posicionales), pero confírmalo con `go build ./... && go vet
  ./...` antes de desplegar.

## [Unreleased] — configs/caimandb.conf y primer subpaquete (parse/)

- `caimandb.conf` ahora se carga/crea en `configs/caimandb.conf` en vez
  de la raíz del directorio de trabajo (`internal/caimandb/constants.go`,
  `internal/caimandb/defaults.go`); `SaveToFile` crea la carpeta
  `configs/` sola si hace falta. Actualizados `help.go`, `README.md`,
  `docs/configuration.md`, `docs/architecture.md`,
  `examples/quickstart.md` y `.gitignore` para reflejarlo.
- Nuevo subpaquete `internal/caimandb/parse`: se movió el tokenizer de
  la sintaxis NQL (`tokenize` → `parse.Tokenize`), el único archivo
  del motor sin ninguna dependencia de `Engine`/`Document`/`Config`/
  `Session`/`Filter`/`Transaction`. `dsl_parser.go` y `cmd_view.go`
  ahora lo importan como `caimandb/internal/caimandb/parse`.
- `docs/known-limitations.md` ampliado: documenta este primer split
  seguro, incluye un filtro `grep` para encontrar más archivos "hoja"
  candidatos, y mantiene la ruta recomendada (`httpapi`, `cluster`,
  dejar `raft_fsm.go`/`transaction.go` en el núcleo) para quien
  quiera seguir dividiendo el paquete con un compilador Go a mano.

## [Unreleased] — Reorganización del repositorio

- Nueva estructura de carpetas a nivel de repo: `docs/` (con
  `docs/api/`), `deployments/docker/`, `scripts/`, `configs/`,
  `examples/`, `test/` (con `test/integration/` y `test/fixtures/`),
  `.github/workflows/`.
- Documentación nueva: arquitectura (`docs/architecture.md`),
  referencia NQL completa (`docs/nql-reference.md`), API HTTP
  (`docs/api/http-api.md`), configuración (`docs/configuration.md`),
  limitaciones conocidas (`docs/known-limitations.md`).
- Plantilla de configuración (`configs/caimandb.conf.example`) generada
  a partir del struct `Config` real.
- `Dockerfile` + `docker-compose.yml` para levantar CaimanDB en
  contenedor.
- Scripts de build/test/run (`scripts/*.sh`) y `Makefile`.
- Workflow de CI en GitHub Actions (`go build`, `go vet`, `go test`,
  `go mod tidy` check).
- `internal/caimandb` se dejó intacto a propósito: es un único paquete
  Go con más de 40 archivos acoplados al mismo `Engine` mediante
  campos no exportados; dividirlo de verdad requiere exportar buena
  parte de ese estado y verificarlo con un compilador, algo que no
  fue posible confirmar en este entorno (ver
  `docs/known-limitations.md`).

## Pase anterior — Corrección de compilación

- Estructura `cmd/` + `internal/caimandb/` en vez de un directorio
  plano de 68 archivos en la raíz.
- Eliminada la redeclaración duplicada de `views`/`viewsMu`.
- Implementados 11 manejadores NQL que `dsl_parser.go` referenciaba
  pero no existían (`handleDrop`, `handleRename`, `handleInfo`,
  `handleDescribe`, `handleStats`, `handleSize`, `handleRebuild`,
  `handleCheck`, `handleRepair`, `handleFlexCommand`,
  `handleTransaction`), en `cmd_admin_extra.go`.
- Eliminados imports no usados de `github.com/hashicorp/raft` y
  `errors` en 11 archivos.
