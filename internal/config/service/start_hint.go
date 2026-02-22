package service

import (
	"context"
	"strings"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
)

// start_hint.go
//
// StartHint (подсказка в личке).
// ТВОЁ ТРЕБОВАНИЕ: отправить ОДИН РАЗ и НЕ менять при смене языка.
// Для этого используется EnsureStartHintOnce.
// ShowStartHint оставлен (если вдруг нужен режим "бар редактируется").
//
// Если хочешь "один раз и не менять" — в handler вызывай EnsureStartHintOnce,
// а НЕ ShowStartHint.

func (s *SupportService) EnsureStartHintOnce(ctx context.Context, userID, chatID int64) error {
	mid, ok, err := s.store.GetStatusMsgID(ctx, userID)
	if err != nil {
		return err
	}
	if ok && mid != 0 {
		return nil
	}

	lang := "RU"
	if l, ok, err := s.store.GetLangByUserID(ctx, userID); err == nil && ok && strings.TrimSpace(l) != "" {
		lang = normLang(l)
	}

	text := StartHintText(lang)

	sent, err := s.bot.SendMessage(ctx, telegoutil.Message(telegoutil.ID(chatID), text))
	if err != nil {
		return err
	}

	return s.store.SetStatusMsgID(ctx, userID, sent.MessageID)
}

// ShowStartHint гарантирует: у пользователя всегда есть "бар" (может редактироваться).
// 1) Пытаемся отредактировать status_msg_id
// 2) Если нельзя — отправляем новый и сохраняем новый id
func (s *SupportService) ShowStartHint(ctx context.Context, userID, chatID int64) error {
	lang := "RU"
	if l, ok, err := s.store.GetLangByUserID(ctx, userID); err == nil && ok && strings.TrimSpace(l) != "" {
		lang = normLang(l)
	}

	text := StartHintText(lang)

	mid, ok, err := s.store.GetStatusMsgID(ctx, userID)
	if err != nil {
		return err
	}

	if ok && mid != 0 {
		emptyKB := &telego.InlineKeyboardMarkup{InlineKeyboard: make([][]telego.InlineKeyboardButton, 0)}

		_, editErr := s.bot.EditMessageText(ctx, &telego.EditMessageTextParams{
			ChatID:      telegoutil.ID(chatID),
			MessageID:   mid,
			Text:        text,
			ReplyMarkup: emptyKB,
		})

		if editErr == nil || isMessageNotModified(editErr) {
			return nil
		}
		if !isUneditableMessage(editErr) {
			return editErr
		}
	}

	sent, sendErr := s.bot.SendMessage(ctx, telegoutil.Message(telegoutil.ID(chatID), text))
	if sendErr != nil {
		return sendErr
	}

	return s.store.SetStatusMsgID(ctx, userID, sent.MessageID)
}

func StartHintText(lang string) string {
	switch strings.ToUpper(lang) {
	case "UA":
		return "Напишіть ваше повідомлення — команда DocData відповість вам найближчим часом.\n\n💬 Це живий чат підтримки."
	case "EN":
		return "Send your message — the DocData team will respond shortly.\n\n💬 This is a live support chat."
	default:
		return "Напишите ваше сообщение — команда DocData ответит вам в ближайшее время.\n\n💬 Это живой чат поддержки."
	}
}
