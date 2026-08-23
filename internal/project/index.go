package project

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

const indexSchema = `
PRAGMA journal_mode=DELETE;
CREATE TABLE metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE episodes (id TEXT PRIMARY KEY, number INTEGER NOT NULL, title TEXT NOT NULL, status TEXT NOT NULL, manifest_path TEXT NOT NULL);
CREATE TABLE characters (id TEXT PRIMARY KEY, name TEXT NOT NULL, manifest_path TEXT NOT NULL);
CREATE TABLE locations (id TEXT PRIMARY KEY, name TEXT NOT NULL, manifest_path TEXT NOT NULL);
CREATE TABLE props (id TEXT PRIMARY KEY, name TEXT NOT NULL, manifest_path TEXT NOT NULL);
CREATE TABLE scenes (id TEXT PRIMARY KEY, episode_id TEXT NOT NULL, title TEXT NOT NULL, sort_order INTEGER NOT NULL, manifest_path TEXT NOT NULL);
CREATE TABLE shots (id TEXT PRIMARY KEY, episode_id TEXT NOT NULL, scene_id TEXT NOT NULL, title TEXT NOT NULL, sort_order INTEGER NOT NULL, manifest_path TEXT NOT NULL);
CREATE TABLE assets (id TEXT PRIMARY KEY, episode_id TEXT, shot_id TEXT, script_block_id TEXT, kind TEXT NOT NULL, sha256 TEXT NOT NULL, relative_path TEXT NOT NULL, manifest_path TEXT NOT NULL);
CREATE TABLE runs (id TEXT PRIMARY KEY, operation TEXT NOT NULL, status TEXT NOT NULL, episode_id TEXT, shot_id TEXT, updated_at TEXT NOT NULL, manifest_path TEXT NOT NULL);
`

var rebuildIndexMu sync.Mutex

func IndexPath(root string) (string, error) {
	return ResolveRelative(root, filepath.Join(".dramaops", "index.sqlite"))
}

func RebuildIndex(root string) error {
	rebuildIndexMu.Lock()
	defer rebuildIndexMu.Unlock()

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
	temporary, err := os.CreateTemp(filepath.Dir(indexPath), ".dramaops-index-rebuild-*.sqlite")
	if err != nil {
		return fmt.Errorf("create temporary index: %w", err)
	}
	temporaryPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return err
	}
	defer func() {
		_ = os.Remove(temporaryPath)
		_ = os.Remove(temporaryPath + "-wal")
		_ = os.Remove(temporaryPath + "-shm")
	}()

	db, err := sql.Open("sqlite", temporaryPath)
	if err != nil {
		return fmt.Errorf("open index: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(indexSchema); err != nil {
		_ = db.Close()
		return fmt.Errorf("create index schema: %w", err)
	}
	tx, err := db.Begin()
	if err != nil {
		_ = db.Close()
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
		_ = db.Close()
	}()
	projectJSON, _ := json.Marshal(snapshot.Project)
	if _, err := tx.Exec("INSERT INTO metadata(key,value) VALUES(?,?)", "project", string(projectJSON)); err != nil {
		return err
	}
	for _, value := range snapshot.Episodes {
		if _, err := tx.Exec("INSERT INTO episodes(id,number,title,status,manifest_path) VALUES(?,?,?,?,?)", value.ID, value.Number, value.Title, value.Status, filepath.ToSlash(filepath.Join("episodes", value.ID, "episode.json"))); err != nil {
			return err
		}
	}
	for _, value := range snapshot.Characters {
		if _, err := tx.Exec("INSERT INTO characters(id,name,manifest_path) VALUES(?,?,?)", value.ID, value.Name, filepath.ToSlash(filepath.Join("characters", value.ID+".json"))); err != nil {
			return err
		}
	}
	for _, value := range snapshot.Locations {
		if _, err := tx.Exec("INSERT INTO locations(id,name,manifest_path) VALUES(?,?,?)", value.ID, value.Name, filepath.ToSlash(filepath.Join("locations", value.ID+".json"))); err != nil {
			return err
		}
	}
	for _, value := range snapshot.Props {
		if _, err := tx.Exec("INSERT INTO props(id,name,manifest_path) VALUES(?,?,?)", value.ID, value.Name, filepath.ToSlash(filepath.Join("props", value.ID+".json"))); err != nil {
			return err
		}
	}
	for _, value := range snapshot.Scenes {
		if _, err := tx.Exec("INSERT INTO scenes(id,episode_id,title,sort_order,manifest_path) VALUES(?,?,?,?,?)", value.ID, value.EpisodeID, value.Title, value.Order, filepath.ToSlash(filepath.Join("scenes", value.ID+".json"))); err != nil {
			return err
		}
	}
	for _, value := range snapshot.Shots {
		if _, err := tx.Exec("INSERT INTO shots(id,episode_id,scene_id,title,sort_order,manifest_path) VALUES(?,?,?,?,?,?)", value.ID, value.EpisodeID, value.SceneID, value.Title, value.Order, filepath.ToSlash(filepath.Join("shots", value.ID+".json"))); err != nil {
			return err
		}
	}
	for _, value := range snapshot.Assets {
		if _, err := tx.Exec("INSERT INTO assets(id,episode_id,shot_id,script_block_id,kind,sha256,relative_path,manifest_path) VALUES(?,?,?,?,?,?,?,?)", value.ID, value.EpisodeID, value.ShotID, value.ScriptBlockID, value.Kind, value.SHA256, value.RelativePath, filepath.ToSlash(filepath.Join("assets", value.ID, "asset.json"))); err != nil {
			return err
		}
	}
	for _, value := range snapshot.Runs {
		if _, err := tx.Exec("INSERT INTO runs(id,operation,status,episode_id,shot_id,updated_at,manifest_path) VALUES(?,?,?,?,?,?,?)", value.ID, value.Operation, value.Status, value.EpisodeID, value.ShotID, value.UpdatedAt.Format("2006-01-02T15:04:05.999999999Z07:00"), filepath.ToSlash(filepath.Join("runs", value.ID+".json"))); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	if err := db.Close(); err != nil {
		return fmt.Errorf("close rebuilt index: %w", err)
	}
	_ = os.Remove(indexPath + "-wal")
	_ = os.Remove(indexPath + "-shm")
	if err := os.Rename(temporaryPath, indexPath); err != nil {
		return fmt.Errorf("install rebuilt index: %w", err)
	}
	return nil
}

func CountIndexed(root, table string) (int, error) {
	allowed := map[string]bool{"episodes": true, "characters": true, "locations": true, "props": true, "scenes": true, "shots": true, "assets": true, "runs": true}
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
