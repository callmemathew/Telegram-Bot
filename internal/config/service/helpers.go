package service

import (
	"database/sql"
	"fmt"
	"strings"

	"tg-bot/internal/config/storage"

	"github.com/mymmrac/telego"
)

// helpers.go
//
// Мелкие утилиты и форматирование:
// - нормализация языка
// - преобразование строк в sql.NullString
// - формат имён, эмодзи языка, и прочие helper-функции
// - проверка вложений и подписи

func isMessageNotModified(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "message is not modified")
}

func isUneditableMessage(err error) bool {
	if err == nil {
		return false
	}
	e := strings.ToLower(err.Error())
	return strings.Contains(e, "message to edit not found") ||
		strings.Contains(e, "message can't be edited") ||
		strings.Contains(e, "message is too old") ||
		strings.Contains(e, "message_id_invalid") ||
		strings.Contains(e, "message not found") ||
		strings.Contains(e, "message identifier is not specified") ||
		strings.Contains(e, "bad request: message to edit not found")
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
	return fmt.Sprintf("id%d", u.TelegramUserID)
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
	default:
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
