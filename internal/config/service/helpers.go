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

	mediaType, _ := extractMediaInfo(m)

	switch lang {
	case "UA":
		switch mediaType {
		case "photo":
			return "🖼 Фото"
		case "video":
			return "🎥 Відео"
		case "video_note":
			return "📹 Відеоповідомлення"
		case "voice":
			return "🎙 Голосове"
		case "audio":
			return "🎵 Аудіо"
		case "document":
			return "📄 Документ"
		case "animation":
			return "🎞 GIF"
		case "sticker":
			return "💟 Стікер"
		default:
			return "Без тексту"
		}

	case "EN":
		switch mediaType {
		case "photo":
			return "🖼 Photo"
		case "video":
			return "🎥 Video"
		case "video_note":
			return "📹 Video note"
		case "voice":
			return "🎙 Voice message"
		case "audio":
			return "🎵 Audio"
		case "document":
			return "📄 Document"
		case "animation":
			return "🎞 GIF"
		case "sticker":
			return "💟 Sticker"
		default:
			return "No text"
		}

	default: // RU
		switch mediaType {
		case "photo":
			return "🖼 Фото"
		case "video":
			return "🎥 Видео"
		case "video_note":
			return "📹 Видеосообщение"
		case "voice":
			return "🎙 Голосовое"
		case "audio":
			return "🎵 Аудио"
		case "document":
			return "📄 Документ"
		case "animation":
			return "🎞 GIF"
		case "sticker":
			return "💟 Стикер"
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

func extractMediaInfo(m *telego.Message) (string, string) {
	if m == nil {
		return "", ""
	}

	switch {
	case len(m.Photo) > 0:
		p := m.Photo[len(m.Photo)-1]
		return "photo", p.FileID

	case m.Video != nil:
		return "video", m.Video.FileID

	case m.VideoNote != nil:
		return "video_note", m.VideoNote.FileID

	case m.Voice != nil:
		return "voice", m.Voice.FileID

	case m.Audio != nil:
		return "audio", m.Audio.FileID

	case m.Animation != nil:
		return "animation", m.Animation.FileID

	case m.Sticker != nil:
		return "sticker", m.Sticker.FileID

	case m.Document != nil:
		// Telegram часто присылает GIF как document
		mime := strings.ToLower(strings.TrimSpace(m.Document.MimeType))

		switch mime {
		case "image/gif", "video/mp4", "video/webm":
			return "animation", m.Document.FileID
		default:
			return "document", m.Document.FileID
		}

	default:
		return "", ""
	}
}
func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
