package wizard

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/kozalosev/goSadTgBot/base"
	"github.com/kozalosev/goSadTgBot/selfdestruct"
	"github.com/loctools/go-l10n/loc"
	"github.com/samber/lo"
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
	SetGroupCleanupPolicy(CleanupIfAllowed(time.Hour, time.Hour)) // long enough not to delete anything during the test
	t.Cleanup(func() { SetGroupCleanupPolicy(nil) })

	reqenv := &base.RequestEnv{Lang: loc.NewPool("en").GetContext("en")}
	handler := testHandlerWithAction{stateStorage: FakeStorage{}, actionWasRunFlag: &flagContainer{}}
	clearRegisteredDescriptors()
	PopulateWizardDescriptors([]base.MessageHandler{handler})

	wizard := NewWizard(handler, 3)
	wizard.AddEmptyField(TestName, Text)
	wizard.AddEmptyField(TestName2, Text)
	wizard.AddEmptyField(TestName3, Text)
	form := wizard.(*Form)
	bot := &base.FakeBotAPI{DeletionAllowed: true, FirstSentMessageID: 100}
	form.resources = NewEnv(&base.ApplicationEnv{Bot: bot, Ctx: ctx}, FakeStorage{})

	form.ProcessNextField(reqenv, newGroupMessage("/command"))

	assert.Equal(t, int64(TestGroupID), form.ChatID)
	output, ok := bot.GetOutput().([]tgbotapi.Chattable)
	require.True(t, ok, "the question must be sent as a part of the dialogue, not by Reply")
	require.Len(t, output, 1)
	question, ok := output[0].(tgbotapi.MessageConfig)
	require.True(t, ok, "%T", output[0])
	assert.Equal(t, TestID, question.ReplyParameters.MessageID)
	assert.Equal(t, tgbotapi.ForceReply{ForceReply: true, Selective: true}, question.ReplyMarkup)
	assert.Equal(t, dialogue{QuestionIDs: []int{100}, UserMessageIDs: []int{TestID}, PendingID: 100}, form.Dialogue)
}

func TestForm_NoPolicy_RepliesAsIs(t *testing.T) {
	form, bot := newGroupFormForCleanup(t, nil, true)
	form.reply(testReqEnv(), newGroupMessage("/command"), TestPromptDesc, base.NoOpCustomizer)

	assert.Equal(t, []string{TestPromptDesc}, bot.GetOutput())
	assert.Zero(t, form.Dialogue)
}

const (
	testDelay          = 10 * time.Millisecond
	testAbandonedAfter = 30 * time.Millisecond
	firstQuestionID    = 100
)

func testReqEnv() *base.RequestEnv {
	return &base.RequestEnv{Lang: loc.NewPool("en").GetContext("en")}
}

// newGroupFormForCleanup prepares a form in a group with the policy and a worker of the self-destruction polling often.
// The questions get the identifiers from firstQuestionID on.
func newGroupFormForCleanup(t *testing.T, policy GroupCleanupPolicy, deletionAllowed bool) (*Form, *base.FakeBotAPI) {
	SetGroupCleanupPolicy(policy)
	t.Cleanup(func() { SetGroupCleanupPolicy(nil) })
	bot := &base.FakeBotAPI{DeletionAllowed: deletionAllowed, FirstSentMessageID: firstQuestionID}
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
		switch deletion := req.(type) {
		case tgbotapi.DeleteMessageConfig:
			ids = append(ids, deletion.MessageID)
		case tgbotapi.DeleteMessagesConfig:
			ids = append(ids, deletion.MessageIDs...)
		}
	}
	return ids
}

func newAnswer(messageID, questionID int) *tgbotapi.Message {
	answer := newGroupMessage(TestValue)
	answer.MessageID = messageID
	answer.ReplyToMessage = &tgbotapi.Message{MessageID: questionID}
	return answer
}

func TestCleanup_WithoutRightsNothingIsDeleted(t *testing.T) {
	form, bot := newGroupFormForCleanup(t, CleanupIfAllowed(testDelay, testDelay), false)
	form.reply(testReqEnv(), newGroupMessage("/command"), TestPromptDesc, base.NoOpCustomizer)
	form.handleAnswer(newAnswer(TestID+1, firstQuestionID))
	form.completeDialogue(testReqEnv(), newAnswer(TestID+1, firstQuestionID))

	assert.Never(t, func() bool { return len(deletedMessages(bot)) > 0 }, 100*time.Millisecond, 5*time.Millisecond)
}

func TestCleanup_AbandonedDialogueIsDeletedAtOnce(t *testing.T) {
	form, bot := newGroupFormForCleanup(t, CleanupIfAllowed(time.Hour, testAbandonedAfter), true)
	reqenv := testReqEnv()
	form.reply(reqenv, newGroupMessage("/command"), TestPromptDesc, base.NoOpCustomizer)
	answer := newAnswer(TestID+1, firstQuestionID)
	form.handleAnswer(answer)
	form.reply(reqenv, answer, TestPromptDesc, base.NoOpCustomizer)

	assert.Eventually(t, func() bool { return len(deletedMessages(bot)) > 0 }, time.Second, 5*time.Millisecond)
	assert.ElementsMatch(t, []int{TestID, firstQuestionID, TestID + 1, firstQuestionID + 1}, deletedMessages(bot))
	assert.Len(t, lo.Filter(requestsOf(bot), isDeletion), 1, "the dialogue is deleted by one request")
}

func TestCleanup_CompletedDialogueIsDeletedAtOnce(t *testing.T) {
	form, bot := newGroupFormForCleanup(t, CleanupIfAllowed(testDelay, time.Hour), true)
	reqenv := testReqEnv()
	form.reply(reqenv, newGroupMessage("/command"), TestPromptDesc, base.NoOpCustomizer)
	answer := newAnswer(TestID+1, firstQuestionID)
	form.handleAnswer(answer)
	form.completeDialogue(reqenv, answer)

	assert.Eventually(t, func() bool { return len(deletedMessages(bot)) > 0 }, time.Second, 5*time.Millisecond)
	assert.ElementsMatch(t, []int{TestID, firstQuestionID, TestID + 1}, deletedMessages(bot))
	assert.Never(t, func() bool { return len(deletedMessages(bot)) > 3 }, 50*time.Millisecond, 5*time.Millisecond,
		"the task of the abandoned dialogue must be cancelled")
}

func TestCleanup_PolicyGetsTheDialogue(t *testing.T) {
	var dialogues []GroupDialogue
	form, bot := newGroupFormForCleanup(t, func(d GroupDialogue) GroupCleanup {
		dialogues = append(dialogues, d)
		return GroupCleanup{
			Delete:  d.QuestionIDs,
			After:   testDelay,
			Warning: &selfdestruct.Warning{Text: d.Lang.Tr("warning"), MessageID: d.QuestionIDs[len(d.QuestionIDs)-1], Grace: testDelay},
		}
	}, false)
	reqenv := testReqEnv()
	form.reply(reqenv, newGroupMessage("/command"), TestPromptDesc, base.NoOpCustomizer)
	answer := newAnswer(TestID+1, firstQuestionID)
	form.handleAnswer(answer)
	form.completeDialogue(reqenv, answer)

	require.Len(t, dialogues, 2)
	assert.Equal(t, GroupDialogue{
		Stage: StageWaiting, ChatID: TestGroupID, QuestionIDs: []int{firstQuestionID}, UserMessageIDs: []int{TestID}, Lang: reqenv.Lang,
	}, dialogues[0])
	assert.Equal(t, StageCompleted, dialogues[1].Stage)
	assert.Equal(t, []int{TestID, TestID + 1}, dialogues[1].UserMessageIDs)

	assert.Eventually(t, func() bool { return len(deletedMessages(bot)) > 0 }, time.Second, 5*time.Millisecond)
	assert.Equal(t, []int{firstQuestionID}, deletedMessages(bot), "only what the policy has chosen")
	edits := lo.Filter(requestsOf(bot), func(c tgbotapi.Chattable, _ int) bool {
		_, ok := c.(tgbotapi.EditMessageTextConfig)
		return ok
	})
	require.Len(t, edits, 1, "the question is replaced with the warning before the deletion")
	assert.Equal(t, firstQuestionID, edits[0].(tgbotapi.EditMessageTextConfig).MessageID)
}

func requestsOf(bot *base.FakeBotAPI) []tgbotapi.Chattable {
	requests, _ := bot.GetOutput().([]tgbotapi.Chattable)
	return requests
}

func isDeletion(c tgbotapi.Chattable, _ int) bool {
	switch c.(type) {
	case tgbotapi.DeleteMessageConfig, tgbotapi.DeleteMessagesConfig:
		return true
	}
	return false
}

func TestForm_DialogueIsStored(t *testing.T) {
	form := Form{ChatID: TestGroupID, Dialogue: dialogue{QuestionIDs: []int{1}, UserMessageIDs: []int{2}, PendingID: 1}}
	data, err := json.Marshal(form)
	require.NoError(t, err)
	var restored Form
	require.NoError(t, json.Unmarshal(data, &restored))
	assert.Equal(t, form.Dialogue, restored.Dialogue)

	data, err = json.Marshal(Form{})
	require.NoError(t, err)
	assert.NotContains(t, string(data), "dialogue", "no dialogue in the states of the private chats")
}

func TestCancelGroupDialogue(t *testing.T) {
	var stages []DialogueStage
	cleanupIfAllowed := CleanupIfAllowed(testDelay, time.Hour)
	form, bot := newGroupFormForCleanup(t, func(d GroupDialogue) GroupCleanup {
		stages = append(stages, d.Stage)
		return cleanupIfAllowed(d)
	}, true)
	storage := inMemoryStorage{storage: map[int64]Wizard{}}
	reqenv := testReqEnv()
	form.reply(reqenv, newGroupMessage("/command"), TestPromptDesc, base.NoOpCustomizer)
	require.NoError(t, storage.SaveState(TestID, form))

	cancel := newGroupMessage("/cancel")
	cancel.MessageID = TestID + 1
	CancelGroupDialogue(form.resources.appEnv, storage, reqenv, cancel)

	assert.Equal(t, []DialogueStage{StageWaiting, StageCancelled}, stages)
	assert.Eventually(t, func() bool { return len(deletedMessages(bot)) > 0 }, time.Second, 5*time.Millisecond,
		"the dialogue is deleted without waiting for the form to expire")
	assert.ElementsMatch(t, []int{firstQuestionID, TestID}, deletedMessages(bot))
}

func TestCancelGroupDialogue_NoState(t *testing.T) {
	form, bot := newGroupFormForCleanup(t, CleanupIfAllowed(testDelay, testDelay), true)
	storage := inMemoryStorage{storage: map[int64]Wizard{}}

	assert.NotPanics(t, func() {
		CancelGroupDialogue(form.resources.appEnv, storage, testReqEnv(), newGroupMessage("/cancel"))
	})
	assert.Never(t, func() bool { return len(deletedMessages(bot)) > 0 }, 50*time.Millisecond, 5*time.Millisecond)
}
