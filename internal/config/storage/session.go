package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// sessions.go
//
// Всё по таблице support_sessions:
// - waiting session
// - get/set user_header_msg (status bar)
// - get/set pinned_msg_id
// - get session by user/thread
//
// Важно: методы здесь “тупые”, просто SQL.
// Логику “когда вызывать” решает service.

func (s *SQLiteStore) EnsureSessionRow(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `
INSERT OR IGNORE INTO support_sessions (user_id, thread_id, updated_at)
VALUES (?, 0, datetime('now'))
`, userID)
	return err
}

func (s *SQLiteStore) UpsertSessionWaiting(ctx context.Context, userID int64, threadID int, userHeaderMsgID int64) error {
	// гарантируем, что строка есть
	_ = s.EnsureSessionRow(ctx, userID)

	q := `
INSERT INTO support_sessions (user_id, thread_id, user_header_msg, updated_at)
VALUES (?, ?, ?, datetime('now'))
ON CONFLICT(user_id) DO UPDATE SET
  thread_id       = excluded.thread_id,
  user_header_msg = COALESCE(excluded.user_header_msg, support_sessions.user_header_msg),
  updated_at      = datetime('now');
`
	var header any = nil
	if userHeaderMsgID != 0 {
		header = userHeaderMsgID
	}

	_, err := s.db.ExecContext(ctx, q, userID, threadID, header)
	return err
}

func (s *SQLiteStore) GetSessionByUserID(ctx context.Context, userID int64) (SupportSession, bool, error) {
	var ss SupportSession
	err := s.db.QueryRowContext(ctx, `
SELECT user_id, thread_id, manager_id, manager_first, manager_last, manager_username,
       user_header_msg, pinned_msg_id, updated_at
FROM support_sessions
WHERE user_id = ?
LIMIT 1
`, userID).Scan(
		&ss.UserID, &ss.ThreadID,
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

func (s *SQLiteStore) GetSessionByThreadID(ctx context.Context, threadID int) (SupportSession, bool, error) {
	var ss SupportSession
	err := s.db.QueryRowContext(ctx, `
SELECT user_id, thread_id, manager_id, manager_first, manager_last, manager_username,
       user_header_msg, pinned_msg_id, updated_at
FROM support_sessions
WHERE thread_id = ?
LIMIT 1
`, threadID).Scan(
		&ss.UserID, &ss.ThreadID,
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
	_ = s.EnsureSessionRow(ctx, userID)

	_, err := s.db.ExecContext(ctx, `
UPDATE support_sessions
SET user_header_msg = ?, updated_at = datetime('now')
WHERE user_id = ?
`, msgID, userID)

	return err
}

// ===== pinned card (pinned_msg_id) =====

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
	_ = s.EnsureSessionRow(ctx, userID)

	_, err := s.db.ExecContext(ctx, `
UPDATE support_sessions
SET pinned_msg_id = ?, updated_at = datetime('now')
WHERE user_id = ?
`, msgID, userID)
	return err
}

// ===== optional: если где-то в сервисе ещё используется ActivateSession =====

func (s *SQLiteStore) ActivateSession(ctx context.Context, userID int64, manager User, pinnedMsgID int64) error {
	_ = s.EnsureSessionRow(ctx, userID)

	q := `
UPDATE support_sessions SET
  manager_id       = ?,
  manager_first    = ?,
  manager_last     = ?,
  manager_username = ?,
  pinned_msg_id    = ?,
  updated_at       = datetime('now')
WHERE user_id = ?;
`
	_, err := s.db.ExecContext(ctx, q,
		manager.UserID,
		nullStringOrNil(manager.FirstName),
		nullStringOrNil(manager.LastName),
		nullStringOrNil(manager.Username),
		nullInt64OrNil(sql.NullInt64{Int64: pinnedMsgID, Valid: pinnedMsgID != 0}),
		userID,
	)
	if err != nil {
		return fmt.Errorf("ActivateSession: %w", err)
	}
	return nil
}
