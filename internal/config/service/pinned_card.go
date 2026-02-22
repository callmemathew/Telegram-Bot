package service

import (
	"context"
	"fmt"
	"strings"

	"tg-bot/internal/config/storage"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
)

// pinned_card.go
//
// PIN CARD в топике менеджеров:
// - если pinned_msg_id есть -> EditMessageText + Pin
// - если нет -> SendMessage + сохранить pinned_msg_id + Pin

func (s *SupportService) UpsertPinnedCard(
	ctx context.Context,
	user storage.User,
	lang string,
	managerName string,
) error {
	lang = normLang(lang)

	if !user.ThreadID.Valid || user.ThreadID.Int64 == 0 {
		return nil
	}
	threadID := int(user.ThreadID.Int64)

	card := fmt.Sprintf(
		"📌 *LIVE SUPPORT SESSION*\n"+
			"👤 *User:* %s\n"+
			"🆔 *user_id:* `%d`\n"+
			"🌍 *Lang:* %s %s\n"+
			"💼 *Manager:* %s\n",
		displayUser(user),
		user.UserID,
		langEmoji(lang), lang,
		prettyManager(managerName),
	)

	pid, hasPin, err := s.store.GetPinnedMsgID(ctx, user.UserID)
	if err != nil {
		return err
	}

	// 1) если уже есть pinned_msg_id -> редактируем
	if hasPin && pid != 0 {
		_, err := s.bot.EditMessageText(ctx, &telego.EditMessageTextParams{
			ChatID:    telegoutil.ID(s.managersChatID),
			MessageID: pid,
			Text:      card,
			ParseMode: "Markdown",
		})
		if err != nil && !strings.Contains(err.Error(), "message is not modified") {
			return err
		}

		_ = s.bot.PinChatMessage(ctx, &telego.PinChatMessageParams{
			ChatID:              telegoutil.ID(s.managersChatID),
			MessageID:           pid,
			DisableNotification: true,
		})
		return nil
	}

	// 2) иначе -> создаём pinned card
	msg := telegoutil.Message(telegoutil.ID(s.managersChatID), card)
	msg.ParseMode = "Markdown"
	msg.MessageThreadID = threadID

	sent, err := s.bot.SendMessage(ctx, msg)
	if err != nil {
		return err
	}

	if err := s.store.SetPinnedMsgID(ctx, user.UserID, sent.MessageID); err != nil {
		return err
	}

	_ = s.bot.PinChatMessage(ctx, &telego.PinChatMessageParams{
		ChatID:              telegoutil.ID(s.managersChatID),
		MessageID:           sent.MessageID,
		DisableNotification: true,
	})

	return nil
}
