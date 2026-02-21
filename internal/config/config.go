package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken       string
	ManagersChatID int64
	DBPath         string
}

func MustLoad() Config {
	_ = godotenv.Load()

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("BOT_TOKEN is empty")
	}

	managers := os.Getenv("MANAGERS_CHAT_ID")
	if managers == "" {
		log.Fatal("MANAGERS_CHAT_ID is empty")
	}
	managersID, err := strconv.ParseInt(managers, 10, 64)
	if err != nil {
		log.Fatal("MANAGERS_CHAT_ID must be int64:", err)
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./docdata.db"
	}

	return Config{
		BotToken:       token,
		ManagersChatID: managersID,
		DBPath:         dbPath,
	}
}
