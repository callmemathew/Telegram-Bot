// callback.go
//
// Обработка нажатий inline-кнопок (CallbackQuery).
//
// Сейчас используется для:
// - выбора языка (lang:RU / UA / EN)
// - сохранения языка в БД
// - редактирования сообщения с меню языка
// - вызова сервисных функций обновления UI
//
// Здесь нет сложной логики — только реакция на кнопки.

package handlers

import (
	"context"
	"log"
	"strings"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegoutil"
)

func (h *Handlers) handleCallback(ctx context.Context, cb *telego.CallbackQuery) {
	if cb == nil {
		return
	}

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

	userID := cb.From.ID
	chatID := cb.From.ID

	// ensure
	if err := h.support.EnsureUser(ctx, &cb.From, chatID); err != nil {
		log.Println("EnsureUser error:", err)
		return
	}

	// ✅ ВАЖНО: узнаём, был ли язык ДО сохранения (чтобы хинт слать только новым)
	_, hadLang, _ := h.support.GetLang(ctx, userID)
	firstTime := !hadLang

	// save lang
	if err := h.support.SetLang(ctx, userID, lang); err != nil {
		log.Println("SetLang error:", err)
		return
	}

	// редактируем ТО ЖЕ сообщение с меню -> "saved" + убрать кнопки
	if cb.Message != nil {
		emptyKB := &telego.InlineKeyboardMarkup{InlineKeyboard: [][]telego.InlineKeyboardButton{}}

		_, err := h.bot.EditMessageText(ctx, &telego.EditMessageTextParams{
			ChatID:      telegoutil.ID(chatID),
			MessageID:   cb.Message.GetMessageID(),
			Text:        langSavedText(lang),
			ReplyMarkup: emptyKB,
		})
		if err != nil {
			log.Println("Edit lang menu -> saved error:", err)
		}
	}

	// ✅ ХИНТ ТОЛЬКО 1 РАЗ: сразу после первого выбора языка
	if firstTime {
		h.sendText(ctx, chatID, startHintText(lang))
	}

	_ = h.support.RefreshLangUI(ctx, userID)
	if err := h.support.RefreshLangUI(ctx, userID); err != nil {
		log.Println("RefreshLangUI error:", err)
	}
}
