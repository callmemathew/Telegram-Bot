package storage

import "database/sql"

// models.go
//
// Только структуры таблиц users/support_sessions.
// Никаких запросов, никакой логики.

type User struct {
	UserID    int64
	ChatID    int64
	Username  sql.NullString
	FirstName sql.NullString
	LastName  sql.NullString
	Lang      sql.NullString
	ThreadID  sql.NullInt64
}

type SupportSession struct {
	UserID        int64
	ThreadID      int64
	ManagerID     sql.NullInt64
	ManagerFirst  sql.NullString
	ManagerLast   sql.NullString
	ManagerUser   sql.NullString
	UserHeaderMsg sql.NullInt64 // status bar message_id in user's private chat
	PinnedMsgID   sql.NullInt64 // pinned card message_id in topic
	UpdatedAt     sql.NullString
}
