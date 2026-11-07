package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
}

func InitDB(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("không thể tạo thư mục dữ liệu: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("không thể mở SQLite database: %w", err)
	}

	// Cấu hình connection pool tối ưu cho SQLite
	db.SetMaxOpenConns(1) // SQLite hoạt động an toàn nhất ở chế độ 1 writer

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("không thể ping database: %w", err)
	}

	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("lỗi chạy migration: %w", err)
	}

	log.Printf("[Database] Khởi tạo SQLite thành công tại: %s", dbPath)
	return &DB{db}, nil
}

func runMigrations(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS sites (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		domain TEXT UNIQUE NOT NULL,
		root_path TEXT NOT NULL,
		php_version TEXT DEFAULT '8.2',
		wp_version TEXT,
		owner_user TEXT DEFAULT 'www-data',
		db_name TEXT,
		db_user TEXT,
		ssl_active INTEGER DEFAULT 0,
		ssl_expires_at DATETIME,
		fastcgi_cache INTEGER DEFAULT 0,
		is_discovered INTEGER DEFAULT 0,
		status TEXT DEFAULT 'active',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS site_plugins (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		site_id INTEGER NOT NULL,
		slug TEXT NOT NULL,
		name TEXT,
		version TEXT,
		update_version TEXT,
		status TEXT,
		has_update INTEGER DEFAULT 0,
		UNIQUE(site_id, slug),
		FOREIGN KEY(site_id) REFERENCES sites(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_site_plugins_site_slug ON site_plugins(site_id, slug);

	CREATE TABLE IF NOT EXISTS scan_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		site_id INTEGER NOT NULL,
		scan_type TEXT,
		file_path TEXT NOT NULL,
		issue_description TEXT,
		severity TEXT DEFAULT 'warning',
		detected_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(site_id) REFERENCES sites(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		site_id INTEGER,
		action TEXT NOT NULL,
		status TEXT DEFAULT 'pending',
		progress INTEGER DEFAULT 0,
		log_output TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return err
	}

	// Dọn dẹp dữ liệu trùng lặp cũ nếu có trong site_plugins
	cleanupDuplicates := `
	DELETE FROM site_plugins
	WHERE id NOT IN (
		SELECT MIN(id)
		FROM site_plugins
		GROUP BY site_id, slug
	);
	CREATE UNIQUE INDEX IF NOT EXISTS uq_site_plugins_site_slug ON site_plugins(site_id, slug);
	`
	_, _ = db.Exec(cleanupDuplicates)
	return nil
}

