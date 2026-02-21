package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"tg-bot/internal/config/storage"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
)

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

// =========================
// User ensure
// =========================

func (s *SupportService) EnsureUser(ctx context.Context, tgUser *telego.User, chatID int64) error {
	if tgUser == nil {
		return nil
	}
	u := storage.User{
		UserID:    tgUser.ID,
		ChatID:    chatID,
		Username:  toNullString(tgUser.Username),
		FirstName: toNullString(tgUser.FirstName),
		LastName:  toNullString(tgUser.LastName),
	}
	return s.store.EnsureUser(ctx, u)
}

// =========================
// Lang (DB)
// =========================

func (s *SupportService) SetLang(ctx context.Context, userID int64, lang string) error {
	lang = normLang(lang)
	return s.store.SetLang(ctx, userID, lang)
}

func (s *SupportService) GetLang(ctx context.Context, userID int64) (string, bool, error) {
	return s.store.GetLangByUserID(ctx, userID)
}

// =========================
// RefreshLangUI (после SetLang):
// - topic title (если есть thread)
// - pinned card (если есть pinned_msg_id)
// =========================

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
	_ = s.UpsertPinnedCard(ctx, u, ss, lang, managerName)
	return nil
}

// =========================
// PIN CARD (MANAGER TOPIC) — 1 message, edit it + pin once
// =========================

func (s *SupportService) UpsertPinnedCard(
	ctx context.Context,
	user storage.User,
	ss storage.SupportSession,
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

// =========================
// USER -> MANAGERS
// =========================

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

	// ensure thread
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

	// ensure session exists (для pinned/manager info)
	if _, okS, _ := s.store.GetSessionByUserID(ctx, user.ID); !okS {
		_ = s.store.UpsertSessionWaiting(ctx, user.ID, threadID, 0)
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

// =========================
// MANAGER -> USER
// =========================

func (s *SupportService) OnManagerReply(ctx context.Context, m *telego.Message) error {
	if m == nil || m.From == nil || m.MessageThreadID == 0 {
		return nil
	}

	// find user by thread
	u, err := s.store.GetUserByThreadID(ctx, m.MessageThreadID)
	if err != nil {
		return err
	}

	// lang (можно не использовать, но оставим для будущего)
	lang := "RU"
	if l, ok, _ := s.store.GetLangByUserID(ctx, u.UserID); ok && strings.TrimSpace(l) != "" {
		lang = normLang(l)
	}
	_ = lang

	managerName := buildManagerName(m.From)

	// ensure session exists
	ss, okS, _ := s.store.GetSessionByUserID(ctx, u.UserID)
	if !okS || ss.ThreadID == 0 {
		_ = s.store.UpsertSessionWaiting(ctx, u.UserID, m.MessageThreadID, 0)
		ss, _, _ = s.store.GetSessionByUserID(ctx, u.UserID)
	}

	txt := strings.TrimSpace(m.Text)
	isReadyCmd := strings.HasPrefix(strings.ToLower(txt), "/ready")
	hasAtt := hasAttachment(m)

	// activate on /ready OR first normal reply OR attachment
	shouldActivate := false
	if strings.TrimSpace(sessionManagerName(ss)) == "" {
		if isReadyCmd || hasAtt || (txt != "" && !strings.HasPrefix(txt, "/")) {
			shouldActivate = true
		}
	}

	if shouldActivate {
		_ = s.UpsertPinnedCard(ctx, u, ss, lang, managerName)

		pid, okP, _ := s.store.GetPinnedMsgID(ctx, u.UserID)
		var pinnedMsgID int64
		if okP && pid != 0 {
			pinnedMsgID = int64(pid)
		}

		manager := storage.User{
			UserID:    m.From.ID,
			Username:  toNullString(m.From.Username),
			FirstName: toNullString(m.From.FirstName),
			LastName:  toNullString(m.From.LastName),
		}
		_ = s.store.ActivateSession(ctx, u.UserID, manager, pinnedMsgID)

		ss, _, _ = s.store.GetSessionByUserID(ctx, u.UserID)
		_ = s.UpsertPinnedCard(ctx, u, ss, lang, managerName)

		if isReadyCmd {
			return nil
		}
	}

	// USER CHAT OUTPUT
	if !hasAtt {
		if txt == "" || strings.HasPrefix(txt, "/") {
			return nil
		}
		out := fmt.Sprintf("%s:\n%s", managerName, txt)
		_, err := s.bot.SendMessage(ctx, telegoutil.Message(telegoutil.ID(u.ChatID), out))
		return err
	}

	// attachment flow
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
	return err

}

// =========================
// helpers
// =========================
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
func isMessageNotModified(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "message is not modified")
}

func hasAttachment(m *telego.Message) bool {
	if m == nil {
		return false
	}
	return m.Photo != nil ||
		m.Document != nil ||
		m.Video != nil ||
		m.Voice != nil ||
		m.VideoNote != nil ||
		m.Audio != nil ||
		m.Animation != nil
}

func topicTitle(u *telego.User) string {
	if u == nil {
		return "Unknown"
	}
	if strings.TrimSpace(u.Username) != "" {
		return "@" + strings.TrimSpace(u.Username)
	}
	full := strings.TrimSpace(strings.TrimSpace(u.FirstName) + " " + strings.TrimSpace(u.LastName))
	if full != "" {
		return full
	}
	return fmt.Sprintf("id%d", u.ID)
}

func displayUser(u storage.User) string {
	if u.Username.Valid && strings.TrimSpace(u.Username.String) != "" {
		return "@" + strings.TrimSpace(u.Username.String)
	}
	fn := ""
	ln := ""
	if u.FirstName.Valid {
		fn = strings.TrimSpace(u.FirstName.String)
	}
	if u.LastName.Valid {
		ln = strings.TrimSpace(u.LastName.String)
	}
	full := strings.TrimSpace(fn + " " + ln)
	if full != "" {
		return full
	}
	return fmt.Sprintf("id%d", u.UserID)
}

func prettyManager(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "—"
	}
	return name
}

func langEmoji(lang string) string {
	switch normLang(lang) {
	case "UA":
		return "🇺🇦"
	case "EN":
		return "🇬🇧"
	default:
		return "🇷🇺"
	}
}

func sessionManagerName(ss storage.SupportSession) string {
	// prefer username if present
	if ss.ManagerUser.Valid && strings.TrimSpace(ss.ManagerUser.String) != "" {
		return "@" + strings.TrimSpace(ss.ManagerUser.String)
	}
	fn := ""
	ln := ""
	if ss.ManagerFirst.Valid {
		fn = strings.TrimSpace(ss.ManagerFirst.String)
	}
	if ss.ManagerLast.Valid {
		ln = strings.TrimSpace(ss.ManagerLast.String)
	}
	full := strings.TrimSpace(fn + " " + ln)
	if full != "" {
		return full
	}
	return ""
}

func attachmentText(lang string, m *telego.Message) string {
	lang = normLang(lang)
	switch lang {
	case "UA":
		switch {
		case m.Photo != nil:
			return "🖼 Фото"
		case m.Video != nil:
			return "🎥 Відео"
		case m.VideoNote != nil:
			return "📹 Кружечок"
		case m.Voice != nil:
			return "🎙 Голосове"
		case m.Audio != nil:
			return "🎵 Аудіо"
		case m.Document != nil:
			return "📄 Файл"
		default:
			return "Без тексту"
		}
	case "EN":
		switch {
		case m.Photo != nil:
			return "🖼 Photo"
		case m.Video != nil:
			return "🎥 Video"
		case m.VideoNote != nil:
			return "📹 Video note"
		case m.Voice != nil:
			return "🎙 Voice message"
		case m.Audio != nil:
			return "🎵 Audio"
		case m.Document != nil:
			return "📄 File"
		default:
			return "No text"
		}
	default: // RU
		switch {
		case m.Photo != nil:
			return "🖼 Фото"
		case m.Video != nil:
			return "🎥 Видео"
		case m.VideoNote != nil:
			return "📹 Кружочек"
		case m.Voice != nil:
			return "🎙 Голосовое"
		case m.Audio != nil:
			return "🎵 Аудио"
		case m.Document != nil:
			return "📄 Файл"
		default:
			return "Без текста"
		}
	}
}

// ---------- shared helpers ----------

func normLang(lang string) string {
	lang = strings.ToUpper(strings.TrimSpace(lang))
	switch lang {
	case "RU", "UA", "EN":
		return lang
	default:
		return "RU"
	}
}

func toNullString(s string) sql.NullString {
	s = strings.TrimSpace(s)
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}
