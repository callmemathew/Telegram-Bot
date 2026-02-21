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

// RefreshLangUI (после SetLang):
// - topic title (если есть thread)
// - pinned card (если есть pinned_msg_id)
// - status bar (если есть status msg) в зависимости от waiting/active
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
// STOP / CLOSE
// =========================

func (s *SupportService) StopSession(ctx context.Context, userID int64) error {
	if err := s.store.CloseSession(ctx, userID); err != nil {
		return err
	}

	// обновим pinned card если есть (чтобы статус стал CLOSED)
	u, err := s.store.GetUserByUserID(ctx, userID)
	if err != nil {
		return nil
	}

	lang := "RU"
	if l, ok, _ := s.store.GetLangByUserID(ctx, userID); ok && strings.TrimSpace(l) != "" {
		lang = normLang(l)
	}

	ss, okS, _ := s.store.GetSessionByUserID(ctx, userID)
	if okS {
		_ = s.UpsertPinnedCard(ctx, u, ss, lang, sessionManagerName(ss))
	}
	return nil
}

// =========================
// STATUS BAR (USER CHAT) — 1 message, edit it
// =========================

func (s *SupportService) UpsertUserStatusBarText(ctx context.Context, userID int64, chatID int64, text string) error {
	msgID, ok, err := s.store.GetStatusMsgID(ctx, userID)
	if err != nil {
		return err
	}

	// edit existing
	if ok && msgID != 0 {
		_, err := s.bot.EditMessageText(ctx, &telego.EditMessageTextParams{
			ChatID:    telegoutil.ID(chatID),
			MessageID: msgID,
			Text:      text,
		})
		if err != nil && strings.Contains(err.Error(), "message is not modified") {
			return nil
		}
		return err
	}

	// send once + save id
	sent, err := s.bot.SendMessage(ctx, telegoutil.Message(telegoutil.ID(chatID), text))
	if err != nil {
		return err
	}
	return s.store.SetStatusMsgID(ctx, userID, sent.MessageID)
}

func isMessageNotModified(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "message is not modified")
}

// удобный вызов: пересчитать текст бара по текущей сессии (без параметров)
func (s *SupportService) UpsertUserStatusBarAuto(ctx context.Context, userID int64, chatID int64) error {
	lang := "RU"
	if l, ok, _ := s.store.GetLangByUserID(ctx, userID); ok && strings.TrimSpace(l) != "" {
		lang = normLang(l)
	}

	ss, okS, _ := s.store.GetSessionByUserID(ctx, userID)
	if okS {
		mn := sessionManagerName(ss)
		return s.UpsertUserStatusBarText(ctx, userID, chatID, statusText(lang, ss.Status, mn))
	}
	return s.UpsertUserStatusBarText(ctx, userID, chatID, statusText(lang, storage.SessionWaiting, ""))
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

	statusLabel := strings.ToUpper(string(ss.Status))
	if statusLabel == "" {
		statusLabel = "WAITING"
	}

	card := fmt.Sprintf(
		"📌 *LIVE SUPPORT SESSION*\n"+
			"👤 *User:* %s\n"+
			"🆔 *user_id:* `%d`\n"+
			"🌍 *Lang:* %s %s\n"+
			"📡 *Status:* `%s`\n"+
			"💼 *Manager:* %s\n",
		displayUser(user),
		user.UserID,
		langEmoji(lang), lang,
		statusLabel,
		prettyManager(managerName),
	)

	// 🔎 Проверяем есть ли pinned id
	pid, hasPin, err := s.store.GetPinnedMsgID(ctx, user.UserID)
	if err != nil {
		return err
	}

	// =====================================================
	// 1️⃣ Если pinned_msg_id есть → всегда редактируем
	// =====================================================
	if hasPin && pid != 0 {
		_, err := s.bot.EditMessageText(ctx, &telego.EditMessageTextParams{
			ChatID:    telegoutil.ID(s.managersChatID),
			MessageID: pid,
			Text:      card,
			ParseMode: "Markdown",
		})

		// игнорируем "not modified"
		if err != nil && !strings.Contains(err.Error(), "message is not modified") {
			return err
		}

		// 🔒 убедимся что закреплено
		_ = s.bot.PinChatMessage(ctx, &telego.PinChatMessageParams{
			ChatID:              telegoutil.ID(s.managersChatID),
			MessageID:           pid,
			DisableNotification: true,
		})

		return nil
	}

	// =====================================================
	// 2️⃣ Если pinned_msg_id нет → создаём и сохраняем
	// =====================================================
	msg := telegoutil.Message(telegoutil.ID(s.managersChatID), card)
	msg.ParseMode = "Markdown"
	msg.MessageThreadID = threadID

	sent, err := s.bot.SendMessage(ctx, msg)
	if err != nil {
		return err
	}

	// сохраняем message_id
	if err := s.store.SetPinnedMsgID(ctx, user.UserID, sent.MessageID); err != nil {
		return err
	}

	// закрепляем
	_ = s.bot.PinChatMessage(ctx, &telego.PinChatMessageParams{
		ChatID:              telegoutil.ID(s.managersChatID),
		MessageID:           sent.MessageID,
		DisableNotification: true,
	})

	return nil
}

// =========================
// USER -> MANAGERS
//  - TEXT: 1 message
//  - ATTACH: (optional header) + CopyMessage
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

	// ensure session exists
	ss, okS, err := s.store.GetSessionByUserID(ctx, user.ID)
	if err != nil {
		return err
	}
	if !okS {
		_ = s.store.UpsertSessionWaiting(ctx, user.ID, threadID, 0)
		ss, _, _ = s.store.GetSessionByUserID(ctx, user.ID)
	}

	// if CLOSED -> reset to WAITING (so managers can connect again)
	if ss.Status == storage.SessionClosed {
		_ = s.store.ResetSessionToWaiting(ctx, user.ID, threadID)
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

func (s *SupportService) TouchWaiting(ctx context.Context, userID int64) error {
	u, err := s.store.GetUserByUserID(ctx, userID)
	if err != nil {
		return err
	}

	lang := "RU"
	if l, ok, _ := s.store.GetLangByUserID(ctx, userID); ok && strings.TrimSpace(l) != "" {
		lang = normLang(l)
	}

	// если тред есть — reset waiting (чтобы closed точно ушёл)
	if u.ThreadID.Valid && u.ThreadID.Int64 != 0 {
		_ = s.store.ResetSessionToWaiting(ctx, userID, int(u.ThreadID.Int64))
	}

	// бар НЕ создаём, если его нет
	_, hasBar, _ := s.store.GetStatusMsgID(ctx, userID)
	if hasBar {
		return s.UpsertUserStatusBarText(ctx, userID, u.ChatID, statusText(lang, storage.SessionWaiting, ""))
	}
	return nil
}

// =========================
// MANAGER -> USER
//  - /ready OR first reply activates session
//  - TEXT: 1 message
//  - ATTACH: (optional header) + CopyMessage
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

	// lang
	lang := "RU"
	if l, ok, _ := s.store.GetLangByUserID(ctx, u.UserID); ok && strings.TrimSpace(l) != "" {
		lang = normLang(l)
	}

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
	if ss.Status != storage.SessionActive {
		if isReadyCmd {
			shouldActivate = true
		} else if hasAtt {
			shouldActivate = true
		} else if txt != "" && !strings.HasPrefix(txt, "/") {
			shouldActivate = true
		}
	}

	if shouldActivate {
		// make sure pinned exists/updated
		_ = s.UpsertPinnedCard(ctx, u, ss, lang, managerName)

		// read pinned id
		pid, okP, _ := s.store.GetPinnedMsgID(ctx, u.UserID)
		var pinnedMsgID int64
		if okP && pid != 0 {
			pinnedMsgID = int64(pid)
		}

		// activate session
		manager := storage.User{
			UserID:    m.From.ID,
			Username:  toNullString(m.From.Username),
			FirstName: toNullString(m.From.FirstName),
			LastName:  toNullString(m.From.LastName),
		}
		_ = s.store.ActivateSession(ctx, u.UserID, manager, pinnedMsgID)

		// reload + update pinned (ACTIVE)
		ss, _, _ = s.store.GetSessionByUserID(ctx, u.UserID)
		_ = s.UpsertPinnedCard(ctx, u, ss, lang, managerName)

		if isReadyCmd {
			return nil
		}
	}

	// ✅ USER CHAT OUTPUT (clean)
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

func (s *SupportService) StartSession(ctx context.Context, userID int64, chatID int64) error {
	// 0) Ensure user (на всякий)
	_ = s.store.EnsureUser(ctx, storage.User{UserID: userID, ChatID: chatID})

	// 1) язык должен быть
	lang := "RU"
	if l, ok, _ := s.store.GetLangByUserID(ctx, userID); ok && strings.TrimSpace(l) != "" {
		lang = normLang(l)
	}

	// 2) threadID если есть — ок (если нет, будет создан при первом сообщении юзера)
	threadID, okT, _ := s.store.GetThreadByUserID(ctx, userID)
	if !okT {
		threadID = 0
	}

	// 3) СЕССИЯ -> WAITING (важно: сбрасываем closed/active)
	_ = s.store.ResetSessionToWaiting(ctx, userID, threadID)

	// 4) БАР: должен стать WAITING
	// если бара нет — создаём один раз
	if _, hasBar, _ := s.store.GetStatusMsgID(ctx, userID); !hasBar {
		return s.UpsertUserStatusBarText(ctx, userID, chatID, statusText(lang, storage.SessionWaiting, ""))
	}

	// если бар есть — просто редактируем в WAITING
	return s.UpsertUserStatusBarText(ctx, userID, chatID, statusText(lang, storage.SessionWaiting, ""))
}

func (s *SupportService) BindStatusBarToMessage(
	ctx context.Context,
	userID int64,
	chatID int64,
	msgID int,
	lang string,
) error {
	// 1) store msgID as status-bar id
	if err := s.store.SetStatusMsgID(ctx, userID, msgID); err != nil {
		return err
	}

	lang = normLang(lang)

	// 2) detect current session state
	ss, ok, err := s.store.GetSessionByUserID(ctx, userID)
	if err != nil {
		return err
	}

	state := storage.SessionWaiting
	managerName := ""

	if ok {
		state = ss.Status
		managerName = sessionManagerName(ss)
	}

	barText := statusText(lang, state, managerName)

	// 3) edit THIS message text (turn menu msg -> bar msg)
	_, err = s.bot.EditMessageText(ctx, &telego.EditMessageTextParams{
		ChatID:    telegoutil.ID(chatID),
		MessageID: msgID,
		Text:      barText,
	})

	if isMessageNotModified(err) {
		return nil
	}
	return err
}

// =========================
// helpers
// =========================

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

func statusText(lang string, st storage.SessionStatus, managerName string) string {
	lang = normLang(lang)

	switch st {
	case storage.SessionActive:
		switch lang {
		case "UA":
			return fmt.Sprintf("✅ Менеджер підключився: %s\nПишіть повідомлення — це живий чат.", prettyManager(managerName))
		case "EN":
			return fmt.Sprintf("✅ Manager connected: %s\nYou can message here — live chat.", prettyManager(managerName))
		default:
			return fmt.Sprintf("✅ Менеджер подключился: %s\nПиши сюда — это живой чат.", prettyManager(managerName))
		}

	case storage.SessionClosed:
		switch lang {
		case "UA":
			return "⛔ Діалог завершено. Щоб почати знову — /start"
		case "EN":
			return "⛔ Dialog closed. To start again — /start"
		default:
			return "⛔ Диалог завершён. Чтобы начать заново — /start"
		}

	default: // waiting
		switch lang {
		case "UA":
			return "⏳ Підключаємо менеджера…\nЗазвичай до 1 хв."
		case "EN":
			return "⏳ Connecting a manager…\nUsually up to 1 minute."
		default:
			return "⏳ Подключаем менеджера…\nОбычно до 1 минуты."
		}
	}
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

func (s *SupportService) OpenLangMenu(ctx context.Context, userID, chatID int64) error {
	// гарантируем запись юзера
	_ = s.store.EnsureUser(ctx, storage.User{UserID: userID, ChatID: chatID})

	kb := telegoutil.InlineKeyboard(
		telegoutil.InlineKeyboardRow(
			telegoutil.InlineKeyboardButton("Русский").WithCallbackData("lang:RU"),
			telegoutil.InlineKeyboardButton("Українська").WithCallbackData("lang:UA"),
			telegoutil.InlineKeyboardButton("English").WithCallbackData("lang:EN"),
		),
	)

	// если бар уже есть -> редактируем его и показываем клаву
	mid, ok, _ := s.store.GetStatusMsgID(ctx, userID)
	if ok && mid != 0 {
		_, err := s.bot.EditMessageText(ctx, &telego.EditMessageTextParams{
			ChatID:      telegoutil.ID(chatID),
			MessageID:   mid,
			Text:        "🌍 Выберите язык / Оберіть мову / Choose language:",
			ReplyMarkup: kb, // telego позволяет
		})
		if isMessageNotModified(err) {
			return nil
		}
		return err
	}

	// иначе -> отправим одно сообщение и СРАЗУ привяжем как бар
	msg := telegoutil.Message(telegoutil.ID(chatID), "🌍 Выберите язык / Оберіть мову / Choose language:")
	msg.ReplyMarkup = kb

	sent, err := s.bot.SendMessage(ctx, msg)
	if err != nil {
		return err
	}

	// привязали message_id к user_header_msg
	return s.store.SetStatusMsgID(ctx, userID, sent.MessageID)
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
