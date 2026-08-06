# Referencia de NQL (CaimanDB Query Language)

Este documento resume el contenido del comando `HELP` interactivo
(`internal/caimandb/help.go`), organizado por categoría.

## Bases de datos

| Comando | Descripción |
|---|---|
| `CREATE DB <name>` | Crear una base de datos |
| `DROP DB <name>` | Eliminar una base de datos |
| `RENAME DB <old> TO <new>` | Renombrar una base de datos |
| `USE <name>` | Cambiar de base de datos activa |
| `SHOW DBS` | Listar bases de datos |
| `INFO DB <name>` / `DESCRIBE DB <name>` | Detalle / esquema |
| `STATS DB [<name>]` / `SIZE DB [<name>]` | Estadísticas / tamaño |
| `COMPACT <db>` | Recolección de basura |
| `ANALYZE DB` / `OPTIMIZE DB` | Análisis y optimización |
| `BACKUP <db> TO <file>` / `RESTORE <db> FROM <file>` | Copias de seguridad |

## Bloques (colecciones)

| Comando | Descripción |
|---|---|
| `CREATE BLOCK [<db>] <name>` | Crear un bloque |
| `DROP BLOCK [<db>] <name>` | Eliminar un bloque |
| `RENAME BLOCK [<db>] <old> TO <new>` | Renombrar |
| `SHOW BLOCKS [<db>]` | Listar bloques |
| `EMPTY BLOCK [<db>] <name>` / `CLEAR [<db>] <name>` | Vaciar bloque |
| `REBUILD BLOCK` / `CHECK BLOCK` / `REPAIR BLOCK` | Mantenimiento de índices/integridad |

## Documentos — INSERT

```sql
INSERT users {"name": "John", "age": 30}
INSERT users name: "John", age: 30
INSERT users {"name": "John"}; {"name": "Jane"}
INSERT users [{"name": "John"}, {"name": "Jane"}]
INSERT users FROM "file.json"
```

## Documentos — FIND

```sql
FIND users WHERE age > 18 AND status = "active"
FIND users SELECT name, age WHERE age > 18
FIND users ORDER name, age:DESC WHERE age > 18
FIND users WHERE age > 18 LIMIT 50 OFFSET 100
GET users <id>
```

### WHERE con paréntesis, precedencia y NOT (FIND / SEARCH)

`FIND` y `SEARCH` compilan la cláusula `WHERE` a un árbol de expresión
real (AST) en `internal/caimandb/parse/ast.go`, en vez del encadenado
plano izquierda-a-derecha que usan todavía el resto de comandos
(`UPDATE`, `DELETE`, `COUNT`, agregaciones, `VIEW`, admin). Eso permite:

- **Paréntesis** para agrupar y forzar precedencia explícita:

  ```sql
  FIND users WHERE (status = "active" OR status = "trial") AND age >= 18
  ```

- **`AND` con más precedencia que `OR`** (como en SQL): en
  `a = 1 OR b = 2 AND c = 3`, `b = 2 AND c = 3` se evalúa como una
  unidad antes del `OR`. Antes, sin AST, esta consulta se evaluaba
  estrictamente de izquierda a derecha y no había forma de expresar la
  precedencia estándar sin paréntesis explícitos en cada término.

- **`NOT`** delante de una condición o de un grupo:

  ```sql
  FIND users WHERE NOT (status = "banned" OR status = "suspended")
  ```

Los operadores de condición (`=`, `LIKE`, `IN`, `BETWEEN`, `IS NULL`,
...) son los mismos de siempre; lo único nuevo es cómo se combinan.
Para índices, el planificador de consultas sigue usando la
lista plana de condiciones de nivel superior cuando el `WHERE` es una
cadena pura de `AND` (el caso común); si aparece un `OR`, un `NOT` o un
grupo explícito en el nivel superior, simplemente no hay condición
única que indexar y se hace *scan* completo -- sigue siendo correcto,
solo sin ese atajo.

## Documentos — SEARCH (texto completo)

```sql
SEARCH articles "search text"
SEARCH articles "exact phrase" EXACT
SEARCH articles "~fuzzy" FUZZY
SEARCH articles "text" WITH SCORE WITH MATCHES
```

## Documentos — UPDATE / DELETE

```sql
UPDATE users WHERE _id = "abc" SET name = "John", age = 30
UPDATE users WHERE _id = "abc" INC views = 1
UPDATE users WHERE _id = "abc" PUSH tags = "newtag"
UPDATE ALL users SET status = "archived"

DELETE users WHERE age < 18
DELETE ALL users
```

## Agregaciones y GROUP BY

```sql
COUNT users WHERE active = true
SUM orders amount WHERE status = "completed"
AVG products price
GROUP orders BY status SUM amount
```

Funciones disponibles: `COUNT`, `SUM`, `AVG`, `MIN`, `MAX`, `MEDIAN`,
`MODE`, `STDDEV`.

## Transacciones ACID

```sql
BEGIN mydb users
INSERT users {"name": "John"}
COMMIT
-- o ROLLBACK / ABORT

TX STATUS
TX LIST
TX ISOLATION
```

Niveles de aislamiento: `read_committed`, `repeatable_read`
(por defecto), `serializable`.

## JOIN y vistas

```sql
JOIN orders WITH customers ON orders.customer_id = customers._id

VIEW CREATE <name> AS FIND <block> WHERE ...
VIEW DROP <name>
VIEW SHOW
```

## Export / Import

```sql
EXPORT users TO "file.json"
EXPORT users WHERE age > 18 TO "file.csv"
IMPORT users FROM "file.json"
```

## Usuarios, shards y clúster

```sql
CREATE USER alice PASSWORD "..." ROLE readwrite
SHOW USERS

SHARD STATUS
SHARD REBALANCE
SHARD SCALE mydb 8

CLUSTER STATUS
```

## Navegación / sistema

`PWD`, `LS`, `LS <db>`, `CD <db>`, `TREE`, `STATUS`, `HEALTH`,
`VERSION`, `PING`, `HELP`, `EXIT` / `QUIT`.

## Operadores de filtro

`=`/`==`, `!=`/`<>`, `>`, `<`, `>=`, `<=`, `LIKE`, `CONTAINS`,
`EXISTS`, `IN`, `NOT IN`, `BETWEEN`, `STARTS WITH`, `ENDS WITH`,
`AND`, `OR`.
