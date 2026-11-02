package wizard

import (
	"context"
	"testing"
	"time"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/kozalosev/goSadTgBot/base"
	"github.com/kozalosev/goSadTgBot/selfdestruct"
	"github.com/loctools/go-l10n/loc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const TestGroupID = -100123

func newGroupMessage(text string) *tgbotapi.Message {
	return &tgbotapi.Message{
		Text:      text,
		Chat:      tgbotapi.Chat{ID: TestGroupID, Type: "supergroup"},
		MessageID: TestID,
		From:      &tgbotapi.User{ID: TestID},
	}
}

func TestReplyCustomizer(t *testing.T) {
	apply := func(msg *tgbotapi.Message, customizer base.MessageCustomizer) tgbotapi.MessageConfig {
		cfg := tgbotapi.NewMessage(msg.Chat.ID, "text")
		replyCustomizer(msg, customizer)(&cfg)
		return cfg
	}

	private := &tgbotapi.Message{Chat: tgbotapi.Chat{ID: TestID, Type: "private"}}
	assert.Nil(t, apply(private, base.NoOpCustomizer).ReplyMarkup)

	group := newGroupMessage("")
	cfg := apply(group, base.MarkdownCustomizer)
	assert.Equal(t, tgbotapi.ForceReply{ForceReply: true, Selective: true}, cfg.ReplyMarkup)
	assert.Equal(t, tgbotapi.ModeMarkdown, cfg.ParseMode, "the original customizer must be applied too")

	ephemeral := newGroupMessage("")
	ephemeral.EphemeralMessageID = TestID
	assert.Nil(t, apply(ephemeral, base.NoOpCustomizer).ReplyMarkup)
}

func TestForm_BelongsTo(t *testing.T) {
	assert.True(t, (&Form{}).BelongsTo(TestGroupID), "the forms of older versions belong to any chat")
	assert.True(t, (&Form{ChatID: TestGroupID}).BelongsTo(TestGroupID))
	assert.False(t, (&Form{ChatID: TestID}).BelongsTo(TestGroupID))
}

func TestField_validate_NoReplyKeyboardInGroups(t *testing.T) {
	reqenv := &base.RequestEnv{Lang: loc.NewPool("en").GetContext("en")}
	f := Field{
		descriptor: &FieldDescriptor{
			ReplyKeyboardBuilder: func(*base.RequestEnv, *tgbotapi.Message) []string {
				return []string{TestValue}
			},
		},
	}

	// the keyboard isn't shown in groups, so any answer is allowed
	assert.NoError(t, f.validate(reqenv, newGroupMessage(TestValue+"2")))
}

func TestForm_ProcessNextField_InGroup(t *testing.T) {
	SetSelfDestructionDelayInGroups(time.Hour) // long enough not to delete anything during the test
	t.Cleanup(func() { SetSelfDestructionDelayInGroups(0) })

	reqenv := &base.RequestEnv{Lang: loc.NewPool("en").GetContext("en")}
	handler := testHandlerWithAction{stateStorage: FakeStorage{}, actionWasRunFlag: &flagContainer{}}
	clearRegisteredDescriptors()
	PopulateWizardDescriptors([]base.MessageHandler{handler})

	wizard := NewWizard(handler, 3)
	wizard.AddEmptyField(TestName, Text)
	wizard.AddEmptyField(TestName2, Text)
	wizard.AddEmptyField(TestName3, Text)
	form := wizard.(*Form)
	bot := &base.FakeBotAPI{}
	form.resources = NewEnv(&base.ApplicationEnv{Bot: bot, Ctx: ctx}, FakeStorage{})

	form.ProcessNextField(reqenv, newGroupMessage("/command"))

	assert.Equal(t, int64(TestGroupID), form.ChatID)
	output, ok := bot.GetOutput().([]tgbotapi.Chattable)
	require.True(t, ok, "the question must be sent as a self-destroying message, not by Reply")
	require.Len(t, output, 1)
	question, ok := output[0].(tgbotapi.MessageConfig)
	require.True(t, ok, "%T", output[0])
	assert.Equal(t, TestID, question.ReplyParameters.MessageID)
	assert.Equal(t, tgbotapi.ForceReply{ForceReply: true, Selective: true}, question.ReplyMarkup)
}

// newGroupFormForCleanup prepares a form in a group with short timers of the cleanup.
func newGroupFormForCleanup(t *testing.T, deletionAllowed bool) (*Form, *base.FakeBotAPI) {
	SetSelfDestructionDelayInGroups(10 * time.Millisecond)
	SetUnansweredQuestionLifetimeInGroups(10 * time.Millisecond)
	t.Cleanup(func() {
		SetSelfDestructionDelayInGroups(0)
		SetUnansweredQuestionLifetimeInGroups(0)
	})
	bot := &base.FakeBotAPI{DeletionAllowed: deletionAllowed}
	workerCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	selfdestruct.Start(workerCtx, bot, selfdestruct.NewMemoryQueue(), time.Millisecond)
	form := &Form{resources: NewEnv(&base.ApplicationEnv{Bot: bot, Ctx: ctx}, FakeStorage{})}
	return form, bot
}

// deletedMessages returns the identifiers of the messages the bot has requested to delete.
func deletedMessages(bot *base.FakeBotAPI) []int {
	requests, _ := bot.GetOutput().([]tgbotapi.Chattable)
	var ids []int
	for _, req := range requests {
		if deletion, ok := req.(tgbotapi.DeleteMessageConfig); ok {
			ids = append(ids, deletion.MessageID)
		}
	}
	return ids
}

// The fake bot returns 0 as the identifier of every sent message, so the question is the message 0.
func newAnswer() *tgbotapi.Message {
	answer := newGroupMessage(TestValue)
	answer.MessageID = TestID + 1
	answer.ReplyToMessage = &tgbotapi.Message{MessageID: 0}
	return answer
}

func TestCleanup_UnansweredQuestionIsDeleted(t *testing.T) {
	form, bot := newGroupFormForCleanup(t, false)
	form.reply(newGroupMessage("/command"), TestPromptDesc, base.NoOpCustomizer)

	assert.Eventually(t, func() bool { return len(deletedMessages(bot)) == 1 }, time.Second, 5*time.Millisecond)
	assert.Equal(t, []int{0}, deletedMessages(bot))
}

func TestCleanup_AnsweredWithoutRightsIsKept(t *testing.T) {
	form, bot := newGroupFormForCleanup(t, false)
	form.reply(newGroupMessage("/command"), TestPromptDesc, base.NoOpCustomizer)
	form.handleAnswer(newAnswer())

	assert.Never(t, func() bool { return len(deletedMessages(bot)) > 0 }, 100*time.Millisecond, 5*time.Millisecond)
}

func TestCleanup_AnsweredWithRightsIsDeleted(t *testing.T) {
	form, bot := newGroupFormForCleanup(t, true)
	form.reply(newGroupMessage("/command"), TestPromptDesc, base.NoOpCustomizer)
	form.handleAnswer(newAnswer())

	// the answer, the question and the command the question was asked in reply to; the timer of the unanswered
	// question deletes nothing, since the question has been answered
	assert.Eventually(t, func() bool { return len(deletedMessages(bot)) == 3 }, time.Second, 5*time.Millisecond)
	assert.ElementsMatch(t, []int{TestID + 1, 0, TestID}, deletedMessages(bot))
	assert.Never(t, func() bool { return len(deletedMessages(bot)) > 3 }, 50*time.Millisecond, 5*time.Millisecond)
}
