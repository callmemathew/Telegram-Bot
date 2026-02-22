// user.go
//
// Обработка сообщений из личного чата пользователя.
//
// Здесь:
// - логика /start
// - логика /lang
// - неизвестные команды
// - проверка выбран ли язык
// - передача обычных сообщений в сервис (OnUserMessage)
//
// Это слой между Telegram и бизнес-логикой.

package handlers

import (
	"context"
	"log"
	"strings"

	"github.com/mymmrac/telego"
)

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
}
