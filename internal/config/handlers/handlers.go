// handlers.go
//
// Главный файл обработчиков.
// Здесь создаётся структура Handlers и происходит маршрутизация апдейтов:
// - если CallbackQuery → handleCallback
// - если Message → handleMessage
//
// Этот файл НЕ содержит бизнес-логики.
// Он только распределяет события дальше.

package handlers

import (
	"context"
	"log"

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
