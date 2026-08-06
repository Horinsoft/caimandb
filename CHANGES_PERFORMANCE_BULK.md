# Cambios: rendimiento de consultas + BATCH/bulk import con lotes grandes

No pude compilar esto en el sandbox (sin Go instalado y sin acceso de red
para descargarlo), así que corran `go build ./...` y `go vet ./...` apenas
descompriman el zip, antes de desplegar. Revisé todo a mano con cuidado,
pero no hay sustituto de un build real.

## 1. Bug de rendimiento en consultas (el hallazgo principal)

El motor ya construye índices secundarios en cada INSERT
(`buildSecondaryIndexInto`, `index_secondary.go`), pero el planificador de
consultas (`QueryOptimizer.AnalyzeQuery`, `query_optimizer.go`) nunca los
usaba para campos reales: la única llamada a `UpdateIndexStats` en todo el
código (`ops_local.go`) pasaba el par sin sentido `("_id", 0, 0)` en cada
insert individual. Como `AnalyzeQuery` exigía una selectividad ya medida
`< 0.3` para activar el índice, y esa estadística nunca se llenaba para
campos reales (email, status, category, ...), **prácticamente cualquier
FIND con filtros terminaba en un full scan del bloque**, sin importar
cuántos índices secundarios existieran en disco.

Archivos tocados:

- `query_optimizer.go` -- `AnalyzeQuery` ahora ofrece cualquier filtro de
  igualdad/IN como candidato de índice (el lookup es barato y cae de forma
  segura a full scan si no hay resultados), en vez de exigir estadísticas
  previas que nunca llegaban a existir. `QueryPlan` gana un campo
  `Candidates []IndexCandidate` con todos los campos indexables del
  filtro, ordenados por selectividad conocida (los medidos primero, más
  selectivos primero). `IndexField`/`IndexValue` se mantienen apuntando al
  mejor candidato para no romper `Count()` (`ops_delete.go`) ni
  `EXPLAIN FIND` (`cmd_explain.go`), que solo conocían un campo.

- `index_secondary.go` -- `lookupByIndex` ahora alimenta selectividad real
  a `UpdateIndexStats` cada vez que efectivamente se usa un índice
  (aproximación: docs encontrados / DocCount de la base, ya en memoria vía
  `intelEngine.stats`, sin I/O extra), así el planificador aprende con el
  uso real en vez de quedarse vacío para siempre. También agrega
  `intersectIDs`, el helper de intersección de sets de IDs.

- `ops_find.go` -- `Find()` ahora, cuando el plan trae más de un campo
  indexable, intersecta hasta 2 candidatos adicionales (además del mejor)
  antes de tocar ningún documento, en vez de resolver solo por el primer
  campo y filtrar el resto con `matchesQuery` después de traer cada
  documento completo. Acotado (`maxExtraCandidates = 2`) para que un campo
  poco selectivo no fuerce lookups extra quando el set ya es chico.

Nada de esto cambia el formato en disco ni el índice en sí -- es
estrictamente el planificador aprendiendo a usar lo que el motor de
indexado ya construía.

## 2. BATCH / bulk import con lotes grandes (nuevo)

- `ops_bulk_import.go` -- `Engine.BulkImportReader(dbName, block string,
  r io.Reader, opts BulkImportOptions) (*BulkImportResult, error)`:
  - Lee NDJSON (un JSON por línea, default) o un array JSON top-level
    (`Format: "array"`), ambos con decodificadores *streaming* -- nunca
    carga el archivo completo en memoria, solo un lote (`BatchSize`,
    default 20000 documentos) a la vez.
  - Reusa el mismo camino ya endurecido de `InsertBatch`
    (`insertBatchLocalDetailed`): mismo WAL, mismo WriteBatch de Badger,
    misma indexación secundaria inline, mismo manejo de documentos
    grandes/compresión -- no se reimplementa nada de bajo nivel, solo se
    llama en lotes mucho más grandes que los usados hasta ahora.
  - Activa `BULK MODE` automáticamente durante la carga (ventanas de
    batching más anchas, fsync de WAL relajado) y lo restaura al terminar,
    salvo que el caller ya lo esté manejando (`SkipBulkModeToggle`).
  - Reporta progreso incremental (`Progress func(BulkImportProgress)`) y
    un resultado final con contadores exactos + una muestra acotada
    (`MaxErrorSamples`, default 20) de mensajes de error, para no gastar
    memoria en archivos con millones de líneas mal formadas.

- `cmd_import.go` + registro en `dsl_parser.go` -- comando de consola:

  ```
  IMPORT <block> FROM FILE '<path>' [FORMAT NDJSON|ARRAY] [BATCH <n>]
  ```

  Soporta `.gz` de forma transparente (mismo criterio que `RESTORE` ya usa
  para backups). Ejemplo:

  ```
  USE mydb
  IMPORT events FROM FILE '/data/events.ndjson.gz' BATCH 50000
  ```

- `help.go` -- documentado junto a `BULK MODE`.

## Qué no se tocó

- El formato en disco, el WAL, la replicación Raft y el resto del motor de
  indexado quedaron intactos -- todo esto es aditivo.
- No hay tests nuevos incluidos (no pude correr `go test` en este entorno
  para validarlos). Recomiendo agregar al menos un test de integración que
  cargue un NDJSON de varios miles de documentos vía `BulkImportReader` y
  luego confirme que `Find` con un filtro de igualdad reporta
  `IndexUsed != "full_scan"`.
