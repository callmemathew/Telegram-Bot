package service

import (
	"context"
	"fmt"
	"strings"

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

	// lang must be selected
	lang, ok, err := s.store.GetLangByUserID(ctx, user.ID)
	if err != nil {
		return err
	}
	if !ok || strings.TrimSpace(lang) == "" {
		return fmt.Errorf("lang not selected")
	}
	lang = normLang(lang)

	// ensure user row
	_ = s.EnsureUser(ctx, user, m.Chat.ID)

	// НЕ пересылаем команды менеджерам
	if strings.HasPrefix(strings.TrimSpace(m.Text), "/") {
		return nil
	}

	// ensure thread (topic)
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
		_ = s.store.SetThreadID(ctx, user.ID, threadID)
	}
	// достаем storage.User с заполненным ThreadID и создаем/обновляем pinned card
	u, err := s.store.GetUserByUserID(ctx, user.ID)
	if err == nil {
		if err := s.UpsertPinnedCard(ctx, u, lang, "—"); err != nil {
			return err
		}
	}

	// send to managers
	if !hasAttachment(m) {
		text := strings.TrimSpace(m.Text)
		if text == "" {
			return nil
		}

		out := fmt.Sprintf("👤 %s | %s %s\n💬 %s", topicTitle(user), langEmoji(lang), lang, text)
		msg := telegoutil.Message(telegoutil.ID(s.managersChatID), out)
		msg.MessageThreadID = threadID
		_, err := s.bot.SendMessage(ctx, msg)
		return err
	}

	// attachment: header + copy
	label := attachmentText(lang, m)
	if cap := strings.TrimSpace(m.Caption); cap != "" {
		label = cap
	}

	head := fmt.Sprintf("👤 %s | %s %s\n💬 %s", topicTitle(user), langEmoji(lang), lang, label)
	hmsg := telegoutil.Message(telegoutil.ID(s.managersChatID), head)
	hmsg.MessageThreadID = threadID
	_, _ = s.bot.SendMessage(ctx, hmsg)

	_, err = s.bot.CopyMessage(ctx, &telego.CopyMessageParams{
		ChatID:          telegoutil.ID(s.managersChatID),
		FromChatID:      telegoutil.ID(m.Chat.ID),
		MessageID:       m.MessageID,
		MessageThreadID: threadID,
	})
	return err
}
