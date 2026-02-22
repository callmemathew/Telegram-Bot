// texts.go
//
// Вспомогательные функции и тексты.
//
// Здесь:
// - parseCmd (разбор команды из текста)
// - тексты меню языка
// - тексты подтверждения
// - текст подсказки startHint
// - текст неизвестной команды
//
// Этот файл нужен для чистоты архитектуры,
// чтобы не смешивать тексты и логику.

package handlers

import (
	"context"
	"strings"

	"github.com/mymmrac/telego/telegoutil"
)

func (h *Handlers) sendLangMenu(ctx context.Context, chatID int64) {
	kb := telegoutil.InlineKeyboard(
		telegoutil.InlineKeyboardRow(
			telegoutil.InlineKeyboardButton("Русский").WithCallbackData("lang:RU"),
			telegoutil.InlineKeyboardButton("Українська").WithCallbackData("lang:UA"),
			telegoutil.InlineKeyboardButton("English").WithCallbackData("lang:EN"),
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
		return "Напишіть ваше повідомлення — команда DocData відповість вам найближчим часом.\n\n💬 Це живий чат підтримки."
	case "EN":
		return "Send your message — the DocData team will respond shortly.\n\n💬 This is a live support chat."
	default:
		return "Напишите ваше сообщение — команда DocData ответит вам в ближайшее время.\n\n💬 Это живой чат поддержки."
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
