# CaimanDB NQL — Syntax & Examples / Sintaxis y Ejemplos / Syntax und Beispiele

Reference for CaimanDB's query language (NQL). Every section below is repeated in
**English**, **Español**, and **Deutsch**. `<block>` means a block name (optionally
prefixed with `<db>.`), `<db>` means a database name.

---

## 🇬🇧 English

### 1. Databases & Blocks

```sql
CREATE DB shop
USE shop
CREATE BLOCK products
SHOW DBS
SHOW BLOCKS shop
DESCRIBE BLOCK shop products
DROP BLOCK shop products
```

### 2. Insert

```sql
-- JSON
INSERT products {"name": "Keyboard", "price": 49.90, "in_stock": true}

-- key: value
INSERT products name: "Mouse", price: 19.90, in_stock: true

-- key = value
INSERT products name = "Monitor", price = 199.00

-- Multiple documents (semicolon separated)
INSERT products {"name": "A"}; {"name": "B"}; {"name": "C"}

-- Batch (JSON array)
INSERT products [{"name": "A"}, {"name": "B"}, {"name": "C"}]

-- From file
INSERT products FROM "products.json"
INSERT products FROM "products.csv"
```

### 3. Generate synthetic data (benchmarking / seeding)

```sql
-- Auto-scaled worker count, tuned live to keep docs/sec steady
INSERT products GENERATE 1000000

-- Fixed worker count (disables the auto-scale watchdog)
INSERT products GENERATE 200000 WORKERS 8
```

`GENERATE` without an explicit `WORKERS <n>` uses `runRateWatchdog`: it samples
throughput every 2s and adds workers if the rate drops below 85% of the best rate
seen so far, capped at `GOMAXPROCS * GenerateAutoScaleMaxMultiplier` (default 4x)
and a hard ceiling of 64. An explicit `WORKERS <n>` can go up to 64 directly and
disables that automatic behavior.

### 4. Find / Query

```sql
FIND products WHERE _id = "abc123"
FIND products WHERE price > 20 AND in_stock = true
FIND products WHERE name LIKE "%board%" OR name CONTAINS "Mon"
FIND products WHERE price BETWEEN 10 AND 100
FIND products WHERE name IN ("Mouse", "Keyboard")

-- Grouping/precedence
FIND products WHERE (in_stock = true OR price < 5) AND price > 0

-- Projection
FIND products SELECT name, price WHERE price > 20

-- Computed SELECT fields
FIND movies SELECT title, COUNT(actors) as actors_count, year
FIND movies SELECT title, duration_minutes / 60 as hours

-- GROUP BY / HAVING
FIND movies SELECT title, COUNT(actors) as actors_count, year
  WHERE year >= 2000
  GROUP BY title, year
  HAVING COUNT(actors) >= 5

-- Sorting, pagination
FIND products ORDER price:DESC WHERE price > 0
FIND products WHERE price > 0 LIMIT 50 OFFSET 100

-- Table output / EXPLAIN (runs for real, like EXPLAIN ANALYZE)
FIND products WHERE price > 0 --type:table
EXPLAIN FIND products WHERE price > 0 ORDER price:DESC LIMIT 10

-- Get by id
GET products abc123
GET products @ abc123
```

### 5. Update / Delete

```sql
UPDATE products WHERE _id = "abc" SET name = "New name", price = 25
UPDATE products WHERE _id = "abc" INC views = 1
UPDATE products WHERE _id = "abc" DEC stock = 5
UPDATE products WHERE _id = "abc" PUSH tags = "sale"
UPDATE products WHERE _id = "abc" PULL tags = "discontinued"
UPDATE ALL products SET status = "archived"

DELETE products WHERE _id = "abc"
DELETE products WHERE price < 5 OR in_stock = false
DELETE ALL products
```

### 6. Relations

```sql
RELATE movies USE directors
FIND movies SELECT title, directors.name
  WHERE directors.name == "Christopher Nolan"

RELATE sales USE crm.customers, inventory.products
```

### 7. Bulk loading / performance mode

```sql
BULK MODE ON    -- wider batch windows, relaxed WAL fsync policy
-- ... large INSERT/GENERATE/FROM here ...
BULK MODE OFF   -- restore normal low-latency behavior
```

---

## 🇪🇸 Español

### 1. Bases de datos y bloques

```sql
CREATE DB tienda
USE tienda
CREATE BLOCK productos
SHOW DBS
SHOW BLOCKS tienda
DESCRIBE BLOCK tienda productos
DROP BLOCK tienda productos
```

### 2. Insertar

```sql
-- JSON
INSERT productos {"nombre": "Teclado", "precio": 49.90, "en_stock": true}

-- clave: valor
INSERT productos nombre: "Mouse", precio: 19.90, en_stock: true

-- clave = valor
INSERT productos nombre = "Monitor", precio = 199.00

-- Varios documentos (separados por punto y coma)
INSERT productos {"nombre": "A"}; {"nombre": "B"}; {"nombre": "C"}

-- Lote (array JSON)
INSERT productos [{"nombre": "A"}, {"nombre": "B"}, {"nombre": "C"}]

-- Desde archivo
INSERT productos FROM "productos.json"
INSERT productos FROM "productos.csv"
```

### 3. Generar datos sintéticos (benchmarking / carga inicial)

```sql
-- Número de workers auto-escalado, ajustado en vivo para mantener docs/seg estable
INSERT productos GENERATE 1000000

-- Número de workers fijo (desactiva el vigilante de auto-escalado)
INSERT productos GENERATE 200000 WORKERS 8
```

`GENERATE` sin `WORKERS <n>` explícito usa `runRateWatchdog`: mide el ritmo cada
2s y suma workers si cae por debajo del 85% del mejor ritmo visto, con un techo de
`GOMAXPROCS * GenerateAutoScaleMaxMultiplier` (4x por defecto) y un tope absoluto
de 64. Un `WORKERS <n>` explícito puede llegar directo hasta 64 y desactiva ese
comportamiento automático.

### 4. Buscar / consultar

```sql
FIND productos WHERE _id = "abc123"
FIND productos WHERE precio > 20 AND en_stock = true
FIND productos WHERE nombre LIKE "%clado%" OR nombre CONTAINS "Mon"
FIND productos WHERE precio BETWEEN 10 AND 100
FIND productos WHERE nombre IN ("Mouse", "Teclado")

-- Agrupación/precedencia
FIND productos WHERE (en_stock = true OR precio < 5) AND precio > 0

-- Proyección
FIND productos SELECT nombre, precio WHERE precio > 20

-- Campos calculados en SELECT
FIND peliculas SELECT titulo, COUNT(actores) as total_actores, anio
FIND peliculas SELECT titulo, duracion_minutos / 60 as horas

-- GROUP BY / HAVING
FIND peliculas SELECT titulo, COUNT(actores) as total_actores, anio
  WHERE anio >= 2000
  GROUP BY titulo, anio
  HAVING COUNT(actores) >= 5

-- Orden, paginación
FIND productos ORDER precio:DESC WHERE precio > 0
FIND productos WHERE precio > 0 LIMIT 50 OFFSET 100

-- Salida en tabla / EXPLAIN (ejecuta de verdad, como EXPLAIN ANALYZE)
FIND productos WHERE precio > 0 --type:table
EXPLAIN FIND productos WHERE precio > 0 ORDER precio:DESC LIMIT 10

-- Obtener por id
GET productos abc123
GET productos @ abc123
```

### 5. Actualizar / eliminar

```sql
UPDATE productos WHERE _id = "abc" SET nombre = "Nuevo nombre", precio = 25
UPDATE productos WHERE _id = "abc" INC vistas = 1
UPDATE productos WHERE _id = "abc" DEC stock = 5
UPDATE productos WHERE _id = "abc" PUSH etiquetas = "oferta"
UPDATE productos WHERE _id = "abc" PULL etiquetas = "descontinuado"
UPDATE ALL productos SET estado = "archivado"

DELETE productos WHERE _id = "abc"
DELETE productos WHERE precio < 5 OR en_stock = false
DELETE ALL productos
```

### 6. Relaciones

```sql
RELATE peliculas USE directores
FIND peliculas SELECT titulo, directores.nombre
  WHERE directores.nombre == "Christopher Nolan"

RELATE ventas USE crm.clientes, inventario.productos
```

### 7. Carga masiva / modo de rendimiento

```sql
BULK MODE ON    -- ventanas de lote más amplias, fsync del WAL relajado
-- ... aquí el INSERT/GENERATE/FROM masivo ...
BULK MODE OFF   -- restaura el comportamiento normal de baja latencia
```

---

## 🇩🇪 Deutsch

### 1. Datenbanken und Blöcke

```sql
CREATE DB shop
USE shop
CREATE BLOCK produkte
SHOW DBS
SHOW BLOCKS shop
DESCRIBE BLOCK shop produkte
DROP BLOCK shop produkte
```

### 2. Einfügen

```sql
-- JSON
INSERT produkte {"name": "Tastatur", "preis": 49.90, "auf_lager": true}

-- Schlüssel: Wert
INSERT produkte name: "Maus", preis: 19.90, auf_lager: true

-- Schlüssel = Wert
INSERT produkte name = "Monitor", preis = 199.00

-- Mehrere Dokumente (durch Semikolon getrennt)
INSERT produkte {"name": "A"}; {"name": "B"}; {"name": "C"}

-- Stapel (JSON-Array)
INSERT produkte [{"name": "A"}, {"name": "B"}, {"name": "C"}]

-- Aus Datei
INSERT produkte FROM "produkte.json"
INSERT produkte FROM "produkte.csv"
```

### 3. Synthetische Daten erzeugen (Benchmarking / Befüllung)

```sql
-- Automatisch skalierte Worker-Anzahl, live angepasst für stabile Docs/Sek.
INSERT produkte GENERATE 1000000

-- Feste Worker-Anzahl (deaktiviert den Auto-Scale-Watchdog)
INSERT produkte GENERATE 200000 WORKERS 8
```

`GENERATE` ohne explizites `WORKERS <n>` nutzt `runRateWatchdog`: er misst den
Durchsatz alle 2s und fügt Worker hinzu, wenn die Rate unter 85 % der bisher
besten Rate fällt, begrenzt auf `GOMAXPROCS * GenerateAutoScaleMaxMultiplier`
(Standard 4x) und eine absolute Obergrenze von 64. Ein explizites `WORKERS <n>`
kann direkt bis 64 gehen und deaktiviert dieses automatische Verhalten.

### 4. Suchen / Abfragen

```sql
FIND produkte WHERE _id = "abc123"
FIND produkte WHERE preis > 20 AND auf_lager = true
FIND produkte WHERE name LIKE "%tatur%" OR name CONTAINS "Mon"
FIND produkte WHERE preis BETWEEN 10 AND 100
FIND produkte WHERE name IN ("Maus", "Tastatur")

-- Gruppierung/Vorrang
FIND produkte WHERE (auf_lager = true OR preis < 5) AND preis > 0

-- Projektion
FIND produkte SELECT name, preis WHERE preis > 20

-- Berechnete SELECT-Felder
FIND filme SELECT titel, COUNT(schauspieler) as anzahl_schauspieler, jahr
FIND filme SELECT titel, dauer_minuten / 60 as stunden

-- GROUP BY / HAVING
FIND filme SELECT titel, COUNT(schauspieler) as anzahl_schauspieler, jahr
  WHERE jahr >= 2000
  GROUP BY titel, jahr
  HAVING COUNT(schauspieler) >= 5

-- Sortierung, Paginierung
FIND produkte ORDER preis:DESC WHERE preis > 0
FIND produkte WHERE preis > 0 LIMIT 50 OFFSET 100

-- Tabellenausgabe / EXPLAIN (führt die Abfrage wirklich aus, wie EXPLAIN ANALYZE)
FIND produkte WHERE preis > 0 --type:table
EXPLAIN FIND produkte WHERE preis > 0 ORDER preis:DESC LIMIT 10

-- Nach ID abrufen
GET produkte abc123
GET produkte @ abc123
```

### 5. Aktualisieren / Löschen

```sql
UPDATE produkte WHERE _id = "abc" SET name = "Neuer Name", preis = 25
UPDATE produkte WHERE _id = "abc" INC aufrufe = 1
UPDATE produkte WHERE _id = "abc" DEC lagerbestand = 5
UPDATE produkte WHERE _id = "abc" PUSH tags = "sale"
UPDATE produkte WHERE _id = "abc" PULL tags = "eingestellt"
UPDATE ALL produkte SET status = "archiviert"

DELETE produkte WHERE _id = "abc"
DELETE produkte WHERE preis < 5 OR auf_lager = false
DELETE ALL produkte
```

### 6. Beziehungen

```sql
RELATE filme USE regisseure
FIND filme SELECT titel, regisseure.name
  WHERE regisseure.name == "Christopher Nolan"

RELATE verkaeufe USE crm.kunden, lager.produkte
```

### 7. Massenladen / Performance-Modus

```sql
BULK MODE ON    -- breitere Batch-Fenster, gelockerte WAL-fsync-Richtlinie
-- ... hier der große INSERT/GENERATE/FROM ...
BULK MODE OFF   -- normales Verhalten mit niedriger Latenz wiederherstellen
```
