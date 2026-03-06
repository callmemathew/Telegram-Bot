package service

import (
	"context"
	"fmt"
	"strings"
	"tg-bot/internal/config/storage"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
)

// flow_manager.go
//
// Поток: MANAGER -> USER
// - находим пользователя по thread_id
// - пересылаем текст или вложения пользователю
// (без /ready, без ActivateSession, без "статусов")

func (s *SupportService) OnManagerReply(ctx context.Context, m *telego.Message) error {
	if m == nil || m.From == nil || m.MessageThreadID == 0 {
		return nil
	}

	// find user by thread
	u, err := s.store.GetUserByThreadID(ctx, m.MessageThreadID)
	if err != nil {
		return err
	}

	managerName := buildManagerName(m.From)

	txt := strings.TrimSpace(m.Text)
	mediaType, fileID := extractMediaInfo(m)
	hasMedia := mediaType != "" && fileID != ""

	// ===== TEXT ONLY =====
	if !hasMedia {
		if txt == "" || strings.HasPrefix(txt, "/") {
			return nil
		}

		out := fmt.Sprintf("%s:\n%s", managerName, txt)
		_, err := s.bot.SendMessage(ctx, telegoutil.Message(telegoutil.ID(u.ChatID), out))
		if err != nil {
			return err
		}

		_ = s.store.SaveMessage(ctx, storage.Message{
			TelegramUserID: u.TelegramUserID,
			Direction:      "manager",
			Text:           toNullString(txt),
			HasMedia:       0,
		})

		return nil
	}

	// ===== MEDIA =====
	cap := strings.TrimSpace(m.Caption)

	if cap != "" {
		out := fmt.Sprintf("%s:\n%s", managerName, cap)
		_, _ = s.bot.SendMessage(ctx, telegoutil.Message(telegoutil.ID(u.ChatID), out))
	} else {
		out := fmt.Sprintf("%s:", managerName)
		_, _ = s.bot.SendMessage(ctx, telegoutil.Message(telegoutil.ID(u.ChatID), out))
	}

	_, err = s.bot.CopyMessage(ctx, &telego.CopyMessageParams{
		ChatID:     telegoutil.ID(u.ChatID),
		FromChatID: telegoutil.ID(s.managersChatID),
		MessageID:  m.MessageID,
	})
	if err != nil {
		return err
	}

	_ = s.store.SaveMessage(ctx, storage.Message{
		TelegramUserID: u.TelegramUserID,
		Direction:      "manager",
		Text:           toNullString(cap),
		HasMedia:       1,
		MediaType:      toNullString(mediaType),
		FileID:         toNullString(fileID),
	})

	return nil
}

func buildManagerName(u *telego.User) string {
	if u == nil {
		return "Support manager"
	}
	first := strings.TrimSpace(u.FirstName)
	last := strings.TrimSpace(u.LastName)
	if first != "" && last != "" {
		return first + " " + last
	}
	if first != "" {
		return first
	}
	if strings.TrimSpace(u.Username) != "" {
		return "@" + strings.TrimSpace(u.Username)
	}
	return "Support manager"
}
