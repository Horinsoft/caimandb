# Known Limitations

## The WHERE AST is still only used by FIND and SEARCH

`internal/caimandb/parse/ast.go` adds a real expression tree for
`WHERE` (`ParseWhere`): supports parentheses, `NOT`, and standard
`AND`/`OR` precedence (previously none of these three existed — see
the comment at the top of that file for why). It's connected to
`FIND` and `SEARCH` (`handleFind`/`handleSearch` in
`cmd_find.go`, via `parseWhereClause` in `cmd_filters_util.go` and
`matchesQuery`/`evalExpr` in `query_filter.go`).

The rest of the commands with `WHERE` — `UPDATE`, `DELETE`,
`CLEAR`/`COUNT`, aggregations (`SUM`/`AVG`/...), `VIEW CREATE` and
admin commands — still use `parseFilters`
(`cmd_filters_util.go`), the original flat parser: no parentheses, no
`NOT`, and evaluated strictly left-to-right. Migrating them is
mechanical (same pattern `tokens []string, idx *int`: change the call
from `parseFilters` to `parseWhereClause` and decide, command by
command, whether that command still needs to feed a flat `[]Filter`
somewhere else — e.g. `transaction.go` and `raft_fsm.go` might
serialize/replay filters in the WAL/Raft log, so that needs to be
verified with the compiler before touching them), but it wasn't done
in this pass to keep the risk contained without being able to compile
(see below). Candidates, in reasonable order:

1. `DELETE` / `CLEAR`+`COUNT` (`cmd_delete.go`, `cmd_clear_count.go`) —
   they're already a single block of `Filter`s without the
   complication of `transaction.go`/`raft_fsm.go`.
2. Aggregations (`cmd_aggregate.go`) — same shape.
3. `UPDATE` (`cmd_update.go`) and `VIEW CREATE` (`cmd_view.go`) —
   check first whether `ViewDefinition.Filters` or transaction
   logging need to serialize the filter (JSON, WAL, Raft) somewhere;
   if so, add serialization for `parse.Expr` as well before changing
   the parser that produces them.

## `internal/caimandb` remains a single Go package

This is the most significant limitation and the reason why this
reorganization pass focused on **repository** structure
(`docs/`, `deployments/`, `scripts/`, `configs/`, `examples/`,
`test/`, CI) rather than splitting the Go package itself.

We did a concrete investigation of what it would take to split it
into packages like `internal/engine`, `internal/storage`,
`internal/cluster`, `internal/httpapi`, etc. Result of static
analysis (grep for access to unexported `.field` / `.method` across
files):

- Files like `raft_fsm.go` and `transaction.go` directly access more
  than ten **unexported** fields/methods of `Engine` from different
  subsystems (`engine.pool`, `engine.dirMgr`,
  `engine.cacheKey`, `engine.lockManager`, `engine.l1Cache`,
  `engine.shardMgr`, `engine.externalStore`, `engine.intelEngine`,
  `engine.flexEngine`, `engine.buildSecondaryIndex`, ...). Moving them
  to another package would require exporting a good portion of the
  engine's internal state.
- Several subsystems (`cluster.go`, `dist_query.go`, `flexcolumn.go`,
  `http_admin.go`, `http_query.go`, `shard_manager.go`,
  `transaction.go`) hold a reference back to the `*Engine` itself
  (`engine *Engine`). If those types were moved to separate packages
  while `Engine` kept them as fields, it would create an import cycle;
  avoiding it properly requires inverting the dependency with
  interfaces (define in the new package only the methods it actually
  uses, and have `Engine` satisfy them implicitly). This is a real
  and reasonable change, but mechanical and extensive, with high risk
  of breaking a 15,000+ line build if done without a compiler to
  confirm every usage point.

This environment has no network access nor a Go toolchain installed,
so there was no way to compile and confirm a change of that size. We
chose not to apply it blindly.

### What was already separated: `internal/caimandb/parse`

`tokenizer.go` (the `tokenize` function, now `parse.Tokenize`) had no
references to `Engine`, `Document`, `Config`, `Session`, `Filter` or
`Transaction` — only standard library `strings`. Being a true "leaf"
file (zero coupling, not just low coupling), it was moved without
needing to export anything from the engine or risking an import cycle.
The two places that called it (`dsl_parser.go`, `cmd_view.go`) now
import `caimandb/internal/caimandb/parse`.

### If you want to go further

This is achievable work with a Go compiler available (local or CI),
subsystem by subsystem. A quick first filter to find more "leaf"
candidates like `parse`:

```bash
cd internal/caimandb
for f in *.go; do
  grep -qE "\bEngine\b|\bDocument\b|\bConfig\b|\bSession\b|\bFilter\b|\bTransaction\b" "$f" || echo "$f"
done
```

That points to ~18 files (`auth_jwt.go`, `cache.go`, `buffer_pool.go`,
`compression.go`, `ratelimit.go`, `audit.go`, `logging.go`,
`metrics.go`, `locks.go`, `worker_pool.go`, `storage_external.go`,
`raft_logstore.go`, `misc_utils.go`, `nested_fields.go`, among others)
that don't reference the core engine types by name — they're good
candidates for their own packages, but **before moving them** you need
to check the coupling *between them* (e.g. if several share `log()` from
`logging.go` or a cache type), which this filter doesn't detect. After
that, in order of least to most coupling with `Engine`:

1. `internal/httpapi` — move `http_admin.go` and `http_query.go`;
   only requires exporting ~10 fields from `Engine`
   (`nodeID`, `startupTime`, `opCount`, `l1Cache`, `metrics`,
   `tokenManager`, `cluster`, `flexEngine`, `shardMgr`, `config`) via
   getters.
2. `internal/cluster` — move `cluster.go`, `dist_query.go`,
   `flexcolumn.go`, `shard_manager.go`; moderate coupling.
3. Leave `raft_fsm.go` and `transaction.go` in the core package
   (`internal/engine`) — they're the ones that touch internal state
   most deeply and have the least benefit/risk to separate.

Each step should end with `go build ./... && go vet ./...` before
moving to the next.

## `go.sum` not verified

`go.mod` remains the same as in the original project. Run
`go mod tidy` on your machine after the first successful build to
confirm `go.sum` is consistent.

## No automated test suite

The original project didn't include tests. `test/` was added with a
template and a guide (`test/README.md`) to get started, but no tests
were invented that simulate coverage that doesn't exist.
