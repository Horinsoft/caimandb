# Fix: errores de metadatos (`block.json`) al renombrar DB o BLOCK (julio 2026)

**Importante:** igual que en `corruption-fixes-2026-07.md`, este entorno no
tuvo acceso a un toolchain de Go instalado, así que estos cambios se
revisaron a mano (balance de llaves/paréntesis, tipos, imports, ciclos de
paquetes) pero **no se compilaron**. Ejecuta `go build ./... && go vet ./...`
y la suite de tests antes de desplegar.

## Síntoma

`SHOW BLOCKS` (y cualquier `DESCRIBE BLOCK`) podía fallar para un bloque
concreto con un error como:

```
actors (error: open data\movies_db\actors\__meta\block.json: The system
cannot find the file specified.)
```

especialmente después de un `RENAME DATABASE` o `RENAME BLOCK`.

## Causa raíz

`DescribeBlock` llamaba a `DirectoryManager.LoadBlockMeta`, que simplemente
lee `block.json` y **propaga el error tal cual** si el archivo no existe o
no se puede parsear -- sin ningún camino de recuperación. Eso pasaba en más
de un caso:

- **Bloques creados antes de que existiera el archivo de metadatos**, o
  cuyo `__meta/block.json` se perdió/corrompió por cualquier motivo externo
  a CaimanDB.
- **Cualquier operación de escritura (insert/update/delete) sobre un bloque
  cuyo `block.json` ya faltaba**: `ops_insert.go` / `ops_local.go`
  actualizaban los contadores con el patrón `if meta, err :=
  LoadBlockMeta(...); err == nil { ... }` -- si `err != nil` una sola vez,
  esa rama nunca hacía nada, así que el archivo **nunca se volvía a crear**
  y el bloque quedaba "roto" para siempre.
- **`RENAME BLOCK` / `RENAME DATABASE`**: mover el directorio no reescribe
  el *contenido* de `block.json` -- sus campos `Name`/`DB` seguían
  apuntando al nombre viejo indefinidamente. `RenameDB` en particular nunca
  tocaba los `block.json` de los bloques que vivían debajo de la base de
  datos renombrada.

## Fix

Se añadió `DirectoryManager.EnsureBlockMeta(dbName, blockName)`
(`internal/caimandb/storage/directory.go`), que reemplaza las llamadas a
`LoadBlockMeta` en todos los puntos donde antes se leía como si el archivo
siempre fuese a existir y estar al día:

1. Si `block.json` falta o no se puede parsear (y el directorio del bloque
   sí existe), lo **reconstruye** con `CreatedAt` tomado del `mtime` del
   directorio y `SizeBytes` recalculado leyendo el tamaño real en disco
   (`DocCount` arranca en 0 y se autocorrige en la siguiente
   inserción/actualización/borrado, que ya pasan por este mismo método).
2. Si `block.json` carga bien pero `Name`/`DB` no coinciden con el nombre
   actual (típico tras un rename), los corrige y regraba el archivo.
3. `SaveBlockMeta` ahora crea `__meta/` con `MkdirAll` si no existe (igual
   que ya hacía `SaveDBMeta`), para que la reconstrucción del punto 1 no
   falle en un bloque al que nunca se le creó ese subdirectorio.

Puntos actualizados para usar `EnsureBlockMeta` en vez de `LoadBlockMeta`:

- `DescribeBlock` (`ops_block.go`) -- ya no falla `SHOW BLOCKS`/`DESCRIBE
  BLOCK` para un bloque con metadatos ausentes o corruptos.
- `renameBlockLocal` (`ops_local.go`) -- tras mover el directorio y migrar
  las claves, repara/corrige el `block.json` incondicionalmente (antes solo
  lo tocaba si `LoadBlockMeta` ya había tenido éxito).
- `RenameDB` (`ops_database.go`) -- tras renombrar la base de datos, recorre
  todos sus bloques y llama a `EnsureBlockMeta` en cada uno, así el campo
  `DB` de cada `block.json` deja de quedar apuntando al nombre viejo.
- Los tres sitios de contadores en caliente (`insertLocal` en
  `ops_local.go`, `updateLocal`, `deleteLocal`, y el insert masivo en
  `ops_insert.go`) -- para que un `block.json` ausente se repare la primera
  vez que se escribe, en vez de quedar roto para siempre.

## Qué no se tocó

- `LoadBlockMeta` en sí se dejó intacta (sigue siendo una lectura "pura" que
  puede fallar) porque `EnsureBlockMeta` la usa internamente y algún
  llamador futuro podría querer distinguir "no existe" de "está bien" sin
  reparar nada.
- No se auditaron otros metadatos fuera de `block.json`/`db.json` (p. ej.
  metadatos de índices secundarios) en este pase.
