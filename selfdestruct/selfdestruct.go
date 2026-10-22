// Package selfdestruct removes the messages of the bot some time after they have been sent. The timers live in memory,
// so the messages that were not removed before a restart stay in the chats.
package selfdestruct

import (
	log "log/slog"
	"time"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/kozalosev/goSadTgBot/base"
	"github.com/kozalosev/goSadTgBot/logconst"
)

// SendAndDelete sends the message and deletes it after the delay. It returns after sending.
func SendAndDelete(bot base.ExtendedBotAPI, msgToSend tgbotapi.MessageConfig, after time.Duration) {
	sentMsg, err := bot.Send(msgToSend)
	if err != nil {
		log.Error("Failed to call ExtendedBotAPI.Send",
			logconst.FieldFunc, "SendAndDelete",
			logconst.FieldCalledObject, "ExtendedBotAPI",
			logconst.FieldCalledMethod, "Send",
			logconst.FieldError, err)
		return
	}

	go func() {
		time.Sleep(after)
		if err := bot.Request(tgbotapi.NewDeleteMessage(sentMsg.Chat.ID, sentMsg.MessageID)); err != nil {
			log.Error("Couldn't delete a self-destroying message",
				logconst.FieldFunc, "SendAndDelete",
				logconst.FieldError, err)
		}
	}()
}

// ReplaceInline replaces the text of an inline message with the placeholder after the delay. It returns immediately.
// An inline message, including the answer to a guest query, can't be deleted by the bot: Telegram refuses it
// since the bot isn't a member of the chat, even if the chat and the message are decoded from the identifier.
func ReplaceInline(bot base.ExtendedBotAPI, inlineMessageID, placeholder string, after time.Duration) {
	go func() {
		time.Sleep(after)
		edit := tgbotapi.EditMessageTextConfig{
			BaseEdit: tgbotapi.BaseEdit{InlineMessageID: inlineMessageID},
			Text:     placeholder,
		}
		if err := bot.Request(edit); err != nil {
			log.Error("Failed to replace an inline message with the placeholder",
				logconst.FieldFunc, "ReplaceInline",
				"inline_message_id", inlineMessageID,
				logconst.FieldError, err)
		}
	}()
}
