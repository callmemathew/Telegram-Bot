package service

import (
	"context"

	"tg-bot/internal/config/storage"

	"github.com/mymmrac/telego"
)

// user.go
//
// Работа с пользователем в БД:
// - EnsureUser: гарантирует запись пользователя (chat_id, имя, username)
// - SetLang/GetLang: язык пользователя в БД

func (s *SupportService) EnsureUser(ctx context.Context, tgUser *telego.User, chatID int64) error {
	if tgUser == nil {
		return nil
	}
	u := storage.User{
		TelegramUserID: tgUser.ID,
		ChatID:         chatID,
		Username:       toNullString(tgUser.Username),
		FirstName:      toNullString(tgUser.FirstName),
		LastName:       toNullString(tgUser.LastName),
	}
	return s.store.EnsureUser(ctx, u)
}

func (s *SupportService) SetLang(ctx context.Context, userID int64, lang string) error {
	lang = normLang(lang)
	return s.store.SetLang(ctx, userID, lang)
}

func (s *SupportService) GetLang(ctx context.Context, userID int64) (string, bool, error) {
	return s.store.GetLangByUserID(ctx, userID)
}
