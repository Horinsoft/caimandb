// Package storage holds CaimanDB's on-disk storage primitives: the
// per-database/per-block Badger handle pool, directory layout management,
// buffer pooling, external (large-document) blob storage, and payload
// compression. It has no dependency on the query/cluster/replication
// layers, which are the caller's responsibility.
package storage


import (
	"caimandb/internal/caimandb/logging"
	"encoding/json"
	"go.uber.org/zap"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type DirectoryManager struct {
	dataRoot string
	mu       sync.RWMutex
}

func NewDirectoryManager(dataRoot string) *DirectoryManager {
	return &DirectoryManager{
		dataRoot: dataRoot,
	}
}

func (dm *DirectoryManager) Init() error {
	if err := os.MkdirAll(dm.dataRoot, 0750); err != nil {
		return err
	}

	systemDirs := []string{
		filepath.Join(dm.dataRoot, "__users"),
		filepath.Join(dm.dataRoot, "__raft"),
		filepath.Join(dm.dataRoot, "__shards"),
		filepath.Join(dm.dataRoot, "__cluster"),
		filepath.Join(dm.dataRoot, "__system"),
		filepath.Join(dm.dataRoot, "backups"),
	}

	for _, dir := range systemDirs {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return err
		}
	}

	return nil
}

func (dm *DirectoryManager) DBPath(name string) string {
	return filepath.Join(dm.dataRoot, name)
}

// BackupsPath returns the path to the shared backups/ directory where
// EXPORT/BACKUP artifacts (.csv, .json, .bak, etc.) are stored, creating
// it on demand in case Init() ran before this directory existed.
func (dm *DirectoryManager) BackupsPath() string {
	path := filepath.Join(dm.dataRoot, "backups")
	_ = os.MkdirAll(path, 0750)
	return path
}

func (dm *DirectoryManager) DBExists(name string) bool {
	_, err := os.Stat(dm.DBPath(name))
	return err == nil
}

func (dm *DirectoryManager) BlockPath(dbName, blockName string) string {
	return filepath.Join(dm.DBPath(dbName), blockName)
}

func (dm *DirectoryManager) BlockDataPath(dbName, blockName string) string {
	return filepath.Join(dm.BlockPath(dbName, blockName), "__data")
}

func (dm *DirectoryManager) BlockIndexPath(dbName, blockName string) string {
	return filepath.Join(dm.BlockPath(dbName, blockName), "__index")
}

func (dm *DirectoryManager) BlockMetaPath(dbName, blockName string) string {
	return filepath.Join(dm.BlockPath(dbName, blockName), "__meta")
}

func (dm *DirectoryManager) BlockExists(dbName, blockName string) bool {
	_, err := os.Stat(dm.BlockPath(dbName, blockName))
	return err == nil
}

func (dm *DirectoryManager) CreateBlockDirectories(dbName, blockName string) error {
	paths := []string{
		dm.BlockPath(dbName, blockName),
		dm.BlockDataPath(dbName, blockName),
		dm.BlockIndexPath(dbName, blockName),
		dm.BlockMetaPath(dbName, blockName),
	}

	for _, path := range paths {
		if err := os.MkdirAll(path, 0750); err != nil {
			return err
		}
	}

	return nil
}

func (dm *DirectoryManager) RemoveBlockDirectories(dbName, blockName string) error {
	return os.RemoveAll(dm.BlockPath(dbName, blockName))
}

func (dm *DirectoryManager) ListDatabases() ([]string, error) {
	entries, err := os.ReadDir(dm.dataRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var dbs []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), "__") {
			dbs = append(dbs, entry.Name())
		}
	}
	sort.Strings(dbs)
	return dbs, nil
}

func (dm *DirectoryManager) ListBlocks(dbName string) ([]string, error) {
	dbPath := dm.DBPath(dbName)
	entries, err := os.ReadDir(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var blocks []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), "__") {
			blocks = append(blocks, entry.Name())
		}
	}
	sort.Strings(blocks)
	return blocks, nil
}

func (dm *DirectoryManager) SaveBlockMeta(dbName, blockName string, meta BlockMeta) error {
	metaPath := dm.BlockMetaPath(dbName, blockName)
	// Mirrors SaveDBMeta's MkdirAll: don't assume __meta already exists.
	// A block created before this directory was part of the layout, or
	// one whose __meta didn't survive some other edge case, would
	// otherwise fail every save with "no such file or directory" and
	// never be repairable by EnsureBlockMeta.
	if err := os.MkdirAll(metaPath, 0750); err != nil {
		return err
	}
	metaFile := filepath.Join(metaPath, "block.json")

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metaFile, data, 0644)
}

func (dm *DirectoryManager) LoadBlockMeta(dbName, blockName string) (BlockMeta, error) {
	metaFile := filepath.Join(dm.BlockMetaPath(dbName, blockName), "block.json")

	data, err := os.ReadFile(metaFile)
	if err != nil {
		return BlockMeta{}, err
	}

	var meta BlockMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return BlockMeta{}, err
	}

	return meta, nil
}

// EnsureBlockMeta returns blockName's metadata, transparently repairing
// it first if needed instead of surfacing an error for something this
// package can safely fix on its own. Two situations are repaired:
//
//  1. block.json is missing or unreadable (corrupt JSON, wiped by hand,
//     never written by a pre-metadata block, or -- the case that
//     originally motivated this -- a rename whose directory move
//     dropped or raced with __meta for any reason): a fresh record is
//     rebuilt from what's still on disk (directory mtime for
//     CreatedAt, actual on-disk size for SizeBytes) and persisted.
//  2. block.json loads fine but its Name/DB fields no longer match
//     blockName/dbName: this happens after RENAME BLOCK or RENAME
//     DATABASE, since the byte content of an already-existing
//     block.json isn't rewritten just by moving the directory it lives
//     in. The fields are corrected in place and re-saved.
//
// Only the block's directory has to actually exist; DocCount is best-
// effort in the rebuild case (0, since it isn't cheaply recoverable
// from disk alone) but every other field is accurate immediately, and
// DocCount self-corrects on the next insert/update/delete since those
// paths load-modify-save through this same method.
func (dm *DirectoryManager) EnsureBlockMeta(dbName, blockName string) (BlockMeta, error) {
	meta, err := dm.LoadBlockMeta(dbName, blockName)
	if err == nil {
		if meta.Name == blockName && meta.DB == dbName {
			return meta, nil
		}
		logging.Log().Info("Correcting stale block metadata after rename",
			zap.String("db", dbName), zap.String("block", blockName),
			zap.String("meta_db", meta.DB), zap.String("meta_name", meta.Name))
		meta.Name = blockName
		meta.DB = dbName
		meta.UpdatedAt = time.Now().Unix()
		if saveErr := dm.SaveBlockMeta(dbName, blockName, meta); saveErr != nil {
			logging.Log().Warn("Could not persist corrected block metadata",
				zap.String("db", dbName), zap.String("block", blockName), zap.Error(saveErr))
		}
		return meta, nil
	}

	if !dm.BlockExists(dbName, blockName) {
		return BlockMeta{}, err
	}

	logging.Log().Warn("Block metadata missing or unreadable, rebuilding it",
		zap.String("db", dbName), zap.String("block", blockName), zap.Error(err))

	created := time.Now().Unix()
	if info, statErr := os.Stat(dm.BlockPath(dbName, blockName)); statErr == nil {
		created = info.ModTime().Unix()
	}
	size, sizeErr := blockDirSize(dm.BlockDataPath(dbName, blockName))
	if sizeErr != nil {
		size = 0
	}

	rebuilt := BlockMeta{
		Name:      blockName,
		DB:        dbName,
		CreatedAt: created,
		UpdatedAt: time.Now().Unix(),
		DocCount:  0,
		SizeBytes: size,
	}
	if saveErr := dm.SaveBlockMeta(dbName, blockName, rebuilt); saveErr != nil {
		logging.Log().Warn("Could not persist rebuilt block metadata",
			zap.String("db", dbName), zap.String("block", blockName), zap.Error(saveErr))
	}
	return rebuilt, nil
}

// blockDirSize sums file sizes under path. Errors from individual
// entries are ignored (best-effort, used only to seed a rebuilt
// SizeBytes) rather than aborting the whole walk.
func blockDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, walkErr error) error {
		if walkErr == nil && info != nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func (dm *DirectoryManager) SaveDBMeta(dbName string, meta map[string]any) error {
	metaPath := filepath.Join(dm.DBPath(dbName), "__meta")
	if err := os.MkdirAll(metaPath, 0750); err != nil {
		return err
	}

	metaFile := filepath.Join(metaPath, "db.json")
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metaFile, data, 0644)
}

func (dm *DirectoryManager) LoadDBMeta(dbName string) (map[string]any, error) {
	metaFile := filepath.Join(dm.DBPath(dbName), "__meta", "db.json")

	data, err := os.ReadFile(metaFile)
	if err != nil {
		return nil, err
	}

	var meta map[string]any
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	return meta, nil
}

// BlockMeta is the small on-disk metadata record kept alongside each
// block (doc count, size, timestamps). It lives here (rather than with
// Document) because it's read/written exclusively through
// DirectoryManager's Save/LoadBlockMeta.
type BlockMeta struct {
	Name      string `json:"name"`
	DB        string `json:"db"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
	DocCount  int64  `json:"doc_count"`
	SizeBytes int64  `json:"size_bytes"`
}
