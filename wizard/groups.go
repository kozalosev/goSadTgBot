package wizard

import (
	"errors"
	log "log/slog"
	"slices"
	"time"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/kozalosev/goSadTgBot/base"
	"github.com/kozalosev/goSadTgBot/logconst"
	"github.com/kozalosev/goSadTgBot/selfdestruct"
	"github.com/loctools/go-l10n/loc"
)

// The cleanup of group chats after the wizards is chosen by the bot: the package keeps the messages of the dialogue of
// a form and asks the [GroupCleanupPolicy] what to delete and when. Without a policy, nothing is deleted.
//
// The dialogue is deleted by one task of the selfdestruct package. A task for a dialogue nobody continues is scheduled
// after every question, replacing the previous one, and is cancelled by the answer; a task for the completed dialogue is
// scheduled when the last answer is given. The schedule survives a restart if the selfdestruct package is started with
// a Redis queue.

// DialogueStage tells what has happened to the dialogue of a form in a group chat.
type DialogueStage int

const (
	// StageWaiting means a question has been asked and waits for an answer. The cleanup is canceled by the answer, so
	// it's what happens to a dialogue nobody continues.
	StageWaiting DialogueStage = iota
	// StageCompleted means the last answer has been given, and the action of the form is run.
	StageCompleted
	// StageCancelled means the user has canceled the form (see [CancelGroupDialogue]).
	StageCancelled
)

// GroupDialogue is the dialogue of a form in a group chat, passed to a [GroupCleanupPolicy].
type GroupDialogue struct {
	Stage  DialogueStage
	ChatID base.ChatID
	// QuestionIDs are the messages of the bot: the questions and the messages about invalid answers.
	QuestionIDs []base.MessageID
	// UserMessageIDs are the messages of the user: the command and the answers.
	UserMessageIDs []base.MessageID
	// CanDeleteOthers tells if the bot may delete the messages of the user in the chat.
	CanDeleteOthers bool
	// Lang is the language of the user, e.g. for the text of a warning.
	Lang *loc.Context
}

// MessageIDs returns the identifiers of all the messages of the dialogue, of the bot and of the user.
func (d GroupDialogue) MessageIDs() []base.MessageID {
	return slices.Concat(d.QuestionIDs, d.UserMessageIDs)
}

// GroupCleanup is what to do with the dialogue: delete the messages after the delay, showing the warning before it if
// it's set. Nothing is deleted if Delete is empty.
type GroupCleanup struct {
	Delete  []base.MessageID
	After   time.Duration
	Warning *selfdestruct.Warning
}

// GroupCleanupPolicy decides what to delete from a group chat at a stage of the dialogue of a form.
type GroupCleanupPolicy func(GroupDialogue) GroupCleanup

var groupCleanupPolicy GroupCleanupPolicy

// SetGroupCleanupPolicy sets how the dialogues of the forms are cleaned up in group chats. nil disables the cleanup.
// See [CleanupIfAllowed] for a ready one.
func SetGroupCleanupPolicy(policy GroupCleanupPolicy) {
	groupCleanupPolicy = policy
}

// CleanupIfAllowed deletes the whole dialogue the delay after it's completed or canceled, or abandonedAfter the last
// question if nobody answers it (pass the lifetime of the state of a form, so that the question isn't deleted while
// the user can still answer it). It does so only where the bot may delete messages of other users; elsewhere, nothing
// is deleted, so that the messages of the user aren't left alone.
func CleanupIfAllowed(delay, abandonedAfter time.Duration) GroupCleanupPolicy {
	return func(d GroupDialogue) GroupCleanup {
		if !d.CanDeleteOthers {
			return GroupCleanup{}
		}
		after := delay
		if d.Stage == StageWaiting {
			after = abandonedAfter
		}
		return GroupCleanup{Delete: d.MessageIDs(), After: after}
	}
}

// dialogue is the part of the state of a form that keeps its dialogue in a group chat.
type dialogue struct {
	QuestionIDs    []base.MessageID `json:"questionIDs,omitempty"`
	UserMessageIDs []base.MessageID `json:"userMessageIDs,omitempty"`
	// PendingID is the message by which the task deleting the dialogue nobody continues is scheduled.
	PendingID base.MessageID `json:"pendingID,omitempty"`
}

func (d *dialogue) addQuestion(messageID base.MessageID) {
	d.QuestionIDs = append(d.QuestionIDs, messageID)
}

// addUserMessage adds the message once: the answer is added when it's received and again when the next question is
// asked in reply to it.
func (d *dialogue) addUserMessage(messageID base.MessageID) {
	if !slices.Contains(d.UserMessageIDs, messageID) {
		d.UserMessageIDs = append(d.UserMessageIDs, messageID)
	}
}

// reply sends the text of the wizard in reply to the message: with ForceReply in group chats, and as a part of the
// dialogue cleaned up by the policy.
func (form *Form) reply(reqenv *base.RequestEnv, msg *tgbotapi.Message, text string, customizer base.MessageCustomizer) {
	customizer = replyCustomizer(msg, customizer)
	bot := form.resources.appEnv.Bot
	if !tracksDialogue(msg) {
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
	form.Dialogue.addUserMessage(msg.MessageID) // the command; the answers are added by handleAnswer
	form.Dialogue.addQuestion(sent.MessageID)
	form.scheduleCleanup(reqenv, msg.Chat.ID, StageWaiting)
}

// handleAnswer cancels the cleanup of the dialogue nobody continues and adds the answer to the dialogue.
func (form *Form) handleAnswer(msg *tgbotapi.Message) {
	if !tracksDialogue(msg) {
		return
	}
	bot := form.resources.appEnv.Bot
	if form.Dialogue.PendingID != 0 {
		selfdestruct.CancelDeletion(bot, msg.Chat.ID, form.Dialogue.PendingID)
		form.Dialogue.PendingID = 0
	} else if msg.ReplyToMessage != nil {
		// the forms of older versions scheduled the deletion of a question by the question itself
		selfdestruct.CancelDeletion(bot, msg.Chat.ID, msg.ReplyToMessage.MessageID)
	}
	form.Dialogue.addUserMessage(msg.MessageID)
}

// CancelGroupDialogue cleans up the dialogue of the form of the user by the policy at [StageCancelled], so that
// a canceled dialogue doesn't wait for the form to expire. Call it before deleting the state of the form.
func CancelGroupDialogue(appenv *base.ApplicationEnv, stateStorage StateStorage, reqenv *base.RequestEnv, msg *tgbotapi.Message) {
	if !tracksDialogue(msg) {
		return
	}
	var form Form
	if err := stateStorage.GetCurrentState(msg.From.ID, &form); err != nil {
		if !errors.Is(err, ErrNoState) {
			log.Error("Failed to get the state of a wizard",
				logconst.FieldFunc, "CancelGroupDialogue",
				logconst.FieldCalledObject, "StateStorage",
				logconst.FieldCalledMethod, "GetCurrentState",
				logconst.FieldError, err)
		}
		return
	}
	if !form.BelongsTo(msg.Chat.ID) || len(form.Dialogue.QuestionIDs) == 0 {
		return
	}
	form.resources = NewEnv(appenv, stateStorage)
	if form.Dialogue.PendingID != 0 {
		selfdestruct.CancelDeletion(appenv.Bot, msg.Chat.ID, form.Dialogue.PendingID)
	}
	form.scheduleCleanup(reqenv, msg.Chat.ID, StageCancelled)
}

// completeDialogue schedules the cleanup of the completed dialogue, if there was one.
func (form *Form) completeDialogue(reqenv *base.RequestEnv, msg *tgbotapi.Message) {
	if tracksDialogue(msg) && len(form.Dialogue.QuestionIDs) > 0 {
		form.scheduleCleanup(reqenv, msg.Chat.ID, StageCompleted)
	}
}

func (form *Form) scheduleCleanup(reqenv *base.RequestEnv, chatID base.ChatID, stage DialogueStage) {
	bot := form.resources.appEnv.Bot
	cleanup := groupCleanupPolicy(GroupDialogue{
		Stage:           stage,
		ChatID:          chatID,
		QuestionIDs:     slices.Clone(form.Dialogue.QuestionIDs),
		UserMessageIDs:  slices.Clone(form.Dialogue.UserMessageIDs),
		CanDeleteOthers: base.CanDeleteMessagesOfOthers(bot, chatID),
		Lang:            reqenv.Lang,
	})
	if len(cleanup.Delete) == 0 {
		return
	}
	// the task of the dialogue nobody continues is found by PendingID to cancel it when the answer comes
	selfdestruct.Schedule(bot, selfdestruct.Task{
		Kind:           selfdestruct.KindDelete,
		ChatID:         chatID,
		MessageID:      cleanup.Delete[0],
		WithMessageIDs: cleanup.Delete[1:],
		Warning:        cleanup.Warning,
	}, cleanup.After)
	if stage == StageWaiting {
		form.Dialogue.PendingID = cleanup.Delete[0]
	}
}

// tracksDialogue is true for non-ephemeral messages in group chats if there is a cleanup policy. Ephemeral messages
// disappear by themselves.
func tracksDialogue(msg *tgbotapi.Message) bool {
	return groupCleanupPolicy != nil && base.IsGroupChat(&msg.Chat) && msg.EphemeralMessageID == 0
}
