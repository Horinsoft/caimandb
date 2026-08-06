package caimandb

import (
	"caimandb/internal/caimandb/storage"
	"caimandb/internal/caimandb/wal"
	"encoding/json"
	"hash/crc32"

	"go.uber.org/zap"
)

// RecoverWAL replays WAL entries written since the last clean shutdown,
// applying them directly to engine's local storage paths.
//
// This used to be a method on *wal.WAL (WAL.RecoverApply). It now lives
// here, in the root package, because applying entries requires calling
// Engine's unexported local insert/update/delete paths — a dependency
// the low-level wal package must not have on the engine. wal only
// exposes the generic log primitives (ReadAll, IsClean, MarkRecovered,
// PruneToLastSegment) that this function composes.
func RecoverWAL(w *wal.WAL, engine *Engine) (int64, error) {
	if w.IsClean() {
		log().Info("WAL clean shutdown, skipping recovery")
		return 0, nil
	}

	entries, err := w.ReadAll()
	if err != nil {
		return 0, err
	}

	if len(entries) == 0 {
		return 0, nil
	}

	log().Info("Starting WAL recovery",
		zap.Int("entries", len(entries)))

	applied := int64(0)
	batchSize := 5000

	for i := 0; i < len(entries); i += batchSize {
		end := i + batchSize
		if end > len(entries) {
			end = len(entries)
		}
		batch := entries[i:end]

		for _, entry := range batch {
			if crc32.ChecksumIEEE(entry.Data) != entry.Checksum {
				log().Warn("WAL entry checksum mismatch, skipping",
					zap.Uint64("id", entry.ID))
				continue
			}

			entryData := entry.Data
			if entry.Compressed {
				if decompressed, err := storage.DecompressData(entry.Data); err == nil {
					entryData = decompressed
				}
			}

			switch entry.Op {
			case "insert":
				var doc Document
				if err := json.Unmarshal(entryData, &doc); err != nil {
					continue
				}
				if _, err := engine.insertWithIDLocal(entry.DB, entry.Block, doc.ID, doc.Data, doc.ShardID); err != nil {
					log().Warn("WAL insert recovery failed", zap.Error(err))
					continue
				}
				applied++
			case "update":
				var op UpdateOp
				if err := json.Unmarshal(entryData, &op); err != nil {
					continue
				}
				if _, err := engine.updateLocal(entry.DB, entry.Block, nil, op, ""); err != nil {
					log().Warn("WAL update recovery failed", zap.Error(err))
					continue
				}
				applied++
			case "delete":
				var filters []Filter
				if err := json.Unmarshal(entryData, &filters); err != nil {
					continue
				}
				if _, err := engine.deleteLocal(entry.DB, entry.Block, filters, ""); err != nil {
					log().Warn("WAL delete recovery failed", zap.Error(err))
					continue
				}
				applied++
			}
		}

		if i > 0 && i%10000 == 0 {
			log().Info("WAL recovery progress",
				zap.Int("processed", i),
				zap.Int("total", len(entries)),
				zap.Int64("applied", applied))
		}
	}

	if applied > 0 {
		// w.MarkRecovered() is kept in memory only, so a second in-process
		// call to RecoverApply (engine startup calls it once directly and
		// once more from InitCluster when auto-cluster is on) doesn't
		// re-apply the same entries. It is intentionally NOT written to
		// the .clean marker file: that file may only be written by a
		// graceful Close() (see checkCleanliness for why persisting it
		// here would cause silent data loss on a later crash).
		w.MarkRecovered()

		// The entries we just applied are now durably reflected in
		// storage. The segment we read them from is still the *active*
		// segment (new writes are about to be appended to it), so a plain
		// PruneToLastSegment would leave those already-applied entries on
		// disk ahead of the new ones — replaying them again on a second,
		// pre-Close crash. RotateAndPruneFresh closes that window by
		// starting the post-recovery segment empty: insert is idempotent
		// by ID, but update ops (e.g. increments) are not, so this
		// matters. See its doc comment in wal.go for the full story.
		if err := w.RotateAndPruneFresh(); err != nil {
			log().Warn("Failed to rotate WAL to a fresh segment after recovery; "+
				"already-applied entries remain on disk and could be replayed "+
				"again on a subsequent crash before the next clean shutdown",
				zap.Error(err))
		}
	}

	log().Info("WAL recovery completed",
		zap.Int64("applied", applied),
		zap.Int("entries", len(entries)))
	return applied, nil
}

