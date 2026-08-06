// cmd_admin_extra.go implements the DSL command handlers referenced by
// dsl_parser.go that did not yet have an implementation: DROP, RENAME,
// INFO, DESCRIBE, STATS, SIZE, REBUILD, CHECK, REPAIR, FLEX and the
// transaction commands (BEGIN/COMMIT/ROLLBACK/TX).
package caimandb

import (
	"fmt"
	"strings"
)

// resolveDBBlock resolves a database/block pair for admin sub-commands that
// accept either "<db> <block>" or just "<block>" (using the session's
// current database) starting at tokens[argStart].
func resolveDBBlock(sess *Session, tokens []string, argStart int) (db, block string, err error) {
	switch len(tokens) - argStart {
	case 1:
		block = tokens[argStart]
		db = sess.currentDB
		if db == "" {
			return "", "", fmt.Errorf("no database in use; specify: <db> <block>, or USE a database first")
		}
	case 0:
		return "", "", fmt.Errorf("block name required")
	default:
		db = tokens[argStart]
		block = tokens[argStart+1]
	}
	if db == "" {
		return "", "", fmt.Errorf("no database specified and no database in use")
	}
	if block == "" {
		return "", "", fmt.Errorf("block name required")
	}
	return db, block, nil
}

func handleDrop(eng *Engine, sess *Session, tokens []string) string {
	subCmd := strings.ToUpper(tokens[1])
	switch subCmd {
	case "DB":
		if len(tokens) < 3 {
			return "ERROR: DROP DB requires database name"
		}
		db := tokens[2]
		if err := eng.DropDB(db); err != nil {
			return "ERROR: " + err.Error()
		}
		return "Database dropped: " + db

	case "BLOCK":
		db, block, err := resolveDBBlock(sess, tokens, 2)
		if err != nil {
			return "ERROR: " + err.Error()
		}
		if err := eng.DropBlock(db, block); err != nil {
			return "ERROR: " + err.Error()
		}
		return "Block dropped: " + block + " in database: " + db

	case "USER":
		if len(tokens) < 3 {
			return "ERROR: DROP USER requires username"
		}
		username := tokens[2]
		if err := eng.DropUser(username); err != nil {
			return "ERROR: " + err.Error()
		}
		return "User dropped: " + username

	default:
		return "ERROR: unknown DROP object: " + subCmd
	}
}

func handleRename(eng *Engine, sess *Session, tokens []string) string {
	subCmd := strings.ToUpper(tokens[1])
	switch subCmd {
	case "DB":
		// RENAME DB <old> TO <new>
		if len(tokens) < 5 || strings.ToUpper(tokens[3]) != "TO" {
			return "ERROR: usage: RENAME DB <old> TO <new>"
		}
		oldName, newName := tokens[2], tokens[4]
		if err := eng.RenameDB(oldName, newName); err != nil {
			return "ERROR: " + err.Error()
		}
		return "Database renamed: " + oldName + " -> " + newName

	case "BLOCK":
		// RENAME BLOCK <db> <old> TO <new>  (or RENAME BLOCK <old> TO <new> using current db)
		var db, oldName, newName string
		if len(tokens) >= 6 && strings.ToUpper(tokens[4]) == "TO" {
			db, oldName, newName = tokens[2], tokens[3], tokens[5]
		} else if len(tokens) >= 5 && strings.ToUpper(tokens[3]) == "TO" {
			db = sess.currentDB
			oldName, newName = tokens[2], tokens[4]
		} else {
			return "ERROR: usage: RENAME BLOCK [db] <old> TO <new>"
		}
		if db == "" {
			return "ERROR: no database specified and no database in use"
		}
		if err := eng.RenameBlock(db, oldName, newName); err != nil {
			return "ERROR: " + err.Error()
		}
		return "Block renamed: " + oldName + " -> " + newName + " in database: " + db

	default:
		return "ERROR: unknown RENAME object: " + subCmd
	}
}

func describeDBOrBlock(eng *Engine, sess *Session, tokens []string) string {
	subCmd := strings.ToUpper(tokens[1])
	switch subCmd {
	case "DB":
		if len(tokens) < 3 {
			return "ERROR: requires database name"
		}
		info, err := eng.DescribeDB(tokens[2])
		if err != nil {
			return "ERROR: " + err.Error()
		}
		return jsonOut(info)

	case "BLOCK":
		db, block, err := resolveDBBlock(sess, tokens, 2)
		if err != nil {
			return "ERROR: " + err.Error()
		}
		info, err := eng.DescribeBlock(db, block)
		if err != nil {
			return "ERROR: " + err.Error()
		}
		return jsonOut(info)

	default:
		return "ERROR: unknown object: " + subCmd + " (expected DB or BLOCK)"
	}
}

func handleInfo(eng *Engine, sess *Session, tokens []string) string {
	return describeDBOrBlock(eng, sess, tokens)
}

func handleDescribe(eng *Engine, sess *Session, tokens []string) string {
	return describeDBOrBlock(eng, sess, tokens)
}

// handleStats implements "STATS DB [<name>]", matching the OBJECT-keyword
// syntax documented in help.go / docs/nql-reference.md and used by every
// other admin command (ANALYZE, OPTIMIZE, INFO, DESCRIBE, ...).
func handleStats(eng *Engine, sess *Session, tokens []string) string {
	if len(tokens) < 2 {
		return "ERROR: STATS requires object (DB)"
	}
	subCmd := strings.ToUpper(tokens[1])
	switch subCmd {
	case "DB":
		db := sess.currentDB
		if len(tokens) >= 3 {
			db = tokens[2]
		}
		if db == "" {
			return "ERROR: no database specified and no database in use"
		}
		if !eng.dirMgr.DBExists(db) {
			return "ERROR: database not found: " + db
		}
		stats, err := eng.StatsDB(db)
		if err != nil {
			return "ERROR: " + err.Error()
		}
		return jsonOut(stats)

	default:
		return "ERROR: unknown STATS object: " + subCmd + " (expected DB)"
	}
}

// handleSize implements "SIZE DB [<name>]" and "SIZE BLOCK [<db>] <name>",
// matching the OBJECT-keyword syntax documented in help.go /
// docs/nql-reference.md. SIZE BLOCK reads size_bytes via DescribeBlock so
// it reports the exact same figure as SHOW BLOCKS, instead of an
// independently-computed directory walk.
func handleSize(eng *Engine, sess *Session, tokens []string) string {
	if len(tokens) < 2 {
		return "ERROR: SIZE requires object (DB or BLOCK)"
	}
	subCmd := strings.ToUpper(tokens[1])
	switch subCmd {
	case "DB":
		db := sess.currentDB
		if len(tokens) >= 3 {
			db = tokens[2]
		}
		if db == "" {
			return "ERROR: no database specified and no database in use"
		}
		if !eng.dirMgr.DBExists(db) {
			return "ERROR: database not found: " + db
		}
		sz, err := dirSize(eng.dirMgr.DBPath(db))
		if err != nil {
			return "ERROR: " + err.Error()
		}
		return "Size of " + db + ": " + fmtBytes(sz)

	case "BLOCK":
		db, block, err := resolveDBBlock(sess, tokens, 2)
		if err != nil {
			return "ERROR: " + err.Error()
		}
		if !eng.dirMgr.DBExists(db) {
			return "ERROR: database not found: " + db
		}
		info, err := eng.DescribeBlock(db, block)
		if err != nil {
			return "ERROR: " + err.Error()
		}
		var sizeBytes int64
		if v, ok := info["size_bytes"].(int64); ok {
			sizeBytes = v
		}
		return "Size of " + block + " in " + db + ": " + fmtBytes(sizeBytes)

	default:
		return "ERROR: unknown SIZE object: " + subCmd + " (expected DB or BLOCK)"
	}
}

func handleRebuild(eng *Engine, sess *Session, tokens []string) string {
	if len(tokens) < 2 {
		return "ERROR: REBUILD requires object (BLOCK)"
	}
	subCmd := strings.ToUpper(tokens[1])
	switch subCmd {
	case "BLOCK":
		db, block, err := resolveDBBlock(sess, tokens, 2)
		if err != nil {
			return "ERROR: " + err.Error()
		}
		if err := eng.RebuildBlock(db, block); err != nil {
			return "ERROR: " + err.Error()
		}
		return "Block indexes rebuilt: " + block + " in database: " + db
	default:
		return "ERROR: unknown REBUILD object: " + subCmd
	}
}

func handleCheck(eng *Engine, sess *Session, tokens []string) string {
	if len(tokens) < 2 {
		return "ERROR: CHECK requires object (BLOCK)"
	}
	subCmd := strings.ToUpper(tokens[1])
	switch subCmd {
	case "BLOCK":
		db, block, err := resolveDBBlock(sess, tokens, 2)
		if err != nil {
			return "ERROR: " + err.Error()
		}
		report, err := eng.CheckBlock(db, block)
		if err != nil {
			return "ERROR: " + err.Error()
		}
		return jsonOut(report)
	default:
		return "ERROR: unknown CHECK object: " + subCmd
	}
}

func handleRepair(eng *Engine, sess *Session, tokens []string) string {
	if len(tokens) < 2 {
		return "ERROR: REPAIR requires object (BLOCK)"
	}
	subCmd := strings.ToUpper(tokens[1])
	switch subCmd {
	case "BLOCK":
		db, block, err := resolveDBBlock(sess, tokens, 2)
		if err != nil {
			return "ERROR: " + err.Error()
		}
		if err := eng.RepairBlock(db, block); err != nil {
			return "ERROR: " + err.Error()
		}
		return "Block repaired: " + block + " in database: " + db
	default:
		return "ERROR: unknown REPAIR object: " + subCmd
	}
}

func handleFlexCommand(eng *Engine, sess *Session, tokens []string) string {
	if len(tokens) < 2 {
		return "ERROR: FLEX requires a sub-command (STATS, HOT)"
	}
	subCmd := strings.ToUpper(tokens[1])
	switch subCmd {
	case "STATS":
		return jsonOut(eng.flexEngine.Stats())

	case "HOT":
		db := sess.currentDB
		block := ""
		if len(tokens) >= 3 {
			block = tokens[2]
		}
		if db == "" {
			return "ERROR: no database in use"
		}
		hot := eng.flexEngine.HotFields(db, block)
		if len(hot) == 0 {
			return "No hot fields recorded"
		}
		return jsonOut(hot)

	default:
		return "ERROR: unknown FLEX sub-command: " + subCmd
	}
}

func handleTransaction(eng *Engine, sess *Session, tokens []string) string {
	cmd := strings.ToUpper(tokens[0])
	switch cmd {
	case "BEGIN":
		if sess.currentTx != nil && sess.currentTx.Status == "pending" {
			return "ERROR: a transaction is already active (id: " + sess.currentTx.ID + ")"
		}
		db := sess.currentDB
		block := ""
		if len(tokens) >= 2 {
			db = tokens[1]
		}
		if len(tokens) >= 3 {
			block = tokens[2]
		}
		if db == "" {
			return "ERROR: no database specified and no database in use"
		}
		tx, err := eng.txManager.Begin(db, block)
		if err != nil {
			return "ERROR: " + err.Error()
		}
		sess.currentTx = tx
		return "Transaction started: " + tx.ID

	case "COMMIT":
		if sess.currentTx == nil {
			return "ERROR: no active transaction"
		}
		tx := sess.currentTx
		if err := eng.txManager.Commit(tx); err != nil {
			return "ERROR: " + err.Error()
		}
		sess.currentTx = nil
		return "Transaction committed: " + tx.ID

	case "ROLLBACK", "ABORT":
		if sess.currentTx == nil {
			return "ERROR: no active transaction"
		}
		tx := sess.currentTx
		if err := eng.txManager.Abort(tx); err != nil {
			return "ERROR: " + err.Error()
		}
		sess.currentTx = nil
		return "Transaction aborted: " + tx.ID

	case "TX", "TRANSACTION":
		sub := ""
		if len(tokens) >= 2 {
			sub = strings.ToUpper(tokens[1])
		}
		switch sub {
		case "LIST":
			active := eng.txManager.GetActiveTransactions()
			if len(active) == 0 {
				return "No active transactions"
			}
			return "Active transactions:\n  " + strings.Join(active, "\n  ")

		case "STATS":
			return jsonOut(eng.txManager.Stats())

		default:
			if sess.currentTx == nil {
				return "No active transaction in this session"
			}
			return fmt.Sprintf("Current transaction: %s (db: %s, block: %s, status: %s)",
				sess.currentTx.ID, sess.currentTx.DB, sess.currentTx.Block, sess.currentTx.Status)
		}

	default:
		return "ERROR: unknown transaction command: " + cmd
	}
}
