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

	pin := func(messageID int) error {
		if err := s.bot.PinChatMessage(ctx, &telego.PinChatMessageParams{
			ChatID:              telegoutil.ID(s.managersChatID),
			MessageID:           messageID,
			DisableNotification: true,
		}); err != nil {
			return fmt.Errorf("PinChatMessage failed: %w", err)
		}
		return nil
	}

	sendNew := func() error {
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

		return pin(sent.MessageID)
	}

	// 1) если есть pinned_msg_id -> пробуем редактировать
	if hasPin && pid != 0 {
		_, err := s.bot.EditMessageText(ctx, &telego.EditMessageTextParams{
			ChatID:    telegoutil.ID(s.managersChatID),
			MessageID: pid,
			Text:      card,
			ParseMode: "Markdown",
		})

		// ✅ если карточка пропала/не найдена -> создаём новую и переписываем pinned_msg_id
		if err != nil {
			low := strings.ToLower(err.Error())

			if strings.Contains(low, "message to edit not found") ||
				strings.Contains(low, "message_id_invalid") ||
				strings.Contains(low, "message can't be edited") {
				return sendNew()
			}

			if strings.Contains(low, "message is not modified") {
				return pin(pid)
			}

			return err
		}

		return pin(pid)
	}

	// 2) pinned_msg_id нет -> создаём новую
	return sendNew()
}
