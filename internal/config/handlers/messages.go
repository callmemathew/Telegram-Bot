package handlers

import (
	"context"
	"log"
	"strings"

	"tg-bot/internal/config/service"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
)

type Handlers struct {
	bot            *telego.Bot
	support        *service.SupportService
	managersChatID int64
}

func New(bot *telego.Bot, support *service.SupportService, managersChatID int64) *Handlers {
	return &Handlers{
		bot:            bot,
		support:        support,
		managersChatID: managersChatID,
	}
}

func (h *Handlers) HandleUpdate(ctx context.Context, upd telego.Update) {
	switch {
	case upd.CallbackQuery != nil:
		h.handleCallback(ctx, upd.CallbackQuery)
	case upd.Message != nil:
		h.handleMessage(ctx, upd.Message)
	}
}

func (h *Handlers) handleMessage(ctx context.Context, m *telego.Message) {
	if m == nil || m.Chat.ID == 0 {
		return
	}

	// 1) Managers forum chat -> replies from managers
	if m.Chat.ID == h.managersChatID {
		// ignore "General"
		if m.MessageThreadID == 0 {
			return
		}
		if err := h.support.OnManagerReply(ctx, m); err != nil {
			log.Println("OnManagerReply error:", err)
		}
		return
	}

	// 2) Only private user chat
	if m.Chat.Type != telego.ChatTypePrivate {
		return
	}

	h.handleUserPrivate(ctx, m)
}

func (h *Handlers) handleUserPrivate(ctx context.Context, m *telego.Message) {
	if m == nil || m.Chat.ID == 0 {
		return
	}
	chatID := m.Chat.ID

	// EnsureUser always (so SetLang never fails even after DROP)
	if m.From != nil {
		if err := h.support.EnsureUser(ctx, m.From, chatID); err != nil {
			log.Println("EnsureUser error:", err)
		}
	}

	text := strings.TrimSpace(m.Text)
	cmd := parseCmd(text)

	// lang for help texts
	lang := "RU"
	if m.From != nil {
		if l, ok, err := h.support.GetLang(ctx, m.From.ID); err == nil && ok && strings.TrimSpace(l) != "" {
			lang = strings.ToUpper(strings.TrimSpace(l))
		}
	}

	switch cmd {
	case "/start":
		// /start = если языка нет -> меню, если язык есть -> подсказка "пиши"
		if m.From == nil {
			return
		}
		if _, ok, _ := h.support.GetLang(ctx, m.From.ID); !ok {
			h.sendLangMenu(ctx, chatID)
			return
		}
		h.sendText(ctx, chatID, startHintText(lang))
		return

	case "/lang":
		h.sendLangMenu(ctx, chatID)
		return

	case "/stop":
		// если хочешь можно ничего не делать, но лучше подсказка
		h.sendText(ctx, chatID, stoppedText(lang))
		return

	case "":
		// not a command -> continue

	default:
		h.sendText(ctx, chatID, unknownCmdText(lang))
		return
	}

	// normal user message -> forward to managers
	if m.From == nil {
		return
	}

	// if language not chosen -> show menu and do not forward
	if _, ok, _ := h.support.GetLang(ctx, m.From.ID); !ok {
		h.sendLangMenu(ctx, chatID)
		return
	}

	if err := h.support.OnUserMessage(ctx, m); err != nil {
		log.Println("OnUserMessage error:", err)
		h.sendText(ctx, chatID, "⚠️ support error: "+err.Error())
		return
	}

	// ✅ no extra replies here
}

func (h *Handlers) handleCallback(ctx context.Context, cb *telego.CallbackQuery) {
	if cb == nil {
		return
	}

	// remove "loading"
	_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
		CallbackQueryID: cb.ID,
	})

	if !strings.HasPrefix(cb.Data, "lang:") {
		return
	}

	lang := strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(cb.Data, "lang:")))
	if lang != "RU" && lang != "UA" && lang != "EN" {
		lang = "RU"
	}

	// EnsureUser again (safe)
	if err := h.support.EnsureUser(ctx, &cb.From, cb.From.ID); err != nil {
		log.Println("EnsureUser error:", err)
		return
	}

	if err := h.support.SetLang(ctx, cb.From.ID, lang); err != nil {
		log.Println("SetLang error:", err)
		return
	}

	// remove keyboard from the menu message
	if cb.Message != nil {
		msgID := cb.Message.GetMessageID()
		_, _ = h.bot.EditMessageReplyMarkup(ctx, &telego.EditMessageReplyMarkupParams{
			ChatID:      telegoutil.ID(cb.From.ID),
			MessageID:   msgID,
			ReplyMarkup: &telego.InlineKeyboardMarkup{},
		})
	}

	// one confirmation message
	h.sendText(ctx, cb.From.ID, langSavedText(lang))
	h.sendText(ctx, cb.From.ID, startHintText(lang))

	// ✅ если хочешь обновлять topic title / pinned card — оставь, если нет — удали строку
	_ = h.support.RefreshLangUI(ctx, cb.From.ID)
}

func (h *Handlers) sendLangMenu(ctx context.Context, chatID int64) {
	kb := telegoutil.InlineKeyboard(
		telegoutil.InlineKeyboardRow(
			telegoutil.InlineKeyboardButton("🇷🇺 Русский").WithCallbackData("lang:RU"),
			telegoutil.InlineKeyboardButton("🇺🇦 Українська").WithCallbackData("lang:UA"),
			telegoutil.InlineKeyboardButton("🇬🇧 English").WithCallbackData("lang:EN"),
		),
	)

	msg := telegoutil.Message(telegoutil.ID(chatID), pickLangText())
	msg.ReplyMarkup = kb
	_, _ = h.bot.SendMessage(ctx, msg)
}

func (h *Handlers) sendText(ctx context.Context, chatID int64, text string) {
	_, _ = h.bot.SendMessage(ctx, telegoutil.Message(telegoutil.ID(chatID), text))
}

// --------------------
// helpers / texts
// --------------------

func parseCmd(text string) string {
	if !strings.HasPrefix(text, "/") {
		return ""
	}
	cmd := strings.ToLower(strings.Fields(text)[0])
	if i := strings.Index(cmd, "@"); i >= 0 {
		cmd = cmd[:i]
	}
	return cmd
}

func pickLangText() string {
	return "🌍 Выберите язык / Оберіть мову / Choose language:"
}

func langSavedText(lang string) string {
	switch strings.ToUpper(lang) {
	case "UA":
		return "✅ Мову збережено."
	case "EN":
		return "✅ Language saved."
	default:
		return "✅ Язык сохранён."
	}
}

func startHintText(lang string) string {
	switch strings.ToUpper(lang) {
	case "UA":
		return "✍️ Пишіть повідомлення — я передам менеджерам."
	case "EN":
		return "✍️ Send a message — I’ll forward it to managers."
	default:
		return "✍️ Напиши сообщение — я передам менеджерам."
	}
}

func stoppedText(lang string) string {
	switch strings.ToUpper(lang) {
	case "UA":
		return "⛔ Зупинив. Щоб почати знову — /start"
	case "EN":
		return "⛔ Stopped. To start again — /start"
	default:
		return "⛔ Остановил. Чтобы начать заново — /start"
	}
}

func unknownCmdText(lang string) string {
	switch strings.ToUpper(lang) {
	case "UA":
		return "🤷‍♂️ Невідома команда. Використайте /start або /lang."
	case "EN":
		return "🤷‍♂️ Unknown command. Use /start or /lang."
	default:
		return "🤷‍♂️ Неизвестная команда. Используй /start или /lang."
	}
}
