package common

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

func InitDB() {
	// Ensure data directory exists.
	dataDir := "./data"
	_ = os.MkdirAll(dataDir, 0755)

	dbPath := filepath.Join(dataDir, "lan_chat.db")

	var err error
	DB, err = sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatal("Database open failed:", err)
	}

	if err = DB.Ping(); err != nil {
		log.Fatal("Database ping failed:", err)
	}

	log.Println("Database connected:", dbPath)
}

func InitTables() {
	userTable := `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT NOT NULL UNIQUE,
		device_type TEXT NOT NULL DEFAULT 'web',
		device_name TEXT DEFAULT '',
		ip_address TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_seen DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	roomTable := `
	CREATE TABLE IF NOT EXISTS rooms (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	messageTable := `
	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		room_id TEXT NOT NULL DEFAULT 'default',
		sender_id TEXT NOT NULL,
		sender_name TEXT NOT NULL,
		content TEXT DEFAULT '',
		msg_type TEXT DEFAULT 'text',
		file_url TEXT DEFAULT '',
		file_name TEXT DEFAULT '',
		file_size INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	fileTable := `
	CREATE TABLE IF NOT EXISTS files (
		id TEXT PRIMARY KEY,
		original_name TEXT NOT NULL,
		stored_name TEXT NOT NULL,
		file_size INTEGER NOT NULL DEFAULT 0,
		file_type TEXT DEFAULT '',
		uploader_id TEXT NOT NULL,
		uploader_name TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	tables := []string{userTable, roomTable, messageTable, fileTable}
	for _, table := range tables {
		if _, err := DB.Exec(table); err != nil {
			log.Fatal("Create table failed:", err)
		}
	}

	log.Println("Database tables ready")
}
