package wizard

import (
	"testing"
	"time"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/kozalosev/goSadTgBot/base"
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
