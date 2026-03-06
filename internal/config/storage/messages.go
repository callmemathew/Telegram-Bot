package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// messages.go
//
// Таблица messages хранит всю переписку:
// - user -> manager
// - manager -> user
//
// Каждое сообщение = одна строка в БД.

type Message struct {
	ID             int64
	TelegramUserID int64
	Direction      string // "user" | "manager"

	Text      sql.NullString
	HasMedia  int
	MediaType sql.NullString
	FileID    sql.NullString

	CreatedAt string
}

// InitMessages создаёт таблицу messages
func (s *SQLiteStore) InitMessages(ctx context.Context) error {
	q := `
CREATE TABLE IF NOT EXISTS messages (
  id               INTEGER PRIMARY KEY AUTOINCREMENT,
  telegram_user_id INTEGER NOT NULL,
  direction        TEXT NOT NULL,
  text             TEXT,
  has_media        INTEGER NOT NULL DEFAULT 0,
  media_type       TEXT,
  file_id          TEXT,
  created_at       TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_messages_user_id
  ON messages(telegram_user_id);

CREATE INDEX IF NOT EXISTS idx_messages_created_at
  ON messages(created_at);
`
	_, err := s.db.ExecContext(ctx, q)
	return err
}

// SaveMessage сохраняет одно сообщение в таблицу messages
func (s *SQLiteStore) SaveMessage(ctx context.Context, m Message) error {
	q := `
INSERT INTO messages (
  telegram_user_id,
  direction,
  text,
  has_media,
  media_type,
  file_id
) VALUES (?, ?, ?, ?, ?, ?)
`
	_, err := s.db.ExecContext(ctx, q,
		m.TelegramUserID,
		m.Direction,
		nullStringOrNil(m.Text),
		m.HasMedia,
		nullStringOrNil(m.MediaType),
		nullStringOrNil(m.FileID),
	)
	return err
}

// ListMessagesByUser можно использовать для проверки — получить последние сообщения юзера
func (s *SQLiteStore) ListMessagesByUser(ctx context.Context, telegramUserID int64, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}

	q := `
SELECT id, telegram_user_id, direction, text, has_media, media_type, file_id, created_at
FROM messages
WHERE telegram_user_id = ?
ORDER BY id DESC
LIMIT ?
`

	rows, err := s.db.QueryContext(ctx, q, telegramUserID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(
			&m.ID,
			&m.TelegramUserID,
			&m.Direction,
			&m.Text,
			&m.HasMedia,
			&m.MediaType,
			&m.FileID,
			&m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		out = append(out, m)
	}

	return out, rows.Err()
}
