// Package selfdestruct removes the messages of the bot some time after they have been sent. The timers live in memory,
// so the messages that were not removed before a restart stay in the chats.
package selfdestruct

import (
	"context"
	"errors"
	log "log/slog"
	"net/http"
	"strings"
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

// DeleteAfter deletes a message of another user after the delay. It returns immediately. The bot needs the right to
// delete messages in the group for this, so Telegram's refusal is expected and logged at the debug level only;
// other failures are errors.
func DeleteAfter(bot base.ExtendedBotAPI, chatID int64, messageID int, after time.Duration) {
	go func() {
		time.Sleep(after)
		err := bot.Request(tgbotapi.NewDeleteMessage(chatID, messageID))
		if err == nil {
			return
		}
		level := log.LevelError
		if isExpectedDeletionFailure(err) {
			level = log.LevelDebug
		}
		log.Log(context.Background(), level, "Couldn't delete a message of a user",
			logconst.FieldFunc, "DeleteAfter",
			logconst.FieldError, err)
	}()
}

// isExpectedDeletionFailure tells if Telegram has refused to delete a message for a reason the bot can't help:
// it has no right to delete messages of other users in the chat (or the message is older than 48 hours), or the
// message has already been deleted by its author or an admin.
func isExpectedDeletionFailure(err error) bool {
	var apiErr *tgbotapi.Error
	if !errors.As(err, &apiErr) || apiErr.Code != http.StatusBadRequest {
		return false
	}
	return strings.Contains(apiErr.Message, "message can't be deleted") ||
		strings.Contains(apiErr.Message, "message to delete not found")
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
