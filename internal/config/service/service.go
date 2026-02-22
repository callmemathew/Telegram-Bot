package service

import (
	"tg-bot/internal/config/storage"

	"github.com/mymmrac/telego"
)

// service.go
//
// База SupportService: зависимости и конструктор.
// Вся логика разнесена по файлам: user/lang/ui/pinned/flows/hint/helpers.

type SupportService struct {
	bot            *telego.Bot
	managersChatID int64
	store          *storage.SQLiteStore
}

func NewSupportService(bot *telego.Bot, managersChatID int64, store *storage.SQLiteStore) *SupportService {
	return &SupportService{
		bot:            bot,
		managersChatID: managersChatID,
		store:          store,
	}
}
