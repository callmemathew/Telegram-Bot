package storage

import (
	"context"
	"database/sql"

	_ "modernc.org/sqlite"
)

// store.go
//
// База + инициализация схемы.
// Тут НЕТ бизнес-логики. Только Open/Init/New.

type SQLiteStore struct {
	db *sql.DB
}

func OpenSQLite(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

func (s *SQLiteStore) Init(ctx context.Context) error {
	schema := `
CREATE TABLE IF NOT EXISTS users (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL UNIQUE,
  chat_id    INTEGER NOT NULL,
  username   TEXT,
  first_name TEXT,
  last_name  TEXT,
  lang       TEXT,
  thread_id  INTEGER,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_users_user_id   ON users(user_id);
CREATE INDEX IF NOT EXISTS idx_users_thread_id ON users(thread_id);

CREATE TABLE IF NOT EXISTS support_sessions (
  user_id          INTEGER PRIMARY KEY,
  thread_id        INTEGER NOT NULL DEFAULT 0,
  manager_id       INTEGER,
  manager_first    TEXT,
  manager_last     TEXT,
  manager_username TEXT,
  user_header_msg  INTEGER,
  pinned_msg_id    INTEGER,
  updated_at       TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_sessions_thread_id ON support_sessions(thread_id);
`
	_, err := s.db.ExecContext(ctx, schema)
	return err
}
