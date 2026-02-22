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

func (s *SupportService) RefreshLangUI(ctx context.Context, userID int64) error {
	u, err := s.store.GetUserByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("GetUserByUserID: %w", err)
	}

	lang := "RU"
	if l, ok, err := s.store.GetLangByUserID(ctx, userID); err == nil && ok && strings.TrimSpace(l) != "" {
		lang = normLang(l)
	}

	// 1) rename topic (если есть thread_id)
	if u.ThreadID.Valid && u.ThreadID.Int64 != 0 {
		threadID := int(u.ThreadID.Int64)
		title := fmt.Sprintf("%s | %s %s", displayUser(u), langEmoji(lang), lang)

		_ = s.bot.EditForumTopic(ctx, &telego.EditForumTopicParams{
			ChatID:          telegoutil.ID(s.managersChatID),
			MessageThreadID: threadID,
			Name:            title,
		})
	}

	// 2) update pinned card only if it exists
	ss, okS, err := s.store.GetSessionByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("GetSessionByUserID: %w", err)
	}
	if !okS {
		return nil
	}

	pid, hasPin, err := s.store.GetPinnedMsgID(ctx, userID)
	if err != nil {
		return err
	}
	if !hasPin || pid == 0 {
		return nil
	}

	managerName := sessionManagerName(ss)
	_ = s.UpsertPinnedCard(ctx, u, lang, managerName)
	return nil
}
