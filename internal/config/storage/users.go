package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// users.go
//
// Всё по таблице users:
// - EnsureUser / UpsertUser
// - Lang
// - ThreadID
// - GetUser...

func (s *SQLiteStore) EnsureUser(ctx context.Context, u User) error {
	q := `
INSERT INTO users (user_id, chat_id, username, first_name, last_name, updated_at)
VALUES (?, ?, ?, ?, ?, datetime('now'))
ON CONFLICT(user_id) DO UPDATE SET
  chat_id    = excluded.chat_id,
  username   = excluded.username,
  first_name = excluded.first_name,
  last_name  = excluded.last_name,
  updated_at = datetime('now');
`
	_, err := s.db.ExecContext(ctx, q,
		u.UserID, u.ChatID,
		nullStringOrNil(u.Username),
		nullStringOrNil(u.FirstName),
		nullStringOrNil(u.LastName),
	)
	return err
}

// Алиас, чтобы service мог звать UpsertUser (и всё компилилось)
func (s *SQLiteStore) UpsertUser(ctx context.Context, u User) error {
	return s.EnsureUser(ctx, u)
}

func (s *SQLiteStore) SetLang(ctx context.Context, userID int64, lang string) error {
	lang = strings.ToUpper(strings.TrimSpace(lang))
	if lang == "" {
		return errors.New("SetLang: lang empty")
	}

	// гарантируем запись пользователя (чтобы UPDATE не дал 0 rows)
	_, _ = s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO users (user_id, chat_id, created_at, updated_at)
		 VALUES (?, ?, datetime('now'), datetime('now'))`,
		userID, userID,
	)

	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET lang = ?, updated_at = datetime('now') WHERE user_id = ?`,
		lang, userID,
	)
	if err != nil {
		return fmt.Errorf("SetLang: %w", err)
	}

	aff, _ := res.RowsAffected()
	if aff == 0 {
		return errors.New("SetLang: user not found")
	}
	return nil
}

func (s *SQLiteStore) GetLangByUserID(ctx context.Context, userID int64) (string, bool, error) {
	var l sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT lang FROM users WHERE user_id = ? LIMIT 1`, userID).Scan(&l)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if !l.Valid || strings.TrimSpace(l.String) == "" {
		return "", false, nil
	}
	return strings.ToUpper(strings.TrimSpace(l.String)), true, nil
}

func (s *SQLiteStore) GetThreadByUserID(ctx context.Context, userID int64) (int, bool, error) {
	var tid sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT thread_id FROM users WHERE user_id = ?`, userID).Scan(&tid)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if !tid.Valid || tid.Int64 == 0 {
		return 0, false, nil
	}
	return int(tid.Int64), true, nil
}

func (s *SQLiteStore) SetThreadID(ctx context.Context, userID int64, threadID int) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET thread_id = ?, updated_at = datetime('now') WHERE user_id = ?`,
		threadID, userID,
	)
	if err != nil {
		return err
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		return errors.New("SetThreadID: user not found")
	}
	return nil
}

func (s *SQLiteStore) GetUserByThreadID(ctx context.Context, threadID int) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id, chat_id, username, first_name, last_name, lang, thread_id
		FROM users
		WHERE thread_id = ?
		LIMIT 1
	`, threadID).Scan(
		&u.UserID,
		&u.ChatID,
		&u.Username,
		&u.FirstName,
		&u.LastName,
		&u.Lang,
		&u.ThreadID,
	)
	if err != nil {
		return User{}, fmt.Errorf("GetUserByThreadID: %w", err)
	}
	return u, nil
}

func (s *SQLiteStore) GetUserByUserID(ctx context.Context, userID int64) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id, chat_id, username, first_name, last_name, lang, thread_id
		FROM users
		WHERE user_id = ?
		LIMIT 1
	`, userID).Scan(
		&u.UserID,
		&u.ChatID,
		&u.Username,
		&u.FirstName,
		&u.LastName,
		&u.Lang,
		&u.ThreadID,
	)
	if err != nil {
		return User{}, fmt.Errorf("GetUserByUserID: %w", err)
	}
	return u, nil
}

// ===== helpers =====

func nullStringOrNil(v sql.NullString) any {
	if !v.Valid || strings.TrimSpace(v.String) == "" {
		return nil
	}
	return strings.TrimSpace(v.String)
}

func nullInt64OrNil(v sql.NullInt64) any {
	if !v.Valid {
		return nil
	}
	return v.Int64
}
