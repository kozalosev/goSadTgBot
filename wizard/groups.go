package wizard

import (
	log "log/slog"
	"time"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/kozalosev/goSadTgBot/base"
	"github.com/kozalosev/goSadTgBot/logconst"
	"github.com/kozalosev/goSadTgBot/selfdestruct"
)

// The cleanup of group chats after the wizards:
//   - a question answered in a chat where the bot may delete messages of other users is deleted the delay after the
//     answer, together with the answer and the message the question was asked in reply to (the command or the previous
//     answer);
//   - if the bot may not delete messages of other users, an answered question is kept: the answer would be left alone
//     otherwise;
//   - a question nobody has answered is deleted when the form expires, whatever the rights of the bot are.
//
// A question is scheduled for deletion as unanswered right after it's asked, and the answer cancels that deletion. The
// schedule is kept by the selfdestruct package, so it survives a restart if the package is started with a Redis queue.
var (
	selfDestructionDelayInGroups       time.Duration
	unansweredQuestionLifetimeInGroups time.Duration
)

// SetSelfDestructionDelayInGroups makes the questions of the wizards and the answers of the users to them disappear
// from group chats after the delay, so that filling a form doesn't litter the chat. Zero disables it. See also
// [SetUnansweredQuestionLifetimeInGroups].
func SetSelfDestructionDelayInGroups(delay time.Duration) {
	selfDestructionDelayInGroups = delay
}

// SetUnansweredQuestionLifetimeInGroups sets how long a question nobody has answered lives in a group chat. Pass the
// lifetime of the state of a form, so that the question isn't deleted while the user can still answer it. By default,
// it's the delay of [SetSelfDestructionDelayInGroups].
func SetUnansweredQuestionLifetimeInGroups(lifetime time.Duration) {
	unansweredQuestionLifetimeInGroups = lifetime
}

// reply sends the text of the wizard in reply to the message: with ForceReply and self-destroying in group chats.
func (form *Form) reply(msg *tgbotapi.Message, text string, customizer base.MessageCustomizer) {
	customizer = replyCustomizer(msg, customizer)
	bot := form.resources.appEnv.Bot
	if !shouldSelfDestruct(msg) {
		bot.ReplyWithMessageCustomizer(msg, text, customizer)
		return
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ReplyParameters.MessageID = msg.MessageID
	customizer(&reply)
	sent, err := bot.Send(reply)
	if err != nil {
		log.Error("Failed to send a question of the wizard",
			logconst.FieldFunc, "Form.reply",
			logconst.FieldCalledObject, "Bot",
			logconst.FieldCalledMethod, "Send",
			logconst.FieldError, err)
		return
	}

	// deleted as unanswered unless handleAnswer cancels it
	selfdestruct.Schedule(bot, selfdestruct.Task{
		Kind:             selfdestruct.KindDelete,
		ChatID:           msg.Chat.ID,
		MessageID:        sent.MessageID,
		ReplyToMessageID: msg.MessageID,
	}, unansweredQuestionLifetime())
}

// handleAnswer cancels the deletion of the question as unanswered and, if the bot may delete messages of other users
// in the group, deletes the answer, the question and the message the question was asked in reply to after the delay.
func (form *Form) handleAnswer(msg *tgbotapi.Message) {
	if !shouldSelfDestruct(msg) {
		return
	}

	bot := form.resources.appEnv.Bot
	toDelete := []int{msg.MessageID}
	if replyTo := msg.ReplyToMessage; replyTo != nil {
		toDelete = append(toDelete, replyTo.MessageID)
		if question, ok := selfdestruct.CancelDeletion(bot, msg.Chat.ID, replyTo.MessageID); ok && question.ReplyToMessageID != 0 {
			toDelete = append(toDelete, question.ReplyToMessageID)
		}
	}

	if checker, ok := bot.(base.DeletionRightsChecker); !ok || !checker.CanDeleteMessages(msg.Chat.ID) {
		return // the dialogue is kept as is: the answers can't be deleted
	}
	for _, messageID := range toDelete {
		selfdestruct.DeleteAfter(bot, msg.Chat.ID, messageID, selfDestructionDelayInGroups)
	}
}

func unansweredQuestionLifetime() time.Duration {
	if unansweredQuestionLifetimeInGroups > 0 {
		return unansweredQuestionLifetimeInGroups
	}
	return selfDestructionDelayInGroups
}

// shouldSelfDestruct is true for non-ephemeral messages in group chats if the self-destruction is enabled.
// Ephemeral messages disappear by themselves.
func shouldSelfDestruct(msg *tgbotapi.Message) bool {
	return selfDestructionDelayInGroups > 0 && base.IsGroupChat(&msg.Chat) && msg.EphemeralMessageID == 0
}
