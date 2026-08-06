module caimandb

go 1.22

// NOTE: RoaringBitmap/roaring, dgraph-io/ristretto/v2, google/uuid,
// natefinch/atomic, smallnest/ringbuffer, vmihailenco/msgpack/v5, and
// golang.org/x/sync are intentionally NOT pinned here. They were added by
// hand (this environment has no network access to the module proxy to
// verify real tags/pseudo-versions), and a single wrong version string in
// this file breaks every `go` command, not just the package it belongs to
// -- that's exactly what happened with a guessed github.com/eapache/queue/v2
// pseudo-version. build.sh/build.bat resolve and insert all of these for
// real via `go get <module>@latest` before running `go mod tidy`, so they
// end up here with versions the proxy actually has, not guesses.
require (
	github.com/cespare/xxhash/v2 v2.3.0
	github.com/dgraph-io/badger/v4 v4.9.4
	github.com/hashicorp/raft v1.7.3
	github.com/hashicorp/raft-boltdb v0.0.0-20220329195025-15018e9b97e0
	github.com/klauspost/compress v1.19.1
	github.com/prometheus/client_golang v1.24.0
	go.uber.org/zap v1.27.0
	golang.org/x/crypto v0.31.0
	golang.org/x/term v0.27.0
)
