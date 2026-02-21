package app

import (
	"context"
	"log"

	"tg-bot/internal/config"
	"tg-bot/internal/config/handlers"
	"tg-bot/internal/config/service"
	"tg-bot/internal/config/storage"

	"github.com/mymmrac/telego"
)

type App struct {
	bot *telego.Bot
	h   *handlers.Handlers
}

func MustNew(ctx context.Context, cfg config.Config) *App {
	// bot
	bot, err := telego.NewBot(cfg.BotToken, telego.WithDefaultLogger(false, true))
	if err != nil {
		log.Fatal(err)
	}

	// db
	db, err := storage.OpenSQLite(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}

	store := storage.NewSQLiteStore(db)

	if err := store.Init(ctx); err != nil {
		log.Fatal(err)
	}

	support := service.NewSupportService(bot, cfg.ManagersChatID, store)
	h := handlers.New(bot, support, cfg.ManagersChatID)

	return &App{bot: bot, h: h}
}
func setupCommands(ctx context.Context, bot *telego.Bot) error {
	cmds := []telego.BotCommand{
		{Command: "start", Description: "Start bot"},
		{Command: "stop", Description: "Stop / reset"},
		{Command: "lang", Description: "Change language"},
	}
	if err := setupCommands(ctx, bot); err != nil {
		log.Println("set commands error:", err)
	}
	return bot.SetMyCommands(ctx, &telego.SetMyCommandsParams{
		Commands: cmds,
	})

}
func (a *App) Run(ctx context.Context) {
	updates, err := a.bot.UpdatesViaLongPolling(ctx, &telego.GetUpdatesParams{Timeout: 60})
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopped")
			return
		case upd, ok := <-updates:
			if !ok {
				log.Println("Updates closed")
				return
			}
			a.h.HandleUpdate(ctx, upd)
		}
	}
}
