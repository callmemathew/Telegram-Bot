package service

import (
	"context"

	"tg-bot/internal/config/storage"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
)

// lang_menu.go
//
// Показ меню языка "вместо бара" (редактируем status_msg_id).
// Удобно, если хочешь один "закреплённый" message-id вместо спама сообщений.

func (s *SupportService) ShowLangMenu(ctx context.Context, userID, chatID int64) error {
	_ = s.store.EnsureUser(ctx, storage.User{UserID: userID, ChatID: chatID})

	kb := telegoutil.InlineKeyboard(
		telegoutil.InlineKeyboardRow(
			telegoutil.InlineKeyboardButton("Русский").WithCallbackData("lang:RU"),
			telegoutil.InlineKeyboardButton("Українська").WithCallbackData("lang:UA"),
			telegoutil.InlineKeyboardButton("English").WithCallbackData("lang:EN"),
		),
	)

	text := "🌍 Выберите язык / Оберіть мову / Choose language:"

	mid, ok, err := s.store.GetStatusMsgID(ctx, userID)
	if err != nil {
		return err
	}

	if ok && mid != 0 {
		_, err := s.bot.EditMessageText(ctx, &telego.EditMessageTextParams{
			ChatID:      telegoutil.ID(chatID),
			MessageID:   mid,
			Text:        text,
			ReplyMarkup: kb,
		})
		if isMessageNotModified(err) {
			return nil
		}
		if err != nil && isUneditableMessage(err) {
			msg := telegoutil.Message(telegoutil.ID(chatID), text)
			msg.ReplyMarkup = kb
			sent, sendErr := s.bot.SendMessage(ctx, msg)
			if sendErr != nil {
				return sendErr
			}
			return s.store.SetStatusMsgID(ctx, userID, sent.MessageID)
		}
		return err
	}

	msg := telegoutil.Message(telegoutil.ID(chatID), text)
	msg.ReplyMarkup = kb

	sent, err := s.bot.SendMessage(ctx, msg)
	if err != nil {
		return err
	}
	return s.store.SetStatusMsgID(ctx, userID, sent.MessageID)
}
