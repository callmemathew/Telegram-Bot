package app

import (
	"context"
	"log"
	"net/http"

	"tg-bot/internal/config"
	"tg-bot/internal/config/handlers"
	"tg-bot/internal/config/service"
	"tg-bot/internal/config/storage"

	"github.com/mymmrac/telego"
)

type App struct {
	bot *telego.Bot
	h   *handlers.Handlers
	cfg config.Config
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

	if err := setupCommands(ctx, bot); err != nil {
		log.Println("set commands error:", err)
	}

	return &App{
		bot: bot,
		h:   h,
		cfg: cfg,
	}
}

func setupCommands(ctx context.Context, bot *telego.Bot) error {
	cmds := []telego.BotCommand{
		{Command: "start", Description: "Start bot"},
		{Command: "lang", Description: "Change language"},
	}

	return bot.SetMyCommands(ctx, &telego.SetMyCommandsParams{
		Commands: cmds,
	})
}

func (a *App) Run(ctx context.Context) {
	publicURL := a.cfg.WebhookURL
	if publicURL == "" {
		log.Fatal("WEBHOOK_URL is required")
	}

	webhookPath := a.cfg.WebhookPath
	if webhookPath == "" {
		webhookPath = "/webhook"
	}

	port := a.cfg.Port
	if port == "" {
		port = "8080"
	}

	fullWebhookURL := publicURL + webhookPath

	err := a.bot.SetWebhook(ctx, &telego.SetWebhookParams{
		URL:         fullWebhookURL,
		SecretToken: a.bot.SecretToken(),
	})
	if err != nil {
		log.Fatal(err)
	}

	info, err := a.bot.GetWebhookInfo(ctx)
	if err != nil {
		log.Println("get webhook info error:", err)
	} else {
		log.Printf("webhook set: url=%s pending=%d", info.URL, info.PendingUpdateCount)
	}

	mux := http.NewServeMux()

	updates, err := a.bot.UpdatesViaWebhook(
		ctx,
		telego.WebhookHTTPServeMux(mux, webhookPath, a.bot.SecretToken()),
	)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		addr := ":" + port
		log.Println("Webhook server listening on", addr)

		if err := http.ListenAndServe(addr, mux); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

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
