package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	BotToken       string
	DBPath         string
	ManagersChatID int64
	WebhookURL     string
	WebhookPath    string
	Port           string
}

func MustLoad() Config {
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN is required")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		log.Fatal("DB_PATH is required")
	}

	managersChatIDStr := os.Getenv("MANAGERS_CHAT_ID")
	if managersChatIDStr == "" {
		log.Fatal("MANAGERS_CHAT_ID is required")
	}

	managersChatID, err := strconv.ParseInt(managersChatIDStr, 10, 64)
	if err != nil {
		log.Fatalf("invalid MANAGERS_CHAT_ID: %v", err)
	}

	webhookURL := os.Getenv("WEBHOOK_URL")
	if webhookURL == "" {
		log.Fatal("WEBHOOK_URL is required")
	}

	webhookPath := os.Getenv("WEBHOOK_PATH")
	if webhookPath == "" {
		webhookPath = "/webhook"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return Config{
		BotToken:       botToken,
		DBPath:         dbPath,
		ManagersChatID: managersChatID,
		WebhookURL:     webhookURL,
		WebhookPath:    webhookPath,
		Port:           port,
	}
}
