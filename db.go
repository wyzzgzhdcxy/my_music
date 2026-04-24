package main

import (
	"database/sql"
	"log"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func openLyricDB(dataDir string) (*sql.DB, error) {
	dbPath := filepath.Join(dataDir, "lyrics.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1) // SQLite is single-writer

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS lyrics (
		filename TEXT NOT NULL UNIQUE,
		content TEXT NOT NULL,
		created_at INTEGER DEFAULT (strftime('%s', 'now'))
	)`)
	if err != nil {
		db.Close()
		return nil, err
	}

	log.Printf("[歌词DB] 数据库已打开: %s", dbPath)
	return db, nil
}

func getLyricFromDB(db *sql.DB, filename string) string {
	var content string
	err := db.QueryRow(
		"SELECT content FROM lyrics WHERE filename = ?",
		filename,
	).Scan(&content)
	if err != nil {
		return ""
	}
	log.Printf("[歌词DB] 命中: %s", filename)
	return content
}

func saveLyricToDB(db *sql.DB, filename, content string) {
	_, err := db.Exec(
		"INSERT OR REPLACE INTO lyrics (filename, content) VALUES (?, ?)",
		filename, content,
	)
	if err != nil {
		log.Printf("[歌词DB] 保存失败: %s, err=%v", filename, err)
		return
	}
	log.Printf("[歌词DB] 已保存: %s", filename)
}
