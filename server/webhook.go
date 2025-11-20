package server

import (
	"fmt"
	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/kozalosev/goSadTgBot/app"
	"github.com/kozalosev/goSadTgBot/logconst"
	log "log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
)

// AddHttpHandlerForWebhook uses [http.HandleFunc] to add a global route to the server.
func AddHttpHandlerForWebhook(bot *tgbotapi.BotAPI, appParams *app.Params, wg *sync.WaitGroup) {
	whParams := getWebhookParamsFromEnv()
	path := fmt.Sprintf("/%s/%s", whParams.path, bot.Token)
	whURL := fmt.Sprintf("https://%s:%s/%s%s", whParams.host, whParams.port, whParams.appPath, path)
	log.Info(fmt.Sprintf("Webhook URL: %s/***", whURL[:len(bot.Token)]),
		logconst.FieldFunc, "addHttpHandlerForWebhook")
	wh, err := tgbotapi.NewWebhook(whURL)
	if err != nil {
		panic(err)
	}
	if _, err := bot.Request(wh); err != nil {
		panic(err)
	}
	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		upd, err := bot.HandleUpdate(r)
		if err != nil {
			log.Error("Couldn't handle the update",
				logconst.FieldFunc, "addHttpHandlerForWebhook",
				logconst.FieldCalledObject, "BotAPI",
				logconst.FieldCalledMethod, "HandleUpdate",
				logconst.FieldError, err)
		} else {
			app.HandleUpdate(appParams, wg, upd)
		}
	})
}

// Read comments in the `.env` file and
// https://github.com/kozalosev/SadFavBot/wiki/Run-and-configuration#on-a-server-production-mode
type webhookParams struct {
	host    string
	port    string
	path    string
	appPath string
}

func getWebhookParamsFromEnv() webhookParams {
	return webhookParams{
		host:    os.Getenv("WEBHOOK_HOST"),
		port:    os.Getenv("WEBHOOK_PORT"),
		path:    strings.TrimPrefix(os.Getenv("WEBHOOK_PATH"), "/"),
		appPath: strings.Trim(os.Getenv("APP_PATH"), "/"),
	}
}
