# Corrección de bugs de corrupción / pérdida de datos (julio 2026)

Este documento describe bugs reales encontrados en `internal/caimandb/wal.go`
y `internal/caimandb/block_repair.go` que podían causar pérdida silenciosa de
datos o dejar corrupción sin reparar, y los cambios aplicados para
corregirlos.

**Importante:** este entorno no tuvo acceso a red ni a un toolchain de Go
instalado, así que estos cambios se revisaron a mano (balance de
llaves/paréntesis, tipos, firmas de función) pero **no se compilaron**.
Ejecuta `go build ./... && go vet ./...` y la suite de tests antes de
desplegar.

## 1. Bug crítico: el marcador `.clean` nunca se invalidaba al arrancar

**Síntoma:** tras un apagado limpio seguido, en algún momento posterior,
de un cierre NO controlado (`kill -9`, corte de luz, panic), al reiniciar
el motor se perdían silenciosamente **todas** las escrituras hechas entre
el último apagado limpio y el crash — sin ningún error visible.

**Causa:** `WAL.Close()` escribe un archivo `.clean` para decirle al
siguiente arranque "no hace falta replay, todo ya está aplicado". Pero
ese archivo nunca se borraba al reanudar operación. Secuencia del bug:

1. Arranque limpio → `.clean` presente → se omite recovery (correcto).
2. El motor sigue corriendo, ~~se sigue escribiendo al WAL~~, pero el
   archivo `.clean` sigue ahí, ahora **obsoleto**.
3. El proceso muere sin pasar por `Close()`.
4. Reinicio: `checkCleanliness()` encuentra el `.clean` viejo → asume
   que todo estaba aplicado → **se salta el recovery** → todo lo escrito
   entre el paso 1 y el 3 desaparece para siempre.

**Fix:** `checkCleanliness()` ahora borra el marcador inmediatamente
después de leerlo, sea cual sea su valor, porque a partir de ese momento
el WAL vuelve a aceptar escrituras y el marcador ya no es válido para lo
que pase después. Solo se vuelve a escribir en un `Close()` correcto.

Se corrigió también un caso límite relacionado: si el último segmento en
disco ya estaba en o por encima de `maxSize` al arrancar (p. ej. un
crash a mitad de una rotación), `recover()` retornaba temprano vía
`rotate()` **sin llamar nunca a `checkCleanliness()`**, dejando
`w.clean` en su valor por defecto del constructor (`true`) — es decir,
se saltaba el recovery en un escenario que bien podía ser un apagado no
controlado. Ahora `checkCleanliness()` se llama antes de cualquier
`return` temprano.

## 2. `RecoverApply` / `Truncate` escribían el marcador `.clean` en el momento equivocado

Ambas funciones volvían a escribir el archivo `.clean` justo después de
aplicar el recovery (o de descartarlo, en `FastStartup`) — reintroduciendo
exactamente el mismo problema del punto 1, porque la operación normal
seguía corriendo después de eso. Se quitó la escritura del marcador de
ambos sitios: **el único lugar donde `.clean` debe escribirse es
`Close()`**, tras un apagado ordenado.

De paso, `RecoverApply` ahora **poda (borra) los segmentos de WAL ya
aplicados** una vez el recovery termina con éxito (antes se dejaban en
disco indefinidamente). Esto evita un problema de idempotencia: si el
proceso vuelve a caerse antes del próximo `Close()` limpio, ya no se
reprocesan las mismas entradas — importante porque un `insert` es
idempotente por ID, pero una `update` tipo incremento (`$inc`) no lo es,
y aplicarla dos veces corrompería el valor.

## 3. `Write()` en modo streaming confirmaba éxito antes de escribir nada

**Síntoma:** bajo carga, una operación podía reportarse como
"escrita en el WAL" (`err == nil`) mientras la entrada seguía sentada
sin escribir en un canal de Go en memoria, hasta por 5 segundos (el
intervalo del ticker) o indefinidamente si el tráfico nunca llegaba al
umbral de lote (100 entradas). Si el proceso caía en esa ventana, la
entrada se perdía pese a que el llamador ya había recibido éxito.

**Fix:** se introdujo `walSubmission` (entrada + canal de confirmación).
`Write()` en modo streaming ahora espera a que `flushBatch` realmente
escriba (y, si corresponde, sincronice) el lote antes de devolver el
resultado al llamador. Para no perder el beneficio de rendimiento del
agrupado por lotes, se añadió una ventana de "linger" corta e
independiente del ticker de fsync (`walLingerWindow = 20ms`) que fuerza
la escritura de un lote parcial aunque no haya llegado a 100 entradas ni
al tick de 5s — así ninguna escritura queda "en el aire" más de ~20ms
antes de, como mínimo, ser escrita al buffer del sistema operativo.

Este cambio es puramente de temporización/confirmación: no cambia la
política de fsync configurada (`WALSyncAlways` / `WALSyncInterval` /
`WALSyncOff`), que sigue controlando cuándo se hace `fsync` real.

## 4. `RepairBlock` detectaba corrupción pero no la reparaba

**Síntoma:** `CheckBlock` cuenta correctamente los documentos cuyo valor
almacenado no puede deserializarse (`corrupted`), pero `RepairBlock`
simplemente llamaba a `RebuildBlock`, que solo reconstruye **índices
secundarios** a partir de los documentos que sí parsean — los
documentos corruptos se saltaban silenciosamente (`continue`) y se
quedaban en disco para siempre. `RepairBlock` no reparaba nada.

**Fix:** se añadió `quarantineCorruptedDocs`, que borra específicamente
las claves cuyo valor no deserializa (bytes corruptos no se pueden
"reparar", solo eliminar) y deja un log `WARN` por cada clave eliminada
(`db`, `block`, `key`) para que el operador sepa exactamente qué se
perdió y pueda restaurarlo desde un backup si existe uno. `RepairBlock`
ahora: cuenta corrupción → pone en cuarentena las claves corruptas →
reconstruye los índices secundarios sobre lo que queda.

## 5. Nuevo: mantenimiento autónomo (integridad + compactación en frío)

Se añadió `maintenance.go`: un barrido periódico, de baja prioridad, que
corre en segundo plano (`go engine.maintenanceLoop()`, arrancado junto a
`backgroundCleanup`) y hace dos cosas por cada bloque de cada base de
datos:

1. **Verifica integridad** con `CheckBlock` y, si `MaintenanceAutoRepair`
   está activo (por defecto sí), llama a `RepairBlock` automáticamente —
   que ahora sí repara de verdad (ver punto 4 arriba). Esto es lo que
   convierte "detectar corrupción" en "garantizar integridad" de forma
   autónoma: nadie tiene que acordarse de correr `REPAIR` a mano.
2. **Compacta bloques fríos**: si `MaintenanceAutoCompact` está activo,
   cualquier bloque cuyo último `UpdatedAt` sea más viejo que
   `MaintenanceColdAfter` (24h por defecto) se compacta con
   `CompactBlock` (nueva función en `ops_compact.go`, equivalente a
   `Compact` pero a nivel de bloque en vez de base de datos completa),
   liberando espacio en disco de datos que ya no cambian sin tocar los
   bloques que sí están activos.

**Diseñado para bajo consumo de hardware:** el barrido duerme
`MaintenanceBlockPause` (200ms por defecto) entre bloque y bloque,
pase lo que pase en ese bloque — nunca corre "a toda velocidad". El
primer barrido se retrasa 2 minutos tras el arranque (para no competir
con el recovery del WAL) y luego se repite cada `MaintenanceInterval`
(6h por defecto). Todo es configurable vía `Config`
(`maintenance_enabled`, `maintenance_interval`,
`maintenance_block_pause`, `maintenance_auto_repair`,
`maintenance_auto_compact`, `maintenance_cold_after` en el JSON de
config). También hay un disparador manual, `Engine.RunMaintenanceNow()`,
para exponerlo luego vía comando admin o endpoint HTTP si se quiere.

No se tocó `IntelligentEngine` (`engine_core.go`), que ya cubre
auto-tune, auto-scale e índices sugeridos — este mantenimiento es
complementario, no un reemplazo: cubre integridad física y espacio en
disco, algo que `IntelligentEngine` no hacía.

## 6. `RecoverApply`/`RecoverWAL` podían reproducir dos veces la misma entrada tras un segundo crash (agosto 2026)

**Síntoma:** un `update` tipo `$inc` (u otra operación no idempotente)
podía aplicarse dos veces si el motor sufría **dos** cierres no
controlados seguidos, sin un `Close()` limpio entre ambos —
corrompiendo el valor silenciosamente.

**Causa:** tras aplicar con éxito las entradas recuperadas, el código
llamaba a `WAL.PruneToLastSegment()`, que borra todos los segmentos
**excepto el último**. El problema es que ese "último" segmento es el
mismo archivo, todavía abierto, en el que `ReadAll()` acababa de
encontrar (y aplicar) esas entradas — y en el que las escrituras
*nuevas*, posteriores al arranque, se van a seguir añadiendo a
continuación. Si el proceso vuelve a caer antes del próximo `Close()`
limpio, el siguiente arranque vuelve a leer ese mismo segmento —
entradas viejas ya aplicadas incluidas — y las vuelve a aplicar.

**Fix:** nuevo `WAL.RotateAndPruneFresh()` (`internal/caimandb/wal/wal.go`):
cierra el segmento actual, abre uno nuevo **vacío**, y solo entonces
borra el resto (incluido el que se acaba de cerrar). `RecoverWAL`
(`wal_recovery.go`) ahora llama a este método en vez de
`PruneToLastSegment()` directamente. Con esto, tras una recuperación
exitosa no queda en disco ninguna entrada "ya aplicada": el segmento
activo arranca en cero y solo contiene lo que se escriba de aquí en
adelante, cerrando la ventana de doble aplicación.

`PruneToLastSegment()` se deja intacta (y documentada) para el resto de
sus usos, que no comparten este problema porque no se llaman justo
después de reproducir el segmento activo.

## Qué NO se tocó

- `maintenance.go` es código nuevo (no una edición de algo existente),
  así que revísalo con el mismo cuidado — en particular confirma que
  `DirectoryManager.ListDatabases`/`ListBlocks` y `CheckBlock` se
  comportan como se asume aquí bajo tu volumen real de bloques antes de
  dejar `MaintenanceEnabled: true` en producción con datos grandes.


- No se auditó `transaction.go` / `raft_fsm.go` en detalle en este pase
  (fuera del alcance de tiempo disponible). Dado que ambos serializan
  datos hacia el WAL/Raft log, conviene revisarlos con el mismo criterio
  (¿se confirma éxito antes de que el dato esté realmente persistido?).
- No se auditó `badger_pool.go` más allá de su uso en `block_repair.go`.
- Compilar y correr `go vet` / tests reales es el siguiente paso
  obligatorio antes de confiar en estos cambios en producción.
