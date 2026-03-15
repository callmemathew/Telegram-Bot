package service

import (
	"context"
	"fmt"
	"strings"
	"tg-bot/internal/config/storage"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
)

// flow_user.go
//
// Поток: USER -> MANAGERS
// - проверка языка
// - создание форума topic (если нет)
// - отправка текста или CopyMessage вложений
// (без session / status / pinned)

func (s *SupportService) OnUserMessage(ctx context.Context, m *telego.Message) error {
	if m == nil || m.From == nil || m.Chat.ID == 0 {
		return nil
	}
	user := m.From

	lang, ok, err := s.store.GetLangByUserID(ctx, user.ID)
	if err != nil {
		return err
	}
	if !ok || strings.TrimSpace(lang) == "" {
		return fmt.Errorf("lang not selected")
	}
	lang = normLang(lang)

	if err := s.EnsureUser(ctx, user, m.Chat.ID); err != nil {
		return err
	}

	if strings.HasPrefix(strings.TrimSpace(m.Text), "/") {
		return nil
	}

	threadID, hasThread, err := s.store.GetThreadByUserID(ctx, user.ID)
	if err != nil {
		return err
	}
	if !hasThread || threadID == 0 {
		title := fmt.Sprintf("%s | %s %s", topicTitle(user), langEmoji(lang), lang)
		created, err := s.bot.CreateForumTopic(ctx, &telego.CreateForumTopicParams{
			ChatID: telegoutil.ID(s.managersChatID),
			Name:   title,
		})
		if err != nil {
			return err
		}
		threadID = created.MessageThreadID

		if err := s.store.SetThreadID(ctx, user.ID, threadID); err != nil {
			return err
		}
	}

	u, err := s.store.GetUserByUserID(ctx, user.ID)
	if err == nil {
		if err := s.UpsertPinnedCard(ctx, u, lang, "—"); err != nil {
			return err
		}
	}

	txt := strings.TrimSpace(m.Text)
	mediaType, fileID := extractMediaInfo(m)
	hasMedia := mediaType != "" && fileID != ""

	// ===== TEXT ONLY =====
	if !hasMedia {
		if txt == "" {
			return nil
		}

		out := fmt.Sprintf("👤 %s | %s %s\n💬 %s", topicTitle(user), langEmoji(lang), lang, txt)
		msg := telegoutil.Message(telegoutil.ID(s.managersChatID), out)
		msg.MessageThreadID = threadID

		_, err := s.bot.SendMessage(ctx, msg)
		if err != nil {
			return err
		}

		_ = s.store.SaveMessage(ctx, storage.Message{
			TelegramUserID: user.ID,
			Direction:      "user",
			Text:           toNullString(txt),
			HasMedia:       0,
		})

		return nil
	}

	// ===== MEDIA =====
	label := attachmentText(lang, m)

	head := fmt.Sprintf("👤 %s | %s %s\n💬 %s", topicTitle(user), langEmoji(lang), lang, label)
	hmsg := telegoutil.Message(telegoutil.ID(s.managersChatID), head)
	hmsg.MessageThreadID = threadID

	_, err = s.bot.SendMessage(ctx, hmsg)
	if err != nil {
		return err
	}

	_, err = s.bot.CopyMessage(ctx, &telego.CopyMessageParams{
		ChatID:          telegoutil.ID(s.managersChatID),
		FromChatID:      telegoutil.ID(m.Chat.ID),
		MessageID:       m.MessageID,
		MessageThreadID: threadID,
	})
	if err != nil {
		return err
	}

	mediaType, fileID = extractMediaInfo(m)

	_ = s.store.SaveMessage(ctx, storage.Message{
		TelegramUserID: user.ID,
		Direction:      "user",
		Text:           toNullString(strings.TrimSpace(m.Caption)),
		HasMedia:       1,
		MediaType:      toNullString(mediaType),
		FileID:         toNullString(fileID),
	})

	return nil
}
