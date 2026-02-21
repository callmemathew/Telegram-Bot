package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"tg-bot/internal/config"
	"tg-bot/internal/config/app"
)

func main() {
	cfg := config.MustLoad()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a := app.MustNew(ctx, cfg)

	log.Println("Bot started")
	a.Run(ctx)
}
