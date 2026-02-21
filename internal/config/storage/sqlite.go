package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

type User struct {
	UserID    int64
	ChatID    int64
	Username  sql.NullString
	FirstName sql.NullString
	LastName  sql.NullString
	Lang      sql.NullString
	ThreadID  sql.NullInt64
}

type SessionStatus string

const (
	SessionWaiting SessionStatus = "waiting"
	SessionActive  SessionStatus = "active"
	SessionClosed  SessionStatus = "closed"
)

type SupportSession struct {
	UserID        int64
	ThreadID      int64
	Status        SessionStatus
	ManagerID     sql.NullInt64
	ManagerFirst  sql.NullString
	ManagerLast   sql.NullString
	ManagerUser   sql.NullString
	UserHeaderMsg sql.NullInt64 // status bar message_id in user's private chat
	PinnedMsgID   sql.NullInt64 // pinned card message_id in topic
	UpdatedAt     sql.NullString
}

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

-- LIVE SUPPORT SESSION
CREATE TABLE IF NOT EXISTS support_sessions (
  user_id          INTEGER PRIMARY KEY,
  thread_id        INTEGER NOT NULL DEFAULT 0,
  status           TEXT NOT NULL DEFAULT 'waiting',
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

// =====================
// USERS
// =====================

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

// =====================
// SESSIONS
// =====================

func (s *SQLiteStore) UpsertSessionWaiting(ctx context.Context, userID int64, threadID int, userHeaderMsgID int64) error {
	q := `
INSERT INTO support_sessions (user_id, thread_id, status, user_header_msg, updated_at)
VALUES (?, ?, ?, ?, datetime('now'))
ON CONFLICT(user_id) DO UPDATE SET
  thread_id = excluded.thread_id,

  -- ❗️НЕ даём сбить ACTIVE обратно в WAITING
  status = CASE
    WHEN support_sessions.status = 'active' THEN support_sessions.status
    ELSE excluded.status
  END,

  user_header_msg = COALESCE(excluded.user_header_msg, support_sessions.user_header_msg),
  updated_at      = datetime('now');
`
	var header any = nil
	if userHeaderMsgID != 0 {
		header = userHeaderMsgID
	}
	_, err := s.db.ExecContext(ctx, q, userID, threadID, string(SessionWaiting), header)
	return err
}

func (s *SQLiteStore) GetSessionByUserID(ctx context.Context, userID int64) (SupportSession, bool, error) {
	var ss SupportSession
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id, thread_id, status, manager_id, manager_first, manager_last, manager_username,
		       user_header_msg, pinned_msg_id, updated_at
		FROM support_sessions WHERE user_id = ? LIMIT 1
	`, userID).Scan(
		&ss.UserID, &ss.ThreadID, &ss.Status,
		&ss.ManagerID, &ss.ManagerFirst, &ss.ManagerLast, &ss.ManagerUser,
		&ss.UserHeaderMsg, &ss.PinnedMsgID, &ss.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return SupportSession{}, false, nil
	}
	if err != nil {
		return SupportSession{}, false, err
	}
	return ss, true, nil
}
func (s *SQLiteStore) ResetSessionToWaiting(ctx context.Context, userID int64, threadID int) error {
	// если строки нет — создадим
	_, err := s.db.ExecContext(ctx, `
INSERT INTO support_sessions (user_id, thread_id, status, updated_at)
VALUES (?, ?, ?, datetime('now'))
ON CONFLICT(user_id) DO UPDATE SET
  thread_id        = excluded.thread_id,
  status           = excluded.status,
  manager_id       = NULL,
  manager_first    = NULL,
  manager_last     = NULL,
  manager_username = NULL,
  updated_at       = datetime('now')
`, userID, threadID, string(SessionWaiting))
	return err
}

func (s *SQLiteStore) GetSessionByThreadID(ctx context.Context, threadID int) (SupportSession, bool, error) {
	var ss SupportSession
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id, thread_id, status, manager_id, manager_first, manager_last, manager_username,
		       user_header_msg, pinned_msg_id, updated_at
		FROM support_sessions WHERE thread_id = ? LIMIT 1
	`, threadID).Scan(
		&ss.UserID, &ss.ThreadID, &ss.Status,
		&ss.ManagerID, &ss.ManagerFirst, &ss.ManagerLast, &ss.ManagerUser,
		&ss.UserHeaderMsg, &ss.PinnedMsgID, &ss.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return SupportSession{}, false, nil
	}
	if err != nil {
		return SupportSession{}, false, err
	}
	return ss, true, nil
}

func (s *SQLiteStore) CloseSession(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE support_sessions SET
  status           = ?,
  manager_id       = NULL,
  manager_first    = NULL,
  manager_last     = NULL,
  manager_username = NULL,
  updated_at       = datetime('now')
WHERE user_id = ?
`, string(SessionClosed), userID)
	return err
}

// =====================
// session helpers for UI ids
// =====================

// ===== status bar (user_header_msg) =====

func (s *SQLiteStore) GetStatusMsgID(ctx context.Context, userID int64) (int, bool, error) {
	var v sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT user_header_msg
		FROM support_sessions
		WHERE user_id = ?
		LIMIT 1
	`, userID).Scan(&v)

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

func (s *SQLiteStore) SetStatusMsgID(ctx context.Context, userID int64, msgID int) error {
	// сессия должна существовать (иначе апдейт не заденет строку)
	_, _ = s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO support_sessions (user_id, thread_id, status, updated_at)
		VALUES (?, 0, ?, datetime('now'))
	`, userID, string(SessionWaiting))

	_, err := s.db.ExecContext(ctx, `
		UPDATE support_sessions
		SET user_header_msg = ?, updated_at = datetime('now')
		WHERE user_id = ?
	`, msgID, userID)

	return err
}
func (s *SQLiteStore) GetPinnedMsgID(ctx context.Context, userID int64) (int, bool, error) {
	ss, ok, err := s.GetSessionByUserID(ctx, userID)
	if err != nil || !ok {
		return 0, false, err
	}
	if !ss.PinnedMsgID.Valid || ss.PinnedMsgID.Int64 == 0 {
		return 0, false, nil
	}
	return int(ss.PinnedMsgID.Int64), true, nil
}

func (s *SQLiteStore) SetPinnedMsgID(ctx context.Context, userID int64, msgID int) error {
	// гарантируем строку сессии
	_, _ = s.db.ExecContext(ctx, `
INSERT OR IGNORE INTO support_sessions (user_id, thread_id, status, updated_at)
VALUES (?, 0, ?, datetime('now'))
`, userID, string(SessionWaiting))

	_, err := s.db.ExecContext(ctx, `
UPDATE support_sessions
SET pinned_msg_id = ?, updated_at = datetime('now')
WHERE user_id = ?
`, msgID, userID)
	return err
}

// =====================
// helpers
// =====================

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
func (s *SQLiteStore) ActivateSession(ctx context.Context, userID int64, manager User, pinnedMsgID int64) error {
	// гарантируем строку сессии (если вдруг её нет)
	_, _ = s.db.ExecContext(ctx, `
INSERT OR IGNORE INTO support_sessions (user_id, thread_id, status, updated_at)
VALUES (?, 0, ?, datetime('now'))
`, userID, string(SessionWaiting))

	q := `
UPDATE support_sessions SET
  status = ?,
  manager_id = ?,
  manager_first = ?,
  manager_last = ?,
  manager_username = ?,
  pinned_msg_id = ?,
  updated_at = datetime('now')
WHERE user_id = ?;
`
	_, err := s.db.ExecContext(ctx, q,
		string(SessionActive),
		manager.UserID,
		nullStringOrNil(manager.FirstName),
		nullStringOrNil(manager.LastName),
		nullStringOrNil(manager.Username),
		nullInt64OrNil(sql.NullInt64{Int64: pinnedMsgID, Valid: pinnedMsgID != 0}),
		userID,
	)
	return err
}
