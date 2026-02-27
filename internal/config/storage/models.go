package storage

import "database/sql"

// models.go
//
// Только структуры таблиц users/support_sessions.
// Никаких запросов, никакой логики.

type User struct {
	TelegramUserID int64
	ChatID         int64
	Username       sql.NullString
	FirstName      sql.NullString
	LastName       sql.NullString
	Lang           sql.NullString
	ThreadID       sql.NullInt64
	StatusMsgID    sql.NullInt64
	PinnedMsgID    sql.NullInt64
}
