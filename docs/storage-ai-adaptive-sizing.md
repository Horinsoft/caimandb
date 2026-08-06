# Storage AI: sizing adaptativo de Badger por bloque (julio 2026)

**Importante, igual que en los otros documentos de este pase:** este
entorno no tuvo acceso a red ni a un toolchain de Go instalado, así que lo
que sigue se escribió y revisó a mano (tipos, firmas, balance de
llaves/paréntesis) pero **no se compiló**. Ejecuta
`go build ./... && go vet ./...` antes de confiar en esto, y sobre todo
antes de desplegarlo con datos reales.

## El problema reportado

> Todos los bloques pesan casi lo mismo (~49 MB), incluso con muy pocos
> documentos. Esto indica que el tamaño proviene de la preasignación de
> BadgerDB, no de los datos almacenados.

Correcto. Antes de este cambio, `storage/badger_pool.go` abría **todos**
los bloques (`__data`, `__index`, `__users`, `__system`, sin importar
cuántos documentos tuvieran) con exactamente las mismas opciones fijas de
BadgerDB, definidas en `storage/constants.go`:

```go
badgerValueLogSize       = 64 << 20   // 64MB
badgerMemTableSize       = 128 << 20  // 128MB
badgerNumMemTables       = 4          // → hasta 512MB de memtables
badgerBlockCacheSize     = 512 << 20  // 512MB de caché de bloques
```

Badger reserva/crea sus archivos de memtable y value-log de acuerdo a esos
números al abrir, independientemente de cuántos documentos vayan a vivir
ahí. Un bloque nuevo con 3 documentos paga el mismo footprint en disco que
uno con 300 000. Con miles de bloques pequeños, eso se vuelve un problema
real de espacio en disco (y de RAM, por el `BlockCacheSize` de 512MB × N
bloques abiertos simultáneamente).

## La solución implementada: tiers + presupuesto de RAM compartido

`internal/caimandb/storage/adaptive.go` (nuevo) introduce 4 **tiers** de
configuración de Badger:

| Tier | MemTable | NumMemtables | ValueLog | BlockCache | Pensado para |
|---|---|---|---|---|---|
| `micro` | 4MB | 2 | 8MB | 8MB | Bloque nuevo o casi vacío |
| `small` | 16MB | 3 | 16MB | 32MB | Algunos datos, lejos de "big data" |
| `standard` | 128MB | 4 | 64MB | 512MB | Los valores fijos originales |
| `large` | 256MB | 6 | 128MB | 1GB | Bloque ya con volumen serio |

`storage/badger_pool.go` (`DBPool.pickTierLocked`), al abrir un bloque:

1. **Mira qué hay ya en disco** para ese bloque (`onDiskFootprintBytes`):
   un bloque que no existe todavía, o que existe pero está vacío, cae en
   `micro` — esto es lo que arregla directamente el "~49MB con pocos
   documentos". Un bloque que ya tiene, por ejemplo, 200MB en disco se
   reabre en `standard` o `large`, no en `micro`.
2. **Lo ajusta contra un presupuesto de RAM compartido** entre todos los
   bloques actualmente abiertos en el proceso (`DBPool.budgetTotal`, por
   defecto el 50% de la RAM detectada vía `/proc/meminfo` en Linux, o
   2GB por defecto si no se puede detectar — configurable con
   `storage_ai_ram_fraction` / `storage_ai_max_budget_mb` /
   `CAIMANDB_STORAGE_AI_MAX_BUDGET_MB`). Si el tier "ideal" para ese
   bloque no cabe en lo que queda de presupuesto, se abre en el tier más
   grande que sí quepa — nunca se rechaza abrir un bloque por falta de
   presupuesto, en el peor caso se abre en `micro`.

Esto es exactamente lo que se pedía: *"que los bloques pequeños ocupen
poco espacio y los grandes maximicen el rendimiento"*, con la RAM y la
carga (número de bloques abiertos a la vez) como entrada de la decisión.

`storage_ai_enabled=false` en la config (o `CAIMANDB_STORAGE_AI_ENABLED=false`)
desactiva todo esto y reproduce el comportamiento fijo original
byte-por-byte (tier `standard` para todo).

## Concurrencia y velocidad de consultas: qué ya existía y qué se ajustó

- **Cada bloque ya es una instancia BadgerDB independiente**
  (`pool.OpenBlock` → `openBlock` por `(db, block)`), así que la
  concurrencia entre bloques ya era real antes de este cambio: dos
  bloques distintos se leen/escriben sin contención entre sí.
- Dentro de un mismo bloque, Badger ya maneja sus propias transacciones
  MVCC concurrentes (lecturas snapshot no bloquean escrituras).
- Lo nuevo: el tier `large` sube `NumCompactors` (más goroutines de
  compactación en paralelo) y `NumLevelZeroTables` (más margen antes de
  que las escrituras tengan que esperar a que L0 compacte), pensado para
  que un bloque grande bajo carga de escritura concurrente sostenida no
  se estanque tan pronto. `micro`/`small` se quedan con menos
  compactores porque, con tan poco dato, no hay nada que compactar en
  paralelo — sería puro overhead.
- `AdaptiveStats()` en `DBPool` expone cuántos bloques hay abiertos en
  cada tier y el presupuesto de RAM usado/total, para poder verificar en
  producción que la clasificación se está comportando como se espera
  (aún no está conectado a ningún endpoint HTTP/métrica — ver
  "Siguiente paso" más abajo).

## Lo que esto NO hace (a propósito)

**No hay reajuste en caliente de un bloque que ya está sirviendo
tráfico.** El tier de un bloque se decide una sola vez, la primera vez
que se abre en un proceso dado (normalmente al arrancar, o la primera
vez que algo lo toca si el proceso lo abre de forma perezosa). Si un
bloque nace en `micro` y crece mucho mientras el proceso sigue corriendo,
se queda en `micro` hasta el próximo reinicio/reapertura de ese bloque —
en ese momento `onDiskFootprintBytes` ya verá el tamaño real y lo abrirá
en el tier que le corresponde.

Se decidió así, no por descuido: subir de tier a un bloque **vivo**
significa cerrar su `*badger.DB` y volver a abrirlo con otras opciones, y
en este repo eso no es seguro sin más. Revisando quién usa el pool:

- `ops_insert.go`, `ops_local.go`, `transaction.go` sí toman
  `e.lockManager.Lock(<db>/<block>)` antes de escribir.
- **`ops_find.go` (lecturas) no toma ese lock** — se apoya en el
  aislamiento MVCC de Badger, que asume que el `*badger.DB` sigue vivo
  mientras dura la transacción/iterador.

Cerrar el handle de Badger mientras hay iteradores o transacciones de
lectura en vuelo en otra goroutine no es una operación segura documentada
en Badger. Para hacerlo bien haría falta, como mínimo, que el resize
tome el mismo lock que ya usan los escritores (bloqueando escrituras
nuevas durante el swap) **y** alguna forma de esperar a que los lectores
en vuelo terminen (algo que hoy no existe, porque hoy nada necesita
esperar a que un lector termine). Es exactamente el tipo de cambio que
este repo ya evitó hacer "a ciegas" en otros sitios (ver
`docs/known-limitations.md`) por no tener un compilador ni tests de
concurrencia a mano para verificarlo.

## Siguiente paso razonable (no implementado aquí)

Si se quiere upgrade en caliente de verdad, el camino más seguro es
extender la barrida de `maintenance.go` (que ya es de un solo goroutine,
ya pausa entre bloques, y ya sabe qué bloques están fríos/calientes):

1. Añadir a la config algo como `AdaptiveResizeEnabled` +
   `AdaptiveResizeCheckInterval`.
2. En cada pasada de `runMaintenanceSweep`, para cada bloque: comparar
   `db.Size()` (tamaño real actual de LSM+vlog, que Badger expone) contra
   el tier con el que se abrió (`DBPool.tierOf`).
3. Si se pasó del techo de su tier: tomar `e.lockManager.Lock(key)` (el
   mismo que usan los escritores) para bloquear escrituras nuevas a ese
   bloque, cerrar y reabrir el handle con el tier nuevo, liberar el lock.
   Esto bloquea escritores durante el swap (aceptable, es breve y poco
   frecuente) pero **no protege lectores que no toman ese lock hoy** —
   habría que decidir primero si `ops_find.go` empieza a tomar un
   `RLock()` de ese mismo `shardedLockManager` (ahora mismo tiene
   `RLock`/`RUnlock` ya implementados y sin usar — ver `locks.go`) antes
   de poder llamar esto realmente seguro bajo carga concurrente.

Cada uno de esos tres pasos necesita compilar y correr contra tráfico
concurrente real antes de confiar en él — no se implementó a ciegas en
este pase por la misma razón que el resto de límites documentados en
`docs/known-limitations.md`.
