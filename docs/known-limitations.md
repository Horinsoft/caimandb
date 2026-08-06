# Limitaciones conocidas

## El AST de WHERE solo lo usan `FIND` y `SEARCH` todavía

`internal/caimandb/parse/ast.go` añade un árbol de expresión real para
`WHERE` (`ParseWhere`): soporta paréntesis, `NOT`, y precedencia
estándar de `AND`/`OR` (antes no existía ninguna de las tres cosas --
ver el comentario al principio de ese archivo para el porqué). Está
conectado a `FIND` y `SEARCH` (`handleFind`/`handleSearch` en
`cmd_find.go`, vía `parseWhereClause` en `cmd_filters_util.go` y
`matchesQuery`/`evalExpr` en `query_filter.go`).

El resto de comandos con `WHERE` -- `UPDATE`, `DELETE`,
`CLEAR`/`COUNT`, las agregaciones (`SUM`/`AVG`/...), `VIEW CREATE` y
los comandos de admin -- siguen usando `parseFilters`
(`cmd_filters_util.go`), el parser plano original: sin paréntesis, sin
`NOT`, y evaluado estrictamente de izquierda a derecha. Migrarlos es
mecánico (mismo patrón `tokens []string, idx *int`: cambiar la llamada
a `parseFilters` por `parseWhereClause` y decidir, comando por
comando, si ese comando necesita seguir alimentando un `[]Filter`
plano en algún otro sitio -- p. ej. `transaction.go` y `raft_fsm.go`
podrían serializar/reproducir filtros en el WAL/Raft log, así que hay
que confirmar eso con el compilador antes de tocarlos), pero no se
hizo en este pase para mantener acotado el riesgo sin poder compilar
(ver más abajo). Candidatos, en orden razonable:

1. `DELETE` / `CLEAR`+`COUNT` (`cmd_delete.go`, `cmd_clear_count.go`) --
   ya son de un único bloque de `Filter`s sin la complicación de
   `transaction.go`/`raft_fsm.go`.
2. Agregaciones (`cmd_aggregate.go`) -- misma forma.
3. `UPDATE` (`cmd_update.go`) y `VIEW CREATE` (`cmd_view.go`) -- revisar
   antes si `ViewDefinition.Filters` o el registro de transacciones
   necesitan serializar el filtro (JSON, WAL, Raft) en algún punto; si
   es así, añadir también la serialización de `parse.Expr` antes de
   cambiar el parser que los produce.

## `internal/caimandb` sigue siendo un solo paquete Go

Es la limitación más relevante y la razón por la que este pase de
reorganización se centró en la estructura del **repositorio**
(carpetas `docs/`, `deployments/`, `scripts/`, `configs/`, `examples/`,
`test/`, CI) y no en dividir el paquete Go en sí.

Se investigó de forma concreta qué haría falta para dividirlo en
paquetes como `internal/engine`, `internal/storage`,
`internal/cluster`, `internal/httpapi`, etc. Resultado del análisis
estático (grep de accesos `.campo` / `.método` no exportados entre
archivos):

- Archivos como `raft_fsm.go` y `transaction.go` acceden directamente
  a más de diez campos/métodos **no exportados** del `Engine` de
  subsistemas distintos (`engine.pool`, `engine.dirMgr`,
  `engine.cacheKey`, `engine.lockManager`, `engine.l1Cache`,
  `engine.shardMgr`, `engine.externalStore`, `engine.intelEngine`,
  `engine.flexEngine`, `engine.buildSecondaryIndex`, ...). Moverlos a
  otro paquete obligaría a exportar buena parte del estado interno
  del motor.
- Varios subsistemas (`cluster.go`, `dist_query.go`, `flexcolumn.go`,
  `http_admin.go`, `http_query.go`, `shard_manager.go`,
  `transaction.go`) guardan una referencia de vuelta al propio
  `*Engine` (`engine *Engine`). Si esos tipos se moviesen a paquetes
  separados mientras `Engine` los mantiene como campos, se produciría
  un ciclo de imports; evitarlo correctamente requiere invertir la
  dependencia con interfaces (definir en el paquete nuevo solo los
  métodos que de verdad usa, y que `Engine` los satisfaga
  implícitamente). Es un cambio real y razonable, pero mecánico y
  extenso, y con alto riesgo de romper una build de 15 000+ líneas si
  se hace sin un compilador que confirme cada punto de uso.

Este entorno no tiene acceso a red ni a un toolchain de Go instalado,
así que no había forma de compilar y confirmar un cambio de ese
tamaño. Se optó por no aplicarlo a ciegas.

### Lo que ya se separó: `internal/caimandb/parse`

`tokenizer.go` (la función `tokenize`, ahora `parse.Tokenize`) no
tenía ninguna referencia a `Engine`, `Document`, `Config`, `Session`,
`Filter` ni `Transaction` — solo `strings` de la librería estándar.
Al ser un archivo "hoja" de verdad (cero acoplamiento, no solo
acoplamiento bajo), se movió sin necesidad de exportar nada del
motor ni de arriesgar un ciclo de imports. Los dos puntos que lo
llamaban (`dsl_parser.go`, `cmd_view.go`) ahora importan
`caimandb/internal/caimandb/parse`.

### Si se quiere ir más lejos

Es un trabajo abordable con un compilador Go a mano (local o CI),
subsistema por subsistema. Un primer filtro rápido para encontrar
más candidatos "hoja" como `parse`:

```bash
cd internal/caimandb
for f in *.go; do
  grep -qE "\bEngine\b|\bDocument\b|\bConfig\b|\bSession\b|\bFilter\b|\bTransaction\b" "$f" || echo "$f"
done
```

Eso señala ~18 archivos (`auth_jwt.go`, `cache.go`, `buffer_pool.go`,
`compression.go`, `ratelimit.go`, `audit.go`, `logging.go`,
`metrics.go`, `locks.go`, `worker_pool.go`, `storage_external.go`,
`raft_logstore.go`, `misc_utils.go`, `nested_fields.go`, entre otros)
que no referencian los tipos centrales del motor por nombre — son
buenos candidatos a paquetes propios, pero **antes de moverlos** hay
que revisar el acoplamiento *entre ellos* (p. ej. si varios comparten
`log()` de `logging.go` o un tipo de caché), cosa que este filtro no
detecta. Después de eso, en orden de menor a mayor acoplamiento con
el `Engine`:

1. `internal/httpapi` — mover `http_admin.go` y `http_query.go`;
   solo requiere exportar ~10 campos del `Engine`
   (`nodeID`, `startupTime`, `opCount`, `l1Cache`, `metrics`,
   `tokenManager`, `cluster`, `flexEngine`, `shardMgr`, `config`) vía
   getters.
2. `internal/cluster` — mover `cluster.go`, `dist_query.go`,
   `flexcolumn.go`, `shard_manager.go`; acoplamiento moderado.
3. Dejar `raft_fsm.go` y `transaction.go` en el paquete núcleo
   (`internal/engine`) — son los que más profundamente tocan el
   estado interno y los que menos beneficio/riesgo tienen al
   separarlos.

Cada paso debería terminar con `go build ./... && go vet ./...` antes
de pasar al siguiente.

## `go.sum` no verificado

`go.mod` se mantiene igual que en el proyecto original. Ejecuta
`go mod tidy` en tu máquina tras el primer build exitoso para
confirmar que `go.sum` es consistente.

## Sin suite de pruebas automatizadas

El proyecto original no traía tests. Se añadió `test/` con una
plantilla y una guía (`test/README.md`) para empezar, pero no se
inventaron pruebas que simulen cobertura que no existe.
