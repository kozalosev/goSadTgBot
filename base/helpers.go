package base

import (
	"fmt"

	"slices"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/loctools/go-l10n/loc"
	"github.com/thoas/go-funk"
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
			IsEphemeral: isEphemeralCommand(h, scope),
		}
	}).([]tgbotapi.BotCommand)
}

// isEphemeralCommand is true only for group chats since ephemeral messages are not supported in private ones.
func isEphemeralCommand(handler CommandHandler, scope CommandScope) bool {
	h, ok := handler.(EphemeralCommandHandler)
	return ok && h.IsEphemeral() && scope != CommandScopeAllPrivateChats
}

// https://stackoverflow.com/a/72408490
func chunkBy[T any](items []T, chunkSize int) (chunks [][]T) {
	for chunkSize < len(items) {
		items, chunks = items[chunkSize:], append(chunks, items[0:chunkSize:chunkSize])
	}
	return append(chunks, items)
}
