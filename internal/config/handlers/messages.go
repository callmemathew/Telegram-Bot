package handlers

import (
	"context"
	"log"
	"strings"

	"tg-bot/internal/config/service"

	"github.com/mymmrac/telego"
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
	log.Printf("UPDATE: msg=%v cb=%v", upd.Message != nil, upd.CallbackQuery != nil)

	switch {
	case upd.CallbackQuery != nil:
		h.handleCallback(ctx, upd.CallbackQuery)
	case upd.Message != nil:
		h.handleMessage(ctx, upd.Message)
	}
}
func (h *Handlers) handleMessage(ctx context.Context, m *telego.Message) {
	log.Printf("IN: chat_id=%d chat_type=%s thread_id=%d managersChatID=%d text=%q",
		m.Chat.ID, m.Chat.Type, m.MessageThreadID, h.managersChatID, m.Text,
	)
	if m == nil || m.Chat.ID == 0 {
		return
	}

	// 1) managers forum chat -> replies from managers
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

	// 2) only private user chat
	if m.Chat.Type != telego.ChatTypePrivate {
		return
	}

	h.handleUserPrivate(ctx, m)
}

func (h *Handlers) handleUserPrivate(ctx context.Context, m *telego.Message) {
	if m == nil || m.Chat.ID == 0 || m.From == nil {
		return
	}

	userID := m.From.ID
	chatID := m.Chat.ID

	// always ensure user (после DROP тоже)
	if err := h.support.EnsureUser(ctx, m.From, chatID); err != nil {
		log.Println("EnsureUser error:", err)
	}

	text := strings.TrimSpace(m.Text)
	cmd := parseCmd(text)

	switch cmd {
	case "/start":
		// если языка нет -> открыть меню (редактируем баннер)
		if _, ok, err := h.support.GetLang(ctx, userID); err == nil && !ok {
			if err := h.support.OpenLangMenu(ctx, userID, chatID); err != nil {
				log.Println("OpenLangMenu error:", err)
			}
			return
		}

		// язык есть -> показать hint (редактируем баннер)
		if err := h.support.ShowStartHint(ctx, userID, chatID); err != nil {
			log.Println("ShowStartHint error:", err)
		}
		return

	case "/lang":
		// всегда открываем меню редактированием баннера
		if err := h.support.OpenLangMenu(ctx, userID, chatID); err != nil {
			log.Println("OpenLangMenu error:", err)
		}
		return

	case "":
		// not a command -> continue below

	default:
		// неизвестная команда -> НЕ спамим сообщениями
		// если языка нет -> меню, иначе -> hint
		if _, ok, err := h.support.GetLang(ctx, userID); err == nil && !ok {
			if err := h.support.OpenLangMenu(ctx, userID, chatID); err != nil {
				log.Println("OpenLangMenu error:", err)
			}
		} else {
			if err := h.support.ShowStartHint(ctx, userID, chatID); err != nil {
				log.Println("ShowStartHint error:", err)
			}
		}
		return
	}

	// ===== обычное сообщение (НЕ команда) =====

	// если языка нет -> заставляем выбрать (редактируем баннер) и не форвардим
	if _, ok, err := h.support.GetLang(ctx, userID); err == nil && !ok {
		if err := h.support.OpenLangMenu(ctx, userID, chatID); err != nil {
			log.Println("OpenLangMenu error:", err)
		}
		return
	}

	// форвардим менеджерам
	if err := h.support.OnUserMessage(ctx, m); err != nil {
		log.Println("OnUserMessage error:", err)
	}
}

func (h *Handlers) handleCallback(ctx context.Context, cb *telego.CallbackQuery) {
	if cb == nil {
		return
	}

	// убрать "loading" на кнопке
	_ = h.bot.AnswerCallbackQuery(ctx, &telego.AnswerCallbackQueryParams{
		CallbackQueryID: cb.ID,
	})

	if !strings.HasPrefix(cb.Data, "lang:") {
		return
	}

	userID := cb.From.ID

	// ✅ железно правильный chatID:
	// - в private обычно == userID
	// - но правильнее брать cb.Message.Chat.ID если есть
	chatID := userID

	lang := strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(cb.Data, "lang:")))
	if lang != "RU" && lang != "UA" && lang != "EN" {
		lang = "RU"
	}

	// EnsureUser safe
	if err := h.support.EnsureUser(ctx, &cb.From, chatID); err != nil {
		log.Println("EnsureUser error:", err)
		return
	}

	// Save lang
	if err := h.support.SetLang(ctx, userID, lang); err != nil {
		log.Println("SetLang error:", err)
		return
	}

	// ✅ НЕ отправляем новое сообщение "saved"
	// ✅ просто редактируем баннер на StartHint
	if err := h.support.ShowStartHint(ctx, userID, chatID); err != nil {
		log.Println("ShowStartHint error:", err)
	}

	// manager-side UI refresh (topic title + pin card)
	if err := h.support.RefreshLangUI(ctx, userID); err != nil {
		log.Println("RefreshLangUI error:", err)
	}
}

// helpers

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
