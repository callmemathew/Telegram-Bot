package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
)

// ui_lang.go
//
// Обновление UI после смены языка:
// - переименовать forum topic (если есть thread_id)
// - обновить pinned card (если pinned_msg_id уже есть)

func (s *SupportService) RefreshLangUI(ctx context.Context, tgUserID int64) error {
	u, err := s.store.GetUserByUserID(ctx, tgUserID)
	if err != nil {
		return err
	}

	lang := "RU"
	if u.Lang.Valid && strings.TrimSpace(u.Lang.String) != "" {
		lang = normLang(u.Lang.String)
	}

	if !u.ThreadID.Valid || u.ThreadID.Int64 == 0 {
		// треда ещё нет — нечего переименовывать
		return nil
	}

	threadID := int(u.ThreadID.Int64)
	title := fmt.Sprintf("%s | %s %s", displayUser(u), langEmoji(lang), lang)
	err = s.bot.EditForumTopic(ctx, &telego.EditForumTopicParams{
		ChatID:          telegoutil.ID(s.managersChatID),
		MessageThreadID: threadID,
		Name:            title,
	})
	if err != nil && !isTopicNotModified(err) {
		return fmt.Errorf("EditForumTopic failed: %w", err)
	}
	return nil
}
func isTopicNotModified(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "topic_not_modified")
}
