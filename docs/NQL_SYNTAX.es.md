# CaimanDB NQL — Referencia Completa de Sintaxis (Español)

Otros idiomas: [English](./NQL_SYNTAX.en.md) · [Deutsch](./NQL_SYNTAX.de.md)

Este documento cubre todos los comandos que entiende el motor de
consola/consultas de CaimanDB (NQL): estructura completa, todas las
variantes y ejemplos trabajados.

**Convenciones usadas a continuación:**
- `<bloque>` — un nombre de bloque, opcionalmente como `<db>.<bloque>` para una referencia entre bases de datos.
- `<db>` — un nombre de base de datos.
- `[...]` — parte opcional. `<a|b>` — elegir una. `...` — repetible.
- Los tokens de las sentencias no distinguen mayúsculas/minúsculas (`insert`,
  `INSERT`, `Insert` funcionan igual); esta referencia usa MAYÚSCULAS para
  las palabras clave por convención.

## Índice
1. Comandos de bases de datos
2. Comandos de bloques
3. INSERT — todas las variantes
4. FIND / GET — consultas
5. SEARCH — texto completo
6. UPDATE
7. DELETE
8. Agregaciones (COUNT/SUM/AVG/...)
9. GROUP BY
10. Transacciones ACID
11. TURBO / carga masiva (BULK)
12. JOIN
13. RELATE
14. AUTORELATIONS
15. Vistas (VIEWS)
16. EXPORT / IMPORT
17. Gestión de usuarios
18. Gestión de shards
19. Cluster
20. Navegación y sistema
21. Operadores de filtro
22. Ejemplo completo

---

### 1. Comandos de bases de datos

```
CREATE DB <nombre>                  Crea una nueva base de datos
DROP DB <nombre>                    Elimina una base de datos
RENAME DB <viejo> TO <nuevo>        Renombra una base de datos
USE <nombre>                        Cambia a esa base de datos (sesión actual)
SHOW DBS                            Lista todas las bases de datos (bloques/docs/tamaño)
SHOW DBS <nombre> [<nombre2> ...]   Lista sólo las bases nombradas
INFO DB <nombre>                    Muestra detalles de la base de datos
DESCRIBE DB <nombre>                Muestra el esquema (tipos de campo inferidos)
STATS DB [<nombre>]                 Muestra estadísticas de la base de datos
SIZE DB [<nombre>]                  Muestra el tamaño en disco
COMPACT <db>                        Ejecuta recolección de basura / libera espacio
ANALYZE DB [<nombre>]               Analiza el rendimiento de la base de datos
OPTIMIZE DB [<nombre>]              Optimiza la base de datos (índices, storage tiers)
BACKUP <db> TO <archivo>            Respalda la base de datos a un archivo
RESTORE <db> FROM <archivo>         Restaura la base de datos desde un respaldo
```

```sql
CREATE DB tienda
USE tienda
SHOW DBS
SHOW DBS tienda analytics
INFO DB tienda
STATS DB tienda
BACKUP tienda TO "tienda_2026-08.bak"
RESTORE tienda FROM "tienda_2026-08.bak"
DROP DB tienda_vieja
```

### 2. Comandos de bloques

Un bloque es el equivalente de CaimanDB a una tabla/colección: un contenedor
de documentos sin esquema fijo, dentro de una base de datos.

```
CREATE BLOCK [<db>] <nombre>          Crea un nuevo bloque
DROP BLOCK [<db>] <nombre>            Elimina un bloque
RENAME BLOCK [<db>] <viejo> TO <nuevo>  Renombra un bloque
SHOW BLOCKS [<db>]                    Lista todos los bloques (docs/tamaño/shards)
SHOW BLOCKS <db> <nombre> [<nombre2>] Lista sólo los bloques nombrados
INFO BLOCK [<db>] <nombre>            Muestra detalles del bloque
DESCRIBE BLOCK [<db>] <nombre>        Muestra el esquema del bloque
EMPTY BLOCK [<db>] <nombre>           Elimina todos los documentos del bloque
CLEAR [<db>] <nombre>                 Alias de EMPTY BLOCK
ANALYZE BLOCK [<db>] <nombre>         Analiza el rendimiento del bloque
OPTIMIZE BLOCK [<db>] <nombre>        Optimiza el bloque
REBUILD BLOCK [<db>] <nombre>         Reconstruye todos los índices
CHECK BLOCK [<db>] <nombre>           Verifica la integridad del bloque
REPAIR BLOCK [<db>] <nombre>          Repara un bloque corrupto
SIZE BLOCK [<db>] <nombre>            Muestra el tamaño en disco del bloque
```

```sql
CREATE BLOCK productos
CREATE BLOCK tienda productos       -- db explícita, sin USE
SHOW BLOCKS
SHOW BLOCKS tienda productos inventario
DESCRIBE BLOCK productos
REBUILD BLOCK productos             -- p.ej. tras cambiar campos indexados
EMPTY BLOCK productos                -- conserva el bloque, borra sus documentos
CLEAR productos                      -- igual que arriba
```

### 3. INSERT — todas las variantes

```
INSERT <bloque> [<id>] <objeto-json>
INSERT <bloque> [<id>] clave: valor, clave2: valor2, ...
INSERT <bloque> [<id>] clave = valor, clave2 = valor2, ...
INSERT <bloque> <doc1>; <doc2>; <doc3>; ...
INSERT <bloque> [<array-json-de-documentos>]
INSERT <bloque> FROM "<archivo.json|archivo.csv>"
INSERT <bloque> GENERATE <n> [WORKERS <w>]
```

**Notas de estructura:**
- `<id>` es opcional y, si se incluye, debe ser el token justo después del
  nombre del bloque, y no debe parecer `{`, `[`, `"`, `NULL`, ni una palabra
  reservada (`FROM`, `GENERATE`, `TO`, `WHERE`, `SET`, `LIMIT`, `ORDER`,
  `SELECT`) — esas se interpretan como el inicio del documento/cláusula en
  vez de como un id.
- Con id explícito, el documento se inserta con ese `_id` exacto (vía
  `insertWithID`) en lugar de uno generado automáticamente.
- Los formatos `clave: valor` y `clave = valor` sirven para documentos planos;
  los valores se tipan automáticamente: lo que parsea como número se vuelve
  número, `{...}` se vuelve objeto anidado, el resto queda como texto recortado.
- Tanto varios documentos separados por `;` como un array JSON de nivel
  superior (`[...]`) insertan un lote en una sola llamada; si se da un id
  personalizado, se aplica sólo al primer documento, el resto recibe ids
  generados.

**Documento JSON:**
```sql
INSERT productos {"nombre": "Teclado", "precio": 49.90, "en_stock": true}
INSERT productos {"usuario": {"nombre": "Juan", "edad": 30}}
```

**Documento JSON con id explícito (el token de id va justo después del bloque):**
```sql
INSERT productos kb001 {"nombre": "Teclado", "precio": 49.90, "en_stock": true}
-- -> Inserted document: kb001 (ID: kb001, shard: shard_7)
```

**Formato Clave:Valor:**
```sql
INSERT productos nombre: "Mouse", precio: 19.90, en_stock: true
INSERT productos mouse001 nombre: "Mouse", precio: 19.90, en_stock: true
```

**Formato Clave=Valor:**
```sql
INSERT productos nombre = "Monitor", precio = 199.00
```

**Varios documentos (separados por punto y coma), en una sola sentencia:**
```sql
INSERT productos {"nombre": "A"}; {"nombre": "B"}; {"nombre": "C"}
INSERT productos nombre: "A"; nombre: "B"; nombre: "C"
```

**Inserción por lote (array JSON):**
```sql
INSERT productos [{"nombre": "A"}, {"nombre": "B"}, {"nombre": "C"}]
```

**Importar desde archivo (bloqueante, lee el archivo completo):**
```sql
INSERT productos FROM "productos.json"
INSERT productos FROM "productos.csv"
```

**GENERATE — datos sintéticos para benchmarking / carga inicial:**
```sql
INSERT productos GENERATE 1000000              -- workers auto-escalados
INSERT productos GENERATE 200000 WORKERS 8     -- número fijo de workers (hasta 64)
```
- Sin `WORKERS <n>`, un vigilante interno (`runRateWatchdog`) mide el ritmo
  cada 2s y suma 2 workers a la vez cuando el ritmo cae por debajo del 85%
  del mejor ritmo visto hasta ahora, con un techo de
  `GOMAXPROCS * GenerateAutoScaleMaxMultiplier` (multiplicador por defecto: 4)
  y un tope absoluto de 64 workers, con un enfriamiento entre subidas.
- Con `WORKERS <n>` explícito, se usa exactamente ese número de workers
  durante toda la corrida (hasta el mismo tope absoluto de 64) y se
  desactiva el vigilante.
- Los `GENERATE` grandes activan BULK MODE automáticamente mientras duran.

### 4. FIND / GET — consultas

```
FIND <bloque> [SELECT <campo>[,<campo>...] | <campo> AS <alias> | COUNT(<campo>) AS <alias> | <campo>/<n> AS <alias>]
              [WHERE <condición>]
              [GROUP BY <campo>[,<campo>...]] [HAVING <condición>]
              [ORDER <campo>[:ASC|:DESC][,<campo>...]]
              [LIMIT <n>] [OFFSET <n>]
              [--type:table]

GET <bloque> <id>
GET <bloque> @ <id>

EXPLAIN FIND ...     -- ejecuta la consulta de verdad e informa qué pasó
EXPLAIN SEARCH ...   -- (como EXPLAIN ANALYZE, no sólo un estimado del plan)
```

**Búsqueda básica / por id:**
```sql
FIND productos WHERE _id = "abc123"
GET productos abc123
GET productos @ abc123
```

**Filtros (ver la tabla completa de operadores en el §21):**
```sql
FIND productos WHERE precio > 20 AND en_stock = true
FIND productos WHERE nombre LIKE "%clado%" OR nombre CONTAINS "Mon"
FIND productos WHERE precio BETWEEN 10 AND 100
FIND productos WHERE estado IN ("activo", "pendiente")
FIND productos WHERE etiquetas IN ["go", "database", "nosql"]
```

**Agrupación/precedencia con paréntesis y NOT (sólo FIND/SEARCH):**
```sql
FIND productos WHERE (estado = "activo" OR estado = "prueba") AND precio >= 18
FIND productos WHERE NOT (estado = "baneado" OR estado = "suspendido")
```

**Proyección (SELECT):**
```sql
FIND productos SELECT nombre, precio WHERE precio > 20
```

**Campos calculados en SELECT — COUNT(campo), aritmética simple AS alias:**
```sql
FIND peliculas SELECT titulo, COUNT(actores) as total_actores, anio
FIND peliculas SELECT titulo, duracion_minutos / 60 as horas
```

**GROUP BY / HAVING (sólo FIND):**
```sql
FIND peliculas SELECT titulo, COUNT(actores) as total_actores, anio
  WHERE anio >= 2000
  GROUP BY titulo, anio
  HAVING COUNT(actores) >= 5
```

**Filtrado a través de un alias de RELATE (ver §13):**
```sql
RELATE peliculas USE directores
FIND peliculas SELECT titulo, directores.nombre
  WHERE directores.nombre == "Christopher Nolan"
```

**Orden, paginación, salida en tabla:**
```sql
FIND productos ORDER nombre, precio:DESC WHERE precio > 18
FIND productos WHERE precio > 18 LIMIT 50 OFFSET 100
FIND productos WHERE precio > 18 --type:table
```

**EXPLAIN:**
```sql
EXPLAIN FIND productos WHERE precio > 18 ORDER precio:DESC LIMIT 10
EXPLAIN SEARCH productos "teclado inalambrico"
```

### 5. SEARCH — texto completo

```
SEARCH <bloque> "<texto>" [EXACT | FUZZY]
                           [WITH SCORE] [WITH MATCHES]
                           [WHERE <condición>] [LIMIT <n>] [ORDER <campo>]
```

```sql
SEARCH productos "teclado inalambrico"
SEARCH productos "frase exacta" EXACT
SEARCH productos "~teclao" FUZZY
SEARCH productos "teclado" WITH SCORE WITH MATCHES
SEARCH productos "+debe_incluir -debe_excluir opcional"
SEARCH productos "teclado" WHERE precio > 18 LIMIT 50 ORDER nombre
```

### 6. UPDATE

```
UPDATE <bloque> WHERE <condición> SET <campo> = <valor>[, <campo2> = <valor2> ...]
UPDATE <bloque> WHERE <condición> INC <campo> = <n>
UPDATE <bloque> WHERE <condición> DEC <campo> = <n>
UPDATE <bloque> WHERE <condición> PUSH <campo> = <valor>
UPDATE <bloque> WHERE <condición> PULL <campo> = <valor>
UPDATE ALL <bloque> SET <campo> = <valor>[, ...]
```

- `SET` reemplaza valores de campo. `INC`/`DEC` suman/restan un número a un
  campo numérico. `PUSH`/`PULL` agregan/quitan un valor de un campo array.
- `UPDATE ALL` aplica a todos los documentos del bloque, sin necesitar `WHERE`.
- Las cláusulas pueden combinar varias asignaciones y funciones como `now()`.

```sql
UPDATE productos WHERE _id = "kb001" SET nombre = "Teclado Mecánico", precio = 55
UPDATE productos WHERE _id = "kb001" INC vistas = 1
UPDATE productos WHERE _id = "kb001" DEC stock = 5
UPDATE productos WHERE _id = "kb001" PUSH etiquetas = "oferta"
UPDATE productos WHERE _id = "kb001" PULL etiquetas = "descontinuado"
UPDATE ALL productos SET estado = "archivado"
UPDATE productos WHERE estado = "borrador" SET estado = "publicado", publicado_en = now()
```

### 7. DELETE

```
DELETE <bloque> WHERE <condición>
DELETE ALL <bloque>
EMPTY BLOCK [<db>] <nombre>     -- alias de DELETE ALL
CLEAR [<db>] <nombre>           -- alias de DELETE ALL / EMPTY BLOCK
```

```sql
DELETE productos WHERE _id = "kb001"
DELETE productos WHERE precio < 5 OR en_stock = false
DELETE ALL productos
```

### 8. Agregaciones

```
COUNT  <bloque> [WHERE <condición>]
SUM    <bloque> <campo> [WHERE <condición>]
AVG    <bloque> <campo> [WHERE <condición>]
MIN    <bloque> <campo> [WHERE <condición>]
MAX    <bloque> <campo> [WHERE <condición>]
MEDIAN <bloque> <campo> [WHERE <condición>]
MODE   <bloque> <campo> [WHERE <condición>]
STDDEV <bloque> <campo> [WHERE <condición>]
```

```sql
COUNT productos WHERE en_stock = true
SUM pedidos monto WHERE estado = "completado"
AVG productos precio WHERE categoria = "electronica"
MIN productos precio
MAX productos precio
MEDIAN salarios monto
MODE productos categoria
STDDEV puntajes valor
```

### 9. GROUP BY

```
GROUP <bloque> BY <campo> [COUNT | SUM | AVG | MIN | MAX] [<campo>] [WHERE <condición>]
```

```sql
GROUP usuarios BY ciudad COUNT
GROUP pedidos BY estado SUM monto
GROUP productos BY categoria AVG precio WHERE precio > 10
GROUP logs BY nivel COUNT WHERE timestamp > "2024-01-01"
```

### 10. Transacciones ACID

```
BEGIN [<db> <bloque>]
  <sentencias INSERT|UPDATE|DELETE...>
COMMIT
ROLLBACK | ABORT

TX STATUS       Muestra detalles de la transacción actual
TX LIST         Lista las transacciones activas
TX ISOLATION    Muestra el nivel de aislamiento
```

Niveles de aislamiento (se configuran, no se eligen por sentencia):
`read_committed`, `repeatable_read` (por defecto), `serializable`.

```sql
BEGIN tienda productos
  INSERT productos {"nombre": "Webcam", "precio": 39.90}
  UPDATE productos WHERE _id = "kb001" SET precio = 45
  DELETE productos WHERE _id = "viejo001"
COMMIT
```
```sql
BEGIN tienda productos
  INSERT productos {"nombre": "Mala idea"}
ROLLBACK
```

### 11. TURBO / carga masiva (BULK)

```
BULK MODE ON             Ventanas de lote más amplias, fsync del WAL relajado
BULK MODE OFF             Restaura el comportamiento normal de baja latencia
BULK STATUS               Muestra estadísticas del motor turbo (pool, batching)

IMPORT <bloque> FROM FILE '<ruta>' [FORMAT NDJSON|ARRAY] [BATCH <n>]
```

- Los `INSERT` normales ya auto-agrupan escrituras concurrentes al mismo
  bloque; `BULK MODE` amplía eso aún más para cargas grandes.
- `IMPORT ... FROM FILE` transmite un archivo hacia `<bloque>` en lotes
  grandes (20000 docs/lote por defecto) sin mantener el archivo completo en
  memoria. `FORMAT NDJSON` (por defecto) lee un objeto JSON por línea;
  `FORMAT ARRAY` lee un único array JSON de nivel superior. Una ruta con
  sufijo `.gz` se descomprime al vuelo. Corre bajo BULK MODE automáticamente
  mientras dure la carga.

```sql
BULK MODE ON
INSERT productos GENERATE 2000000
BULK MODE OFF

IMPORT productos FROM FILE '/data/productos.ndjson' FORMAT NDJSON BATCH 50000
IMPORT productos FROM FILE '/data/productos.json.gz' FORMAT ARRAY
BULK STATUS
```

### 12. JOIN

```
JOIN <bloque1> WITH <bloque2> ON <bloque1>.<campo> = <bloque2>.<campo>
```

```sql
JOIN pedidos WITH clientes ON pedidos.cliente_id = clientes._id
JOIN posts WITH usuarios ON posts.autor_id = usuarios._id
```

### 13. RELATE

Registra una sola vez cómo se relaciona un bloque con otros bloques
(opcionalmente en otras bases de datos); luego `FIND` resuelve la relación
automáticamente en vez de repetir una condición `JOIN` en cada consulta.

```
RELATE <bloque> USE <destino1>[,<destino2>,...]
```

- Cada destino es un nombre de bloque simple (misma base de datos) o
  `db.bloque` (entre bases de datos).
- Convención de coincidencia: el documento origen debe tener un campo
  `<destino>_id` (singular del nombre del bloque destino) con el id del
  documento destino — un solo id para relaciones uno-a-uno/muchos-a-uno, o
  un array de ids para uno-a-muchos.
- Ya relacionado, se seleccionan campos del destino con notación de punto, o
  el alias simple para todo el documento relacionado.

```sql
RELATE peliculas USE directores,actores,generos
RELATE ventas USE crm.clientes,inventario.productos,contabilidad.facturas

FIND peliculas SELECT titulo,directores.nombre,actores.nombre
FIND ventas SELECT clientes.nombre,productos.nombre,facturas.total
```

### 14. AUTORELATIONS (autorelaciones automáticas y temporales)

CaimanDB vigila su propio acceso de lectura: cuando el mismo usuario lee el
mismo documento repetidamente en una ventana corta (por defecto: 5 lecturas
en 10 minutos), crea automáticamente una autorelación entre ese usuario y el
documento — sin necesidad de `RELATE`. Cada autorelación lleva
`access_count`/`last_seen`, un puntaje `relevance`, y una pequeña muestra
`key_metadata` de los campos del documento.

A diferencia de `RELATE` (explícita, permanente), las autorelaciones son
temporales: cada acceso adicional adelanta su expiración (TTL por defecto:
24h); cuando un documento deja de ser leído por ese usuario, la relación
expira y una limpieza en segundo plano la elimina. El grafo resultante es
bipartito y dirigido (`usuario -> documento leído`).

```
SHOW AUTORELATIONS <bloque>
    [FROM <id>] [TO <id>] [DEPTH <n>] [DIRECTION IN|OUT|BOTH]
    [FORMAT TABLE|TREE|GRAPH|JSON]
    [WHERE|FILTER <expresión>]
    [ORDER BY DEGREE|ID|NAME|ACCESS_COUNT|RELEVANCE|LAST_SEEN|FIRST_SEEN [ASC|DESC]]
    [LIMIT <n>] [OFFSET <n>]
    [STATS] [PATHS] [ORPHANS] [CYCLES] [BROKEN] [SUMMARY] [VERBOSE];
```

| Modificador | Significado |
|---|---|
| `FROM <id>` | Punto de partida: id de documento o usuario (autodetectado) |
| `TO <id>` | Sólo relaciones que toquen este id como el otro extremo |
| `DEPTH <n>` | Saltos a recorrer desde FROM (por defecto 1) |
| `DIRECTION` | `OUT` = lecturas hacia afuera de un usuario; `IN` = lectores hacia un doc; `BOTH` (por defecto) |
| `FORMAT` | `TABLE` (por defecto), `TREE` (necesita FROM), `GRAPH` (lista de adyacencia), `JSON` |
| `WHERE`/`FILTER` | Condición sobre `doc_id`, `user_id`, `access_count`, `relevance`, `last_seen`, `first_seen` |
| `ORDER BY` | `DEGREE`, `ID`, `NAME`, `ACCESS_COUNT`, `RELEVANCE`, `LAST_SEEN`, `FIRST_SEEN` (+`ASC`/`DESC`) |
| `LIMIT`/`OFFSET` | Paginación sobre el resultado final ya ordenado |
| `STATS`/`SUMMARY` | Antepone un reporte agregado |
| `PATHS` | Muestra el recorrido desde FROM como árbol indentado |
| `ORPHANS` | Sólo pares aislados |
| `CYCLES` | Sólo relaciones que cierran un ciclo |
| `BROKEN` | Sólo relaciones cuyo documento fue borrado después |
| `VERBOSE` | Agrega metadatos completos (first_seen/expires) |

```sql
SHOW AUTORELATIONS productos;
SHOW AUTORELATIONS productos FROM p_042;
SHOW AUTORELATIONS productos FROM p_042 DEPTH 3 DIRECTION BOTH FORMAT TREE;
SHOW AUTORELATIONS productos STATS;
SHOW AUTORELATIONS productos WHERE access_count > 10 ORDER BY DEGREE LIMIT 20;
SHOW AUTORELATIONS productos FROM p145 DEPTH 6 DIRECTION BOTH
  WHERE relevance >= 0.75 ORDER BY ACCESS_COUNT DESC LIMIT 100
  FORMAT TREE STATS VERBOSE;
SHOW AUTORELATIONS productos CYCLES;
SHOW AUTORELATIONS productos BROKEN;
```

### 15. Vistas (VIEWS)

```
VIEW CREATE <nombre> AS FIND <bloque> WHERE <condición>
VIEW DROP <nombre>
VIEW SHOW
VIEW INFO <nombre>
<nombre_de_vista>                -- ejecuta la vista
```

```sql
VIEW CREATE usuarios_activos AS FIND usuarios WHERE activo = true
VIEW SHOW
VIEW INFO usuarios_activos
usuarios_activos
VIEW DROP usuarios_activos
```

### 16. Exportar / Importar

`EXPORT` siempre escribe **ambos** un archivo `.csv` y uno `.json` dentro de
`<data_root>/backups/`, usando el nombre base que le des. Cada
fila/documento exportado incluye tanto `_id` como `id`.

```
EXPORT <bloque> [WHERE <condición>] TO "<archivo>"
IMPORT <bloque> FROM "<archivo.json>"        -- también busca dentro de backups/
IMPORT <bloque> FROM "<archivo.csv>"
```

```sql
EXPORT productos TO "export_productos"
-- -> escribe backups/export_productos.csv y backups/export_productos.json
EXPORT productos WHERE precio > 18 TO "productos_caros"

IMPORT productos FROM "export_productos.json"
IMPORT productos FROM "export_productos.csv"
```

### 17. Gestión de usuarios

```
CREATE USER <nombre> PASSWORD "<clave>" [ROLE admin|readwrite|readonly]
DROP USER <nombre>
SHOW USERS
```

```sql
CREATE USER analista PASSWORD "s3cret!" ROLE readonly
SHOW USERS
DROP USER analista
```

### 18. Gestión de shards

```
SHARD STATUS
SHARD REBALANCE
SHARD SCALE <db> <shards>
```

```sql
SHARD STATUS
SHARD REBALANCE
SHARD SCALE tienda 32
```

### 19. Gestión de cluster

```
CLUSTER STATUS
```

### 20. Navegación y sistema

```
PWD               Muestra la ruta actual
LS                Lista bases de datos
LS <db>           Lista bloques en una base de datos
CD <db>           Cambia a una base de datos
TREE              Muestra el árbol de directorios completo
STATUS            Muestra el estado del sistema
HEALTH            Muestra el chequeo de salud
VERSION           Muestra la versión
PING              Hace ping al motor
HELP              Muestra la ayuda
EXIT, QUIT        Sale de la consola
```

### 21. Operadores de filtro (usables en cualquier `WHERE`)

| Operador | Significado |
|---|---|
| `=`, `==` | Igual |
| `!=`, `<>` | Distinto |
| `>`, `<` | Mayor que / Menor que |
| `>=`, `<=` | Mayor o igual / Menor o igual |
| `LIKE` | Coincidencia de patrón (comodín `%`) |
| `CONTAINS` | Contiene subcadena |
| `EXISTS` | El campo existe |
| `IN` | Valor en la lista |
| `NOT IN` | Valor no en la lista |
| `BETWEEN` | Rango entre dos valores |
| `STARTS WITH` | El texto empieza con |
| `ENDS WITH` | El texto termina con |
| `AND` | Y lógico (por defecto si se omite) |
| `OR` | O lógico |

### 22. Ejemplo completo

```sql
CREATE DB tienda
USE tienda
CREATE BLOCK productos

INSERT productos kb001 {"nombre": "Teclado", "precio": 49.90, "en_stock": true}
INSERT productos {"nombre": "Mouse", "precio": 19.90, "en_stock": true}
INSERT productos nombre: "Monitor", precio: 199.00, en_stock: true

FIND productos WHERE precio > 20
FIND productos WHERE nombre LIKE "%clado%" ORDER precio:DESC
FIND productos WHERE precio BETWEEN 20 AND 200 SELECT nombre, precio

SEARCH productos "teclado" WITH SCORE

UPDATE productos WHERE _id = "kb001" SET precio = 45
UPDATE productos WHERE precio < 30 INC precio = 1

COUNT productos WHERE en_stock = true
AVG productos precio
GROUP productos BY en_stock COUNT

DELETE productos WHERE _id = "kb001"

BEGIN tienda productos
  INSERT productos {"nombre": "Webcam", "precio": 39.90}
  UPDATE productos WHERE nombre = "Mouse" SET precio = 17.90
COMMIT

VIEW CREATE productos_baratos AS FIND productos WHERE precio < 25
productos_baratos

EXPORT productos TO "respaldo_productos"
STATUS
STATS DB tienda
```
