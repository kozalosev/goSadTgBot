package base

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/loctools/go-l10n/loc"
	"github.com/thoas/go-funk"
	"golang.org/x/exp/slices"
)

func filterCommandsByScope(handlers []CommandHandler, scope CommandScope, lc *loc.Context) []tgbotapi.BotCommand {
	privateChatCommands := funk.Filter(handlers, func(h CommandHandler) bool {
		return slices.Contains(h.GetScopes(), scope)
	}).([]CommandHandler)
	return funk.Map(privateChatCommands, func(h CommandHandler) tgbotapi.BotCommand {
		mainCmd := h.GetCommands()[0]
		description := lc.Tr(fmt.Sprintf(cmdTrTemplate, mainCmd))
		return tgbotapi.BotCommand{
			Command:     mainCmd,
			Description: description,
		}
	}).([]tgbotapi.BotCommand)
}
