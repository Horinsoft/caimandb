# Changelog

## [Unreleased] — WAL: Prevents Double Application of Entries After a Second Crash

**Fixed Issue:** After WAL recovery on startup (`RecoverWAL`), the engine only pruned old segments (`PruneToLastSegment`) but left the *active* segment — the same one it had just read and applied — as the destination for new writes. If the process crashed again without a clean `Close()` in between, the next startup would re-read that segment and reapply the same entries a second time. This was harmless for `insert` (idempotent by ID) but corrupts non-idempotent `update` operations (`$inc`, etc.).

**Fix:** New `WAL.RotateAndPruneFresh()` method (`internal/caimandb/wal/wal.go`), used by `RecoverWAL` (`internal/caimandb/wal_recovery.go`) instead of direct `PruneToLastSegment()`: rotates to a new empty segment before pruning, ensuring no already-applied entries remain on disk to be re-read. See `docs/corruption-fixes-2026-07.md` (section 6) for complete details.

**Not Touched:** The rest of the startup/shutdown/cleanup pipeline was already well covered — `.clean` marker invalidated on startup (point 1 of July document), graceful shutdown with `SIGINT`/`SIGTERM`/`SIGHUP` and `http.Server.Shutdown` with timeout, adaptive worker pool that auto-terminates idle goroutines (`internal/caimandb/turbo/pool.go`), and dedicated `tx_cleanup_loop` that aborts and purges abandoned/expired transactions (`internal/caimandb/transaction.go`). This change reviewed that entire path without touching it because it already worked correctly; the only actual bug found was the one above.

---

## [Unreleased] — Storage AI: Adaptive BadgerDB Sizing by Block

**Fixed Issue:** Every block (new or existing) was opened with the same fixed BadgerDB options (`storage/constants.go`), so a nearly empty block occupied the same disk/RAM as one with hundreds of thousands of documents (~49MB reported). With many small blocks, this becomes a real space and memory problem.

**`internal/caimandb/storage/adaptive.go` (new):** 4 tiers of BadgerDB configuration (`micro`/`small`/`standard`/`large`). A block's tier is chosen, the first time it is opened in the process, based on what already exists on disk for that block (empty → `micro`) and adjusted against a shared RAM budget across all simultaneously opened blocks (default 50% of detected RAM via `/proc/meminfo`, configurable).

**`storage/badger_pool.go`:** `DBPool` now performs this selection when opening each block (`OpenDataPath`/`OpenBlock`, without changing its public signature, so none of the ~20 existing call sites were touched) and releases the corresponding budget on close. New `DBPool.AdaptiveStats()` for observability (block count per tier, budget used/total).

**New Configuration:** `storage_ai_enabled` (default `true`), `storage_ai_ram_fraction` (default `0.5`), `storage_ai_max_budget_mb` (default `0` = no explicit cap), with corresponding environment variables `CAIMANDB_STORAGE_AI_*`. `storage_ai_enabled=false` reproduces the exact original fixed behavior.

**What It Does NOT Do Yet, Intentionally:** Does not hot-readjust a block that is already serving traffic (only classified on open). See [`docs/storage-ai-adaptive-sizing.md`](docs/storage-ai-adaptive-sizing.md) for complete analysis of why (reads in `ops_find.go` do not take the write lock, so closing/reopening the Badger handle under concurrent read traffic is not safe without more) and the recommended path to implement it with a compiler at hand.

---

## [Unreleased] — `SHOW AUTORELATIONS`: `WHERE` and `ORDER BY` with Direction

**`WHERE <expression>`** (`cmd_show_autorelations.go`): Synonym for `FILTER` with the same condition syntax used by `FIND`/`SEARCH`/`VIEW`, on `doc_id`, `user_id`, `access_count`, `relevance`, `last_seen`, `first_seen`. `FILTER` remains for backward compatibility.

**`ORDER BY ... ASC|DESC`:** `ORDER BY` now accepts an explicit optional direction, and sortable fields are extended from `DEGREE|ID|NAME` to also include `ACCESS_COUNT`, `RELEVANCE`, `LAST_SEEN`, and `FIRST_SEEN`. Without `ASC`/`DESC`, each field retains its default direction (descending for `DEGREE`/`ACCESS_COUNT`/`RELEVANCE`/`LAST_SEEN`/`FIRST_SEEN`, ascending for `ID`/`NAME`).

**Combined Example:** `SHOW AUTORELATIONS products FROM p145 DEPTH 6 DIRECTION BOTH WHERE relevance >= 0.75 ORDER BY ACCESS_COUNT DESC LIMIT 100 FORMAT TREE STATS VERBOSE;`

---

## [Unreleased] — Temporal Auto-Relations Based on Access Patterns

**`relations_auto.go`, new `AutoRelationManager`:** When the same user repeatedly reads the same document (default: 5 reads in 10 minutes), CaimanDB automatically creates an auto-relation (self-relation) between that user and the document — without requiring an explicit `RELATE`. Each auto-relation stores `access_count`, `last_seen`, a relevance score (`relevance`, combining frequency and recency), and a small sample of key document metadata (`key_metadata`, e.g., `name`/`title`).

**These are temporal data by design:** Each new access extends their expiration (`auto_relation_ttl`, 24h by default); if the user-document pair stops being accessed, the relation expires on its own and a background sweep (`autoRelationCleanupLoop`, every 5 min) removes it. This is not durable data — it's a live signal of "what is relevant right now."

**`FIND` and `GET <id>` feed the detector automatically** (`cmd_find.go`): every document read by a session is reported to the `AutoRelationManager` with the authenticated user of that session.

**`SHOW AUTORELATIONS <block>`** (`cmd_show_autorelations.go`, `cmd_show_autorelations_render.go`, `relations_auto_graph.go`): Full graph-like query command over that set of auto-relations (bipartite and directed: `user -> read document`), with modifiers `FROM`, `TO`, `DEPTH`, `DIRECTION IN|OUT|BOTH`, `FORMAT TABLE|TREE|GRAPH|JSON`, `ORDER BY DEGREE|ID|NAME`, `LIMIT`, `OFFSET`, `FILTER <expression>` (reuses the same WHERE evaluator as `FIND`), `STATS`, `SUMMARY`, `PATHS` (BFS traversal tree from `FROM`), `ORPHANS` (isolated pairs), `CYCLES` (detected with union-find), and `BROKEN` (relations whose document was deleted).

**New Configuration Options:** `auto_relations_enabled`, `auto_relation_threshold`, `auto_relation_window`, `auto_relation_ttl`.

---

## [Unreleased] — Web Console (ui/) with Real Status Panel, Connected to Live Node Data

**`ui/` Console Rewritten** with the look and feel of a Beekeeper Studio-style database client: icon rail with two views (Query Editor / Dashboard), real query tabs, sidebar with databases and node blocks, and status panel with KPIs and charts. All content comes from real HTTP requests to the node itself — no sample data or `Math.random` on the client.

**`GET /entities`** (`http_query.go`), new query server endpoint: lists databases (`Engine.ShowDBs`) and, for the active database, its blocks with real document counts and sizes (`Engine.DescribeBlock`) — feeding the sidebar and block document chart.

**`POST /query` now also returns `rows`/`columns`** for `FIND`/`GET` commands: a structured document grid (re-executing the same read against the engine), in addition to the result text that all commands already returned. This allows the client to render a real table instead of parsing text output designed for the terminal.

**`L1Cache.Stats()` exposes raw bytes** (`used_bytes`, `max_bytes`, `hit_ratio_pct`) alongside already-formatted strings, so the dashboard can calculate a real cache usage percentage without parsing strings like "128.00 MB".

---

## [Unreleased] — High-Performance Engine: Sharded Cache, WAL with Group Commit, and Real-Time Change Streams

**L1/L2 Cache with Sharding (32 partitions)** (`cache.go`). Previously each cache had **a single global mutex** protecting the entire map + LRU list, so any read/write serialized all other threads — the classic bottleneck under sustained concurrent load. Now each cache is split into 32 independent shards (using `fnv32a` hash already used by `shardedLockManager`), each with its own mutex, map, and LRU list/byte budget, reducing contention from "global" to "1/32 of keys." A **real data race** was also fixed: `L2IndexCache.get()` took `RLock` but mutated shared fields (`LastUsed`, `Frequency`) without exclusive lock. Direct access to `l1Cache.mu`/`l1Cache.items` in `RenameDB` (which also had a **latent deadlock**: took `Lock()` then called `del()`, which re-acquires the same mutex) was replaced with the new `L1Cache.DeleteByPrefix` method.

**WAL with Buffered I/O + Configurable fsync (Group Commit)** (`wal.go`). The WAL never performed `fsync` — data remained in the OS buffer without real durability guarantees despite being called "Write-Ahead Log." Each entry was also written with `json.Encoder.Encode` directly against the `*os.File`, without buffering (one syscall per entry). Now each segment uses a `bufio.Writer`, batches are encoded in memory (reusing the buffer pool, which was previously allocated but never used) to emit **a single `Write()` per batch**, and there is a configurable sync policy (`wal_sync_policy`: `always` / `interval` [default, group commit] / `off`) that caps the maximum data loss window without paying an fsync per operation.

**`BufferPool` Without Data Race** (`buffer_pool.go`): counters were plain `int64` mutated from multiple goroutines without atomics.

**Real-Time Change Streams** (`changestream.go`): New `ChangeBus` (in-process pub/sub) that publishes insert/update/delete events as they occur, with non-blocking `Publish()` (a slow subscriber never slows the write path — its event is dropped and counted in `caimandb_change_events_dropped_total`). Exposed via SSE at the new endpoint `GET /watch?db=...&block=...` (`http_query.go`), with the connection exempted from the global server `WriteTimeout` (which would otherwise cut the stream at 30s) and a heartbeat every 15s to keep it alive through proxies/load balancers.

---

## [Unreleased] — New `EXPLAIN FIND ...` / `EXPLAIN SEARCH ...` Command

(`cmd_explain.go`). "EXPLAIN ANALYZE" style: builds the query with the same functions used by `FIND`/`SEARCH` (`buildFindQuery`/`buildSearchQuery`, extracted from `cmd_find.go` so EXPLAIN can never desynchronize from what the actual command does), executes it for real, and reports measured metrics: rows scanned, rows found, what access was actually used (`Actual Access`), what index the optimizer would have chosen for top-level equalities/IN (`Planned`, via `QueryOptimizer.AnalyzeQuery`) -- and the parsed `WHERE` tree (`parse.Expr.String()`), useful now that it supports parentheses and precedence. Does not invent numbers it didn't measure (no "memory used" or fictional "cost").

**`SHOW DBS` and `SHOW BLOCKS` Now Accept One or More Names** (`cmd_show_size.go`): `SHOW DBS`, `SHOW DBS name`, `SHOW DBS name1 name2`; same for `SHOW BLOCKS [<db>] [name1 name2 ...]`. If any name doesn't exist, it's reported on a "Not found: ..." line instead of failing the entire command. `SHOW DBS` now also includes on-disk size per database (previously only showed blocks/docs) and a total aggregate at the end; `SHOW BLOCKS` already showed size per block and now also aggregates a total.

`docs/known-limitations.md` and `docs/nql-reference.md`: unchanged in this pass (see previous block for AST status); `help.go` documents the new `EXPLAIN` and `SHOW DBS/BLOCKS` syntax.

**Scope of This Pass, Explicitly:** Only `EXPLAIN` and the requested `SHOW` improvements were implemented. The broader NQL vision discussed (`USING`/`EXPAND` for relations without global `RELATE` state, `GROUP BY` inside `FIND`, inline aggregation functions (`COUNT()`/`SUM()`/...), `DISTINCT`, `PAGE`/`SIZE`, `FIND ... IDS`, `FIND ... COUNT`, `FIND ... CACHE`, `ANALYZE FIND ...`) is a significantly larger language redesign and was not touched in this pass to avoid risking extensive changes without being able to compile; it remains pending if it should be approached incrementally.

**Not Verified with `go build`/`go vet`** — same caveat as the rest of this changelog: no Go or network in this environment. Manually reviewed with special attention to: that `buildFindQuery`/`buildSearchQuery` extracted from `cmd_find.go` preserve the exact behavior of `handleFind`/`handleSearch` (they are the same code, only moved to a function parameterized by starting index), that no unused imports remain after moving code between files (`time` in `cmd_create_show.go`), and that no function names are duplicated. Confirm with `go build ./... && go vet ./... && go test ./...` before deploying.

---

## [Unreleased] — Real AST for WHERE (Parentheses, Precedence, NOT)

New `internal/caimandb/parse/ast.go`: a recursive-descent parser that compiles a `WHERE` clause into a real expression tree (`Expr`) -- `KindCondition` (leaf) / `KindAnd` / `KindOr` / `KindNot` -- instead of the flat `[]Filter` list with one `Logic` per element that was evaluated strictly left-to-right. Supports:
- Parentheses for grouping (`(a = 1 OR b = 2) AND c = 3`).
- Standard precedence `AND` > `OR` (previously nonexistent: the only possible semantics was "left-to-right", so `a=1 OR b=2 AND c=3` couldn't be written with its usual meaning).
- `NOT` before a condition or a complete group.
- Same operators and same value syntax as always (quotes, JSON arrays/objects, `BETWEEN x AND y`, `IS [NOT] NULL`, two-word operators) -- intentionally preserved, including a peculiarity of the original parser (a quoted value still goes through the same numeric/boolean coercion as an unquoted one).

`internal/caimandb/parse/tokenizer.go`: `(` and `)` are now always emitted as proper tokens (previously only brackets `[]` and braces `{}` were recognized), so AST grouping works without requiring exact spaces around parentheses.

Connected to `FIND` and `SEARCH` (`cmd_find.go`): new `parseWhereClause` in `cmd_filters_util.go` builds the tree and, additionally, a best-effort flat list (only when the tree is a pure chain of top-level `AND`) to avoid touching the existing index planner (`QueryOptimizer.AnalyzeQuery`). `Query` (`ops_find.go`) has a new field `Where *parse.Expr`; `matchesQuery`/`evalExpr` (`query_filter.go`) evaluate the tree when present and otherwise fall back to `matchesFilters` as before -- by design, everything else that constructs `[]Filter` directly (`JOIN`, transactions, `VIEW`, admin) remains exactly as before this change.

`docs/nql-reference.md` documents the new syntax with examples; `docs/known-limitations.md` documents which commands already use the AST and which remain on the flat parser, with a migration path.

**Not Verified with `go build`/`go vet`** — this environment also had no access to a Go compiler or network (same caveat as the rest of this changelog). Manually reviewed with special care at integration points (signature of `Query`, the two call-sites of `matchesFilters`, and that no existing positional `Query{...}` literal uses positional fields), but confirm with `go build ./... && go vet ./...` before deploying.

---

## [Unreleased] — configs/caimandb.conf and First Subpackage (parse/)

`caimandb.conf` now loads/creates from `configs/caimandb.conf` instead of the root working directory (`internal/caimandb/constants.go`, `internal/caimandb/defaults.go`); `SaveToFile` creates the `configs/` folder itself if needed. Updated `help.go`, `README.md`, `docs/configuration.md`, `docs/architecture.md`, `examples/quickstart.md`, and `.gitignore` to reflect this.

New subpackage `internal/caimandb/parse`: the NQL syntax tokenizer (`tokenize` → `parse.Tokenize`) was moved here — the only engine file with no dependencies on `Engine`/`Document`/`Config`/`Session`/`Filter`/`Transaction`. `dsl_parser.go` and `cmd_view.go` now import it as `caimandb/internal/caimandb/parse`.

`docs/known-limitations.md` expanded: documents this first safe split, includes a `grep` filter for finding more "leaf" file candidates, and maintains the recommended path (`httpapi`, `cluster`, leaving `raft_fsm.go`/`transaction.go` in the core) for anyone who wants to continue splitting the package with a Go compiler at hand.

---

## [Unreleased] — Repository Reorganization

New folder structure at repo level: `docs/` (with `docs/api/`), `deployments/docker/`, `scripts/`, `configs/`, `examples/`, `test/` (with `test/integration/` and `test/fixtures/`), `.github/workflows/`.

New documentation: architecture (`docs/architecture.md`), complete NQL reference (`docs/nql-reference.md`), HTTP API (`docs/api/http-api.md`), configuration (`docs/configuration.md`), known limitations (`docs/known-limitations.md`).

Configuration template (`configs/caimandb.conf.example`) generated from the actual `Config` struct.

`Dockerfile` + `docker-compose.yml` for running CaimanDB in a container.

Build/test/run scripts (`scripts/*.sh`) and `Makefile`.

GitHub Actions CI workflow (`go build`, `go vet`, `go test`, `go mod tidy` check).

`internal/caimandb` was intentionally left intact: it is a single Go package with over 40 files coupled to the same `Engine` via unexported fields; truly splitting it requires exporting much of that state and verifying with a compiler, something that could not be confirmed in this environment (see `docs/known-limitations.md`).

---

## Previous Pass — Compilation Fixes

`cmd/` + `internal/caimandb/` structure instead of a flat directory of 68 files in the root.

Removed duplicate redeclaration of `views`/`viewsMu`.

Implemented 11 NQL handlers that `dsl_parser.go` referenced but did not exist (`handleDrop`, `handleRename`, `handleInfo`, `handleDescribe`, `handleStats`, `handleSize`, `handleRebuild`, `handleCheck`, `handleRepair`, `handleFlexCommand`, `handleTransaction`), in `cmd_admin_extra.go`.

Removed unused imports of `github.com/hashicorp/raft` and `errors` in 11 files.
