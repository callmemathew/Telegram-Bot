package main

import (
	"context"
	"log"

	"github.com/joho/godotenv"

	"tg-bot/internal/config"
	"tg-bot/internal/config/app"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println(".env not loaded:", err)
	}

	ctx := context.Background()

	cfg := config.MustLoad()

	log.Println("Bot started")

	a := app.MustNew(ctx, cfg)
	a.Run(ctx)
}
