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
// - EnsureUser / UpsertUser (профиль)
// - Lang
// - ThreadID
// - PinnedMsgID
// - GetUser...

func (s *SQLiteStore) EnsureUser(ctx context.Context, u User) error {
	q := `
INSERT INTO users (telegram_user_id, chat_id, username, first_name, last_name, updated_at)
VALUES (?, ?, ?, ?, ?, datetime('now'))
ON CONFLICT(telegram_user_id) DO UPDATE SET
  chat_id    = excluded.chat_id,
  username   = excluded.username,
  first_name = excluded.first_name,
  last_name  = excluded.last_name,
  updated_at = datetime('now');
`
	_, err := s.db.ExecContext(ctx, q,
		u.TelegramUserID,
		u.ChatID,
		nullStringOrNil(u.Username),
		nullStringOrNil(u.FirstName),
		nullStringOrNil(u.LastName),
	)
	return err
}

// Алиас, чтобы service мог звать UpsertUser
func (s *SQLiteStore) UpsertUser(ctx context.Context, u User) error {
	return s.EnsureUser(ctx, u)
}

func (s *SQLiteStore) SetLang(ctx context.Context, userID int64, lang string) error {
	lang = strings.ToUpper(strings.TrimSpace(lang))
	if lang == "" {
		return errors.New("SetLang: lang empty")
	}

	// гарантируем запись пользователя
	_, _ = s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO users (telegram_user_id, chat_id, created_at, updated_at)
		 VALUES (?, ?, datetime('now'), datetime('now'))`,
		userID, userID,
	)

	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET lang = ?, updated_at = datetime('now') WHERE telegram_user_id = ?`,
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
	err := s.db.QueryRowContext(ctx, `SELECT lang FROM users WHERE telegram_user_id = ? LIMIT 1`, userID).Scan(&l)
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
	err := s.db.QueryRowContext(ctx, `SELECT thread_id FROM users WHERE telegram_user_id = ?`, userID).Scan(&tid)
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
	// гарантируем запись пользователя (на случай если его ещё не было)
	_, _ = s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO users (telegram_user_id, chat_id, created_at, updated_at)
		 VALUES (?, ?, datetime('now'), datetime('now'))`,
		userID, userID,
	)

	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET thread_id = ?, updated_at = datetime('now') WHERE telegram_user_id = ?`,
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

func (s *SQLiteStore) GetPinnedMsgID(ctx context.Context, telegramUserID int64) (int, bool, error) {
	var v sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT pinned_msg_id
		FROM users
		WHERE telegram_user_id = ?
		LIMIT 1
	`, telegramUserID).Scan(&v)

	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if !v.Valid || v.Int64 == 0 {
		return 0, false, nil
	}
	return int(v.Int64), true, nil
}

func (s *SQLiteStore) SetPinnedMsgID(ctx context.Context, telegramUserID int64, msgID int) error {
	_, _ = s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO users (telegram_user_id, chat_id, created_at, updated_at)
		VALUES (?, ?, datetime('now'), datetime('now'))
	`, telegramUserID, telegramUserID)

	_, err := s.db.ExecContext(ctx, `
		UPDATE users
		SET pinned_msg_id = ?, updated_at = datetime('now')
		WHERE telegram_user_id = ?
	`, msgID, telegramUserID)

	return err
}

func (s *SQLiteStore) GetUserByThreadID(ctx context.Context, threadID int) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `
		SELECT telegram_user_id, chat_id, username, first_name, last_name, lang, thread_id, pinned_msg_id
		FROM users
		WHERE thread_id = ?
		LIMIT 1
	`, threadID).Scan(
		&u.TelegramUserID,
		&u.ChatID,
		&u.Username,
		&u.FirstName,
		&u.LastName,
		&u.Lang,
		&u.ThreadID,
		&u.PinnedMsgID,
	)
	if err != nil {
		return User{}, fmt.Errorf("GetUserByThreadID: %w", err)
	}
	return u, nil
}

func (s *SQLiteStore) GetUserByUserID(ctx context.Context, userID int64) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `
		SELECT telegram_user_id, chat_id, username, first_name, last_name, lang,
		       thread_id, status_msg_id, pinned_msg_id
		FROM users
		WHERE telegram_user_id = ?
		LIMIT 1
	`, userID).Scan(
		&u.TelegramUserID,
		&u.ChatID,
		&u.Username,
		&u.FirstName,
		&u.LastName,
		&u.Lang,
		&u.ThreadID,
		&u.StatusMsgID,
		&u.PinnedMsgID,
	)
	if err != nil {
		return User{}, fmt.Errorf("GetUserByTelegramUserID: %w", err)
	}
	return u, nil
}
func (s *SQLiteStore) GetStatusMsgID(ctx context.Context, telegramUserID int64) (int, bool, error) {
	var v sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT status_msg_id
		FROM users
		WHERE telegram_user_id = ?
		LIMIT 1
	`, telegramUserID).Scan(&v)

	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if !v.Valid || v.Int64 == 0 {
		return 0, false, nil
	}
	return int(v.Int64), true, nil
}

func (s *SQLiteStore) SetStatusMsgID(ctx context.Context, telegramUserID int64, msgID int) error {
	// гарантируем строку
	_, _ = s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO users (telegram_user_id, chat_id, created_at, updated_at)
		VALUES (?, ?, datetime('now'), datetime('now'))
	`, telegramUserID, telegramUserID)

	_, err := s.db.ExecContext(ctx, `
		UPDATE users
		SET status_msg_id = ?, updated_at = datetime('now')
		WHERE telegram_user_id = ?
	`, msgID, telegramUserID)

	return err
}

// ===== helpers =====

func nullStringOrNil(v sql.NullString) any {
	if !v.Valid || strings.TrimSpace(v.String) == "" {
		return nil
	}
	return strings.TrimSpace(v.String)
}
