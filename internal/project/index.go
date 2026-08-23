package project

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bg-dao/axon-codex-sceneops/internal/domain"
	_ "modernc.org/sqlite"
)

const indexSchema = `
PRAGMA journal_mode=WAL;
CREATE TABLE IF NOT EXISTS metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS scenes (id TEXT PRIMARY KEY, title TEXT NOT NULL, sort_order INTEGER NOT NULL, manifest_path TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS shots (id TEXT PRIMARY KEY, scene_id TEXT NOT NULL, title TEXT NOT NULL, sort_order INTEGER NOT NULL, manifest_path TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS assets (id TEXT PRIMARY KEY, shot_id TEXT, kind TEXT NOT NULL, sha256 TEXT NOT NULL, relative_path TEXT NOT NULL, manifest_path TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS runs (id TEXT PRIMARY KEY, operation TEXT NOT NULL, status TEXT NOT NULL, shot_id TEXT, updated_at TEXT NOT NULL, manifest_path TEXT NOT NULL);
`

func IndexPath(root string) (string, error) {
	return ResolveRelative(root, filepath.Join(".sceneops", "index.sqlite"))
}

func RebuildIndex(root string) error {
	snapshot, err := NewStore().Open(root)
	if err != nil {
		return err
	}
	indexPath, err := IndexPath(root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", indexPath)
	if err != nil {
		return fmt.Errorf("open index: %w", err)
	}
	defer db.Close()
	if _, err := db.Exec(indexSchema); err != nil {
		return fmt.Errorf("create index schema: %w", err)
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, table := range []string{"metadata", "scenes", "shots", "assets", "runs"} {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil {
			return err
		}
	}
	projectJSON, _ := json.Marshal(snapshot.Project)
	if _, err := tx.Exec("INSERT INTO metadata(key, value) VALUES(?, ?)", "project", string(projectJSON)); err != nil {
		return err
	}
	for _, scene := range snapshot.Scenes {
		if _, err := tx.Exec("INSERT INTO scenes(id,title,sort_order,manifest_path) VALUES(?,?,?,?)", scene.ID, scene.Title, scene.Order, filepath.ToSlash(filepath.Join("scenes", scene.ID+".json"))); err != nil {
			return err
		}
	}
	for _, shot := range snapshot.Shots {
		if _, err := tx.Exec("INSERT INTO shots(id,scene_id,title,sort_order,manifest_path) VALUES(?,?,?,?,?)", shot.ID, shot.SceneID, shot.Title, shot.Order, filepath.ToSlash(filepath.Join("shots", shot.ID+".json"))); err != nil {
			return err
		}
	}
	for _, asset := range snapshot.Assets {
		if _, err := tx.Exec("INSERT INTO assets(id,shot_id,kind,sha256,relative_path,manifest_path) VALUES(?,?,?,?,?,?)", asset.ID, asset.ShotID, asset.Kind, asset.SHA256, asset.RelativePath, filepath.ToSlash(filepath.Join("assets", asset.ID, "asset.json"))); err != nil {
			return err
		}
	}
	for _, run := range snapshot.Runs {
		if _, err := tx.Exec("INSERT INTO runs(id,operation,status,shot_id,updated_at,manifest_path) VALUES(?,?,?,?,?,?)", run.ID, run.Operation, run.Status, run.ShotID, run.UpdatedAt.Format("2006-01-02T15:04:05.999999999Z07:00"), filepath.ToSlash(filepath.Join("runs", run.ID+".json"))); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func CountIndexed(root, table string) (int, error) {
	allowed := map[string]bool{"scenes": true, "shots": true, "assets": true, "runs": true}
	if !allowed[table] {
		return 0, fmt.Errorf("unsupported index table %q", table)
	}
	path, err := IndexPath(root)
	if err != nil {
		return 0, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
	return count, err
}

var _ = domain.SchemaVersion
