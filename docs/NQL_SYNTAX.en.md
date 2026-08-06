Aquí está la versión en inglés con todos los enlaces en el índice:

---

# CaimanDB NQL — Complete Syntax Reference (English)

Other languages: [Español](./NQL_SYNTAX.es.md) · [Deutsch](./NQL_SYNTAX.de.md)

This document covers every NQL command CaimanDB's console/query engine
understands: full structure, every variant, and worked examples.

**Conventions used below:**
- `<block>` — a block name, optionally as `<db>.<block>` for a cross-database reference.
- `<db>` — a database name.
- `[...]` — optional part. `<a|b>` — choose one. `...` — repeatable.
- Statement tokens are case-insensitive (`insert`, `INSERT`, `Insert` all work);
  this reference uses UPPERCASE for keywords by convention.

## Table of contents
- [Database commands](#database-commands)
- [Block commands](#block-commands)
- [INSERT — all variants](#insert--all-variants)
- [FIND / GET — query](#find--get--query)
- [SEARCH — full-text](#search--full-text)
- [UPDATE](#update)
- [DELETE](#delete)
- [Aggregations (COUNT/SUM/AVG/...)](#aggregations-countsumavg)
- [GROUP BY](#group-by)
- [ACID Transactions](#acid-transactions)
- [TURBO / BULK loading](#turbo--bulk-loading)
- [JOIN](#join)
- [RELATE](#relate)
- [AUTORELATIONS](#autorelations)
- [Views](#views)
- [EXPORT / IMPORT](#export--import)
- [User management](#user-management)
- [Shard management](#shard-management)
- [Cluster](#cluster)
- [Navigation & system](#navigation--system)
- [Filter operators](#filter-operators)
- [Full worked example](#full-worked-example)

---

### Database commands

```
CREATE DB <name>                    Create a new database
DROP DB <name>                      Delete a database
RENAME DB <old> TO <new>            Rename a database
USE <name>                          Switch to a database (sets the session's current DB)
SHOW DBS                            List all databases (blocks/docs/size)
SHOW DBS <name> [<name2> ...]       List only the named database(s)
INFO DB <name>                      Show database details
DESCRIBE DB <name>                  Show database schema (inferred field types)
STATS DB [<name>]                   Show database statistics
SIZE DB [<name>]                    Show database size on disk
COMPACT <db>                        Run garbage collection / reclaim space
ANALYZE DB [<name>]                 Analyze database performance
OPTIMIZE DB [<name>]                Optimize database (indexes, storage tiers)
BACKUP <db> TO <file>               Backup database to a file
RESTORE <db> FROM <file>            Restore database from a backup file
```

```sql
CREATE DB shop
USE shop
SHOW DBS
SHOW DBS shop analytics
INFO DB shop
STATS DB shop
BACKUP shop TO "shop_2026-08.bak"
RESTORE shop FROM "shop_2026-08.bak"
DROP DB old_shop
```

### Block commands

A block is CaimanDB's equivalent of a table/collection — a named, schemaless
container of documents inside a database.

```
CREATE BLOCK [<db>] <name>          Create a new block
DROP BLOCK [<db>] <name>            Delete a block
RENAME BLOCK [<db>] <old> TO <new>  Rename a block
SHOW BLOCKS [<db>]                  List all blocks (docs/size/shards)
SHOW BLOCKS <db> <name> [<name2>]   List only the named block(s)
INFO BLOCK [<db>] <name>            Show block details
DESCRIBE BLOCK [<db>] <name>        Show block schema
EMPTY BLOCK [<db>] <name>           Delete all documents from the block
CLEAR [<db>] <name>                 Alias for EMPTY BLOCK
ANALYZE BLOCK [<db>] <name>         Analyze block performance
OPTIMIZE BLOCK [<db>] <name>        Optimize block
REBUILD BLOCK [<db>] <name>         Rebuild all indexes
CHECK BLOCK [<db>] <name>           Check block integrity
REPAIR BLOCK [<db>] <name>          Repair a corrupted block
SIZE BLOCK [<db>] <name>            Show block size on disk
```

```sql
CREATE BLOCK products
CREATE BLOCK shop products          -- explicit db, without USE
SHOW BLOCKS
SHOW BLOCKS shop products inventory
DESCRIBE BLOCK products
REBUILD BLOCK products              -- e.g. after changing indexed fields
EMPTY BLOCK products                -- keeps the block, deletes its documents
CLEAR products                      -- same as above
```

### INSERT — all variants

```
INSERT <block> [<id>] <json-object>
INSERT <block> [<id>] key: value, key2: value2, ...
INSERT <block> [<id>] key = value, key2 = value2, ...
INSERT <block> <doc1>; <doc2>; <doc3>; ...
INSERT <block> [<json-array-of-docs>]
INSERT <block> FROM "<file.json|file.csv>"
INSERT <block> GENERATE <n> [WORKERS <w>]
```

**Structure notes:**
- `<id>` is optional and, when present, must be the token right after the block
  name and must not look like `{`, `[`, `"`, `NULL`, or a reserved keyword
  (`FROM`, `GENERATE`, `TO`, `WHERE`, `SET`, `LIMIT`, `ORDER`, `SELECT`) — those
  are parsed as the start of the document/clause instead of an id.
- With an explicit id, the document is inserted with that exact `_id` (via
  `insertWithID`) instead of an auto-generated one.
- `key: value` and `key = value` both work for flat documents; values are
  auto-typed: things that parse as a number become a number, `{...}` becomes a
  nested object, everything else is a trimmed string.
- Multiple `;`-separated documents and a top-level JSON array (`[...]`) both
  insert a batch in one call; if a custom id is given, it's applied only to the
  first document, the rest get generated ids.

**JSON document:**
```sql
INSERT products {"name": "Keyboard", "price": 49.90, "in_stock": true}
INSERT products {"user": {"name": "John", "age": 30}}
```

**JSON document with an explicit id (the id token comes right after the block name):**
```sql
INSERT products kb001 {"name": "Keyboard", "price": 49.90, "in_stock": true}
-- -> Inserted document: kb001 (ID: kb001, shard: shard_7)
```

**Key:Value format:**
```sql
INSERT products name: "Mouse", price: 19.90, in_stock: true
INSERT products mouse001 name: "Mouse", price: 19.90, in_stock: true
```

**Key=Value format:**
```sql
INSERT products name = "Monitor", price = 199.00
```

**Multiple documents (semicolon-separated), same statement:**
```sql
INSERT products {"name": "A"}; {"name": "B"}; {"name": "C"}
INSERT products name: "A"; name: "B"; name: "C"
```

**Batch insert (JSON array):**
```sql
INSERT products [{"name": "A"}, {"name": "B"}, {"name": "C"}]
```

**Import from file (blocking, reads the whole file):**
```sql
INSERT products FROM "products.json"
INSERT products FROM "products.csv"
```

**GENERATE — synthetic data for benchmarking / seeding:**
```sql
INSERT products GENERATE 1000000              -- auto-scaled workers
INSERT products GENERATE 200000 WORKERS 8     -- fixed worker count (up to 64)
```
- Without `WORKERS <n>`, an internal watchdog (`runRateWatchdog`) samples
  throughput every 2s and adds 2 workers at a time whenever the rate drops
  below 85% of the best rate seen so far, capped at
  `GOMAXPROCS * GenerateAutoScaleMaxMultiplier` (default multiplier: 4) and a
  hard ceiling of 64 workers, with a cooldown between increases.
- With an explicit `WORKERS <n>`, that exact worker count is used for the
  whole run (up to the same hard ceiling of 64) and the watchdog is disabled.
- Large `GENERATE` runs automatically switch to BULK MODE for their duration.

### FIND / GET — query

```
FIND <block> [SELECT <field>[,<field>...] | <field> AS <alias> | COUNT(<field>) AS <alias> | <field>/<n> AS <alias>]
             [WHERE <condition>]
             [GROUP BY <field>[,<field>...]] [HAVING <condition>]
             [ORDER <field>[:ASC|:DESC][,<field>...]]
             [LIMIT <n>] [OFFSET <n>]
             [--type:table]

GET <block> <id>
GET <block> @ <id>

EXPLAIN FIND ...     -- runs the query for real and reports what happened
EXPLAIN SEARCH ...   -- (like EXPLAIN ANALYZE, not just a plan estimate)
```

**Basic find / by id:**
```sql
FIND products WHERE _id = "abc123"
GET products abc123
GET products @ abc123
```

**Filters (see full operator table in §21):**
```sql
FIND products WHERE price > 20 AND in_stock = true
FIND products WHERE name LIKE "%board%" OR name CONTAINS "Mon"
FIND products WHERE price BETWEEN 10 AND 100
FIND products WHERE status IN ("active", "pending")
FIND products WHERE tags IN ["go", "database", "nosql"]
```

**Grouping/precedence with parentheses and NOT (FIND/SEARCH only):**
```sql
FIND products WHERE (status = "active" OR status = "trial") AND price >= 18
FIND products WHERE NOT (status = "banned" OR status = "suspended")
```

**Projection (SELECT):**
```sql
FIND products SELECT name, price WHERE price > 20
```

**Computed SELECT fields — COUNT(field), simple arithmetic AS alias:**
```sql
FIND movies SELECT title, COUNT(actors) as actors_count, year
FIND movies SELECT title, duration_minutes / 60 as hours
```

**GROUP BY / HAVING (FIND only):**
```sql
FIND movies SELECT title, COUNT(actors) as actors_count, year
  WHERE year >= 2000
  GROUP BY title, year
  HAVING COUNT(actors) >= 5
```

**Filtering through a RELATE alias (see §13):**
```sql
RELATE movies USE directors
FIND movies SELECT title, directors.name
  WHERE directors.name == "Christopher Nolan"
```

**Sorting, pagination, table output:**
```sql
FIND products ORDER name, price:DESC WHERE price > 18
FIND products WHERE price > 18 LIMIT 50 OFFSET 100
FIND products WHERE price > 18 --type:table
```

**EXPLAIN:**
```sql
EXPLAIN FIND products WHERE price > 18 ORDER price:DESC LIMIT 10
EXPLAIN SEARCH products "wireless keyboard"
```

### SEARCH — full-text

```
SEARCH <block> "<text>" [EXACT | FUZZY]
                         [WITH SCORE] [WITH MATCHES]
                         [WHERE <condition>] [LIMIT <n>] [ORDER <field>]
```

```sql
SEARCH products "wireless keyboard"
SEARCH products "exact phrase" EXACT
SEARCH products "~keybord" FUZZY
SEARCH products "keyboard" WITH SCORE WITH MATCHES
SEARCH products "+must_include -must_exclude optional"
SEARCH products "keyboard" WHERE price > 18 LIMIT 50 ORDER name
```

### UPDATE

```
UPDATE <block> WHERE <condition> SET <field> = <value>[, <field2> = <value2> ...]
UPDATE <block> WHERE <condition> INC <field> = <n>
UPDATE <block> WHERE <condition> DEC <field> = <n>
UPDATE <block> WHERE <condition> PUSH <field> = <value>
UPDATE <block> WHERE <condition> PULL <field> = <value>
UPDATE ALL <block> SET <field> = <value>[, ...]
```

- `SET` replaces field values. `INC`/`DEC` add/subtract a number from a numeric
  field. `PUSH`/`PULL` append/remove a value from an array field.
- `UPDATE ALL` applies to every document in the block, no `WHERE` needed.
- Clauses can combine multiple assignments and functions such as `now()`.

```sql
UPDATE products WHERE _id = "kb001" SET name = "Mechanical Keyboard", price = 55
UPDATE products WHERE _id = "kb001" INC views = 1
UPDATE products WHERE _id = "kb001" DEC stock = 5
UPDATE products WHERE _id = "kb001" PUSH tags = "on_sale"
UPDATE products WHERE _id = "kb001" PULL tags = "discontinued"
UPDATE ALL products SET status = "archived"
UPDATE products WHERE status = "draft" SET status = "published", published_at = now()
```

### DELETE

```
DELETE <block> WHERE <condition>
DELETE ALL <block>
EMPTY BLOCK [<db>] <name>     -- alias for DELETE ALL
CLEAR [<db>] <name>           -- alias for DELETE ALL / EMPTY BLOCK
```

```sql
DELETE products WHERE _id = "kb001"
DELETE products WHERE price < 5 OR in_stock = false
DELETE ALL products
```

### Aggregations (COUNT/SUM/AVG/...)

```
COUNT  <block> [WHERE <condition>]
SUM    <block> <field> [WHERE <condition>]
AVG    <block> <field> [WHERE <condition>]
MIN    <block> <field> [WHERE <condition>]
MAX    <block> <field> [WHERE <condition>]
MEDIAN <block> <field> [WHERE <condition>]
MODE   <block> <field> [WHERE <condition>]
STDDEV <block> <field> [WHERE <condition>]
```

```sql
COUNT products WHERE in_stock = true
SUM orders amount WHERE status = "completed"
AVG products price WHERE category = "electronics"
MIN products price
MAX products price
MEDIAN salaries amount
MODE products category
STDDEV scores value
```

### GROUP BY

```
GROUP <block> BY <field> [COUNT | SUM | AVG | MIN | MAX] [<field>] [WHERE <condition>]
```

```sql
GROUP users BY city COUNT
GROUP orders BY status SUM amount
GROUP products BY category AVG price WHERE price > 10
GROUP logs BY level COUNT WHERE timestamp > "2024-01-01"
```

### ACID Transactions

```
BEGIN [<db> <block>]
  <INSERT|UPDATE|DELETE statements...>
COMMIT
ROLLBACK | ABORT

TX STATUS       Show current transaction details
TX LIST         List active transactions
TX ISOLATION    Show isolation level
```

Isolation levels (configured, not selected per-statement):
`read_committed`, `repeatable_read` (default), `serializable`.

```sql
BEGIN shop products
  INSERT products {"name": "Webcam", "price": 39.90}
  UPDATE products WHERE _id = "kb001" SET price = 45
  DELETE products WHERE _id = "old001"
COMMIT
```
```sql
BEGIN shop products
  INSERT products {"name": "Bad idea"}
ROLLBACK
```

### TURBO / BULK loading

```
BULK MODE ON            Wider batch windows, relaxed WAL fsync policy
BULK MODE OFF           Restore normal low-latency batching/fsync policy
BULK STATUS             Show turbo engine stats (worker pool, batching)

IMPORT <block> FROM FILE '<path>' [FORMAT NDJSON|ARRAY] [BATCH <n>]
```

- Plain `INSERT`s already auto-batch concurrent writes to the same block;
  `BULK MODE` widens that further for large loads.
- `IMPORT ... FROM FILE` streams a file into `<block>` in large batches
  (default 20000 docs/batch) without holding the whole file in memory.
  `FORMAT NDJSON` (default) reads one JSON object per line; `FORMAT ARRAY`
  reads a single top-level JSON array. A `.gz`-suffixed path is decompressed
  on the fly. Runs under BULK MODE automatically for the duration of the load.

```sql
BULK MODE ON
INSERT products GENERATE 2000000
BULK MODE OFF

IMPORT products FROM FILE '/data/products.ndjson' FORMAT NDJSON BATCH 50000
IMPORT products FROM FILE '/data/products.json.gz' FORMAT ARRAY
BULK STATUS
```

### JOIN

```
JOIN <block1> WITH <block2> ON <block1>.<field> = <block2>.<field>
```

```sql
JOIN orders WITH customers ON orders.customer_id = customers._id
JOIN posts WITH users ON posts.author_id = users._id
```

### RELATE

Register once how a block relates to other blocks (optionally in other
databases); `FIND` then resolves the relation automatically instead of
repeating a `JOIN` condition in every query.

```
RELATE <block> USE <target1>[,<target2>,...]
```

- Each target is a bare block name (same database) or `db.block` (cross-database).
- Match convention: the source document must have a `<target>_id` field (the
  singular of the target block name) holding the target's id — a single id
  for one-to-one/many-to-one, or an array of ids for one-to-many.
- After relating, select target fields with dot notation, or the bare alias
  for the whole related document.

```sql
RELATE movies USE directors,actors,genres
RELATE sales USE crm.customers,inventory.products,accounting.invoices

FIND movies SELECT title,directors.name,actors.name
FIND sales SELECT customers.name,products.name,invoices.total
```

### AUTORELATIONS

CaimanDB watches its own read access: when the same user reads the same
document repeatedly in a short window (default: 5 reads in 10 minutes), it
automatically creates a self-relation between that user and the document — no
`RELATE` needed. Each auto-relation carries `access_count`/`last_seen`,
a `relevance` score, and a small `key_metadata` sample of the doc's fields.

Unlike `RELATE` (explicit, permanent), auto-relations are temporal: every
further access slides their expiry forward (default TTL: 24h); once a
document stops being read by that user, the relation expires and is swept
away by a background pass. The resulting graph is bipartite and directed
(`user -> document read`).

```
SHOW AUTORELATIONS <block>
    [FROM <id>] [TO <id>] [DEPTH <n>] [DIRECTION IN|OUT|BOTH]
    [FORMAT TABLE|TREE|GRAPH|JSON]
    [WHERE|FILTER <expression>]
    [ORDER BY DEGREE|ID|NAME|ACCESS_COUNT|RELEVANCE|LAST_SEEN|FIRST_SEEN [ASC|DESC]]
    [LIMIT <n>] [OFFSET <n>]
    [STATS] [PATHS] [ORPHANS] [CYCLES] [BROKEN] [SUMMARY] [VERBOSE];
```

| Modifier | Meaning |
|---|---|
| `FROM <id>` | Start from this document or user id (auto-detected) |
| `TO <id>` | Keep only relations touching this id as the other end |
| `DEPTH <n>` | Hops to traverse from FROM (default 1) |
| `DIRECTION` | `OUT` = reads outward from a user; `IN` = readers inward into a doc; `BOTH` (default) |
| `FORMAT` | `TABLE` (default), `TREE` (needs FROM), `GRAPH` (adjacency list), `JSON` |
| `WHERE`/`FILTER` | Condition over `doc_id`, `user_id`, `access_count`, `relevance`, `last_seen`, `first_seen` |
| `ORDER BY` | `DEGREE`, `ID`, `NAME`, `ACCESS_COUNT`, `RELEVANCE`, `LAST_SEEN`, `FIRST_SEEN` (+`ASC`/`DESC`) |
| `LIMIT`/`OFFSET` | Pagination over the final, sorted result |
| `STATS`/`SUMMARY` | Prepend an aggregate report |
| `PATHS` | Render the FROM traversal as an indented tree |
| `ORPHANS` | Only isolated pairs |
| `CYCLES` | Only relations closing a cycle |
| `BROKEN` | Only relations whose document was later deleted |
| `VERBOSE` | Add first_seen/expires/full metadata |

```sql
SHOW AUTORELATIONS products;
SHOW AUTORELATIONS products FROM p_042;
SHOW AUTORELATIONS products FROM p_042 DEPTH 3 DIRECTION BOTH FORMAT TREE;
SHOW AUTORELATIONS products STATS;
SHOW AUTORELATIONS products WHERE access_count > 10 ORDER BY DEGREE LIMIT 20;
SHOW AUTORELATIONS products FROM p145 DEPTH 6 DIRECTION BOTH
  WHERE relevance >= 0.75 ORDER BY ACCESS_COUNT DESC LIMIT 100
  FORMAT TREE STATS VERBOSE;
SHOW AUTORELATIONS products CYCLES;
SHOW AUTORELATIONS products BROKEN;
```

### Views

```
VIEW CREATE <name> AS FIND <block> WHERE <condition>
VIEW DROP <name>
VIEW SHOW
VIEW INFO <name>
<view_name>                    -- executes the view
```

```sql
VIEW CREATE active_users AS FIND users WHERE active = true
VIEW SHOW
VIEW INFO active_users
active_users
VIEW DROP active_users
```

### EXPORT / IMPORT

`EXPORT` always writes **both** a `.csv` and a `.json` file into
`<data_root>/backups/`, using the base name you give it. Every exported
row/document includes both `_id` and `id`.

```
EXPORT <block> [WHERE <condition>] TO "<file>"
IMPORT <block> FROM "<file.json>"        -- also looks inside backups/
IMPORT <block> FROM "<file.csv>"
```

```sql
EXPORT products TO "products_export"
-- -> writes backups/products_export.csv and backups/products_export.json
EXPORT products WHERE price > 18 TO "expensive_products"

IMPORT products FROM "products_export.json"
IMPORT products FROM "products_export.csv"
```

### User management

```
CREATE USER <name> PASSWORD "<pass>" [ROLE admin|readwrite|readonly]
DROP USER <name>
SHOW USERS
```

```sql
CREATE USER analyst PASSWORD "s3cret!" ROLE readonly
SHOW USERS
DROP USER analyst
```

### Shard management

```
SHARD STATUS
SHARD REBALANCE
SHARD SCALE <db> <shards>
```

```sql
SHARD STATUS
SHARD REBALANCE
SHARD SCALE shop 32
```

### Cluster management

```
CLUSTER STATUS
```

### Navigation & system

```
PWD               Show current path
LS                List databases
LS <db>           List blocks in database
CD <db>           Change to database
TREE              Show full directory tree
STATUS            Show system status
HEALTH            Show health check
VERSION           Show version
PING              Ping the engine
HELP              Show help
EXIT, QUIT        Exit the shell
```

### Filter operators (usable in every `WHERE`)

| Operator | Meaning |
|---|---|
| `=`, `==` | Equal |
| `!=`, `<>` | Not equal |
| `>`, `<` | Greater than / Less than |
| `>=`, `<=` | Greater or equal / Less or equal |
| `LIKE` | Pattern matching (`%` wildcard) |
| `CONTAINS` | Substring contains |
| `EXISTS` | Field exists |
| `IN` | Value in list |
| `NOT IN` | Value not in list |
| `BETWEEN` | Range between two values |
| `STARTS WITH` | String starts with |
| `ENDS WITH` | String ends with |
| `AND` | Logical AND (default when omitted) |
| `OR` | Logical OR |

### Full worked example

```sql
CREATE DB shop
USE shop
CREATE BLOCK products

INSERT products kb001 {"name": "Keyboard", "price": 49.90, "in_stock": true}
INSERT products {"name": "Mouse", "price": 19.90, "in_stock": true}
INSERT products name: "Monitor", price: 199.00, in_stock: true

FIND products WHERE price > 20
FIND products WHERE name LIKE "%oard%" ORDER price:DESC
FIND products WHERE price BETWEEN 20 AND 200 SELECT name, price

SEARCH products "keyboard" WITH SCORE

UPDATE products WHERE _id = "kb001" SET price = 45
UPDATE products WHERE price < 30 INC price = 1

COUNT products WHERE in_stock = true
AVG products price
GROUP products BY in_stock COUNT

DELETE products WHERE _id = "kb001"

BEGIN shop products
  INSERT products {"name": "Webcam", "price": 39.90}
  UPDATE products WHERE name = "Mouse" SET price = 17.90
COMMIT

VIEW CREATE cheap_products AS FIND products WHERE price < 25
cheap_products

EXPORT products TO "products_backup"
STATUS
STATS DB shop
```
