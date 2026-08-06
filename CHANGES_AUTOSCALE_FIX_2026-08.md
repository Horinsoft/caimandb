# Fix: runaway autoscale + batch size fijo en GENERATE (2026-08)

Contexto: benchmark de inserción masiva en hardware de 2 núcleos (Celeron
N4020) terminó con 18 workers autogenerados y ~3.392 docs/s, muy por
debajo de lo esperable. Diagnóstico y fixes:

## 1. `runRateWatchdog` solo escalaba hacia arriba, nunca se frenaba
(`internal/caimandb/cmd_insert.go`)

El vigilante de `GENERATE` medía docs/seg cada 2s y, si el ritmo caía por
debajo del 85% del mejor ritmo visto, sumaba 2 workers -- sin techo
ligado al hardware real (usaba `maxGenerateWorkers = 64`, un tope de
sanidad genérico) y sin enfriamiento entre sumas.

En una máquina de pocos núcleos, superar el número de núcleos con
workers CPU-bound (preparación de JSON/compresión) + contención sobre el
lock único por bloque en `insertBatchLocalDetailed` **empeora** el
ritmo. El vigilante interpretaba esa caída como "faltan workers" y sumaba
más, en un ciclo que se retroalimentaba y solo paraba en el tope
absoluto de 64.

**Fix:**
- Nuevo `autoScaleMaxWorkers(multiplier)`: techo automático =
  `GOMAXPROCS * multiplier` (default `multiplier = 4`, configurable via
  `Config.GenerateAutoScaleMaxMultiplier`), acotado por el tope absoluto
  de 64. Un `WORKERS <n>` explícito sigue pudiendo llegar hasta 64 --
  este cambio solo afecta al camino *automático*.
- Nuevo enfriamiento (`autoScaleCooldownTicks = 2`): después de sumar
  workers, el vigilante espera 2 chequeos (4s) antes de volver a evaluar
  si hace falta sumar más, en vez de poder disparar en cada tick
  mientras el ritmo siga bajo -- le da tiempo a la última tanda de
  workers a hacer efecto antes de sumar otra.

## 2. Tamaño de WriteBatch de Badger hardcodeado en 1000
(`internal/caimandb/ops_insert.go`)

El commit de `GENERATE` flusheaba el `badger.WriteBatch` cada 1000
documentos procesados, sin forma de ajustarlo.

**Fix:**
- Nuevo `Config.GenerateBatchFlushSize` (default 2000 si no se setea,
  ver `defaultGenerateBatchFlushSize` / `generateBatchFlushSize()` en
  `ops_insert.go`). El valor de 1000 pasó a ser el default previo; ahora
  es configurable y el default subió a 2000.

## Qué NO se tocó (fuera de alcance de este fix)

Siguen existiendo **tres** sistemas de autoscale independientes que no se
coordinan entre sí:
1. El vigilante de `GENERATE` (arreglado acá).
2. `AutoScaleManager` en `shard_manager.go` (sube/baja shards).
3. `IntelligentEngine.autoScaleLoop` en `engine_core.go` (cada
   `AutoScaleInterval`, reajusta otro parámetro de recursos).

Unificarlos en un solo controlador (o al menos pausar (2) y (3) mientras
(1) está activamente escalando) es un cambio de arquitectura más grande,
no incluido acá.

## Cómo probar

```
GENERATE 200000 documents INTO mydb.products
```
sin `WORKERS` explícito, y comparar `docs/sec` final y el número de
workers reportado en el mensaje `[auto-scaled X -> Y workers]` contra el
comportamiento previo. En una máquina de 2 núcleos, Y no debería superar
`GenerateAutoScaleMaxMultiplier * GOMAXPROCS` (8 por default).

Para forzar un valor fijo (desactiva el vigilante):
```
GENERATE 200000 documents INTO mydb.products WORKERS 4
```
