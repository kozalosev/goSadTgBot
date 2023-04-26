package wizard

import (
	"errors"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/kozalosev/goSadTgBot/base"
	"github.com/loctools/go-l10n/loc"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestForm_AddEmptyField(t *testing.T) {
	wizard := NewWizard(testHandler{}, 1)
	wizard.AddEmptyField(TestName, Text)

	form := wizard.(*Form)
	resField := form.Fields[0]

	assert.Equal(t, TestName, resField.Name)
	assert.Equal(t, Text, resField.Type)
	assert.Nil(t, resField.Data)
}

func TestForm_AddPrefilledField(t *testing.T) {
	wizard := NewWizard(testHandler{}, 1)
	wizard.AddPrefilledField(TestName, TestValue)

	form := wizard.(*Form)
	resField := form.Fields[0]

	assert.Equal(t, TestName, resField.Name)
	assert.Equal(t, TestValue, resField.Data)
	assert.Empty(t, resField.Type)
}

func TestRestorationOfFunctions(t *testing.T) {
	wizard := NewWizard(testHandler{}, 1)
	wizard.AddEmptyField(TestName, Text)

	form := wizard.(*Form)
	form.PopulateRestored(&tgbotapi.Message{}, nil)

	assert.Equal(t, getFuncPtr(tAction), getFuncPtr(form.descriptor.action))
	assert.Equal(t, getFuncPtr(textExtractor), getFuncPtr(form.Fields[form.Index].extractor))
	assert.Equal(t, TestPromptDesc, form.Fields[form.Index].descriptor.promptDescription)
}

func TestForm_ProcessNextField(t *testing.T) {
	msg := &tgbotapi.Message{
		Text:      "not" + TestValue,
		Chat:      &tgbotapi.Chat{ID: TestID},
		MessageID: TestID,
		From:      &tgbotapi.User{ID: TestID},
	}
	reqenv := &base.RequestEnv{
		Lang: loc.NewPool("en").GetContext("en"),
	}

	actionFlagCont := &flagContainer{}
	handler := testHandlerWithAction{stateStorage: FakeStorage{}, actionWasRunFlag: actionFlagCont}
	clearRegisteredDescriptors()
	PopulateWizardDescriptors([]base.MessageHandler{handler})

	wizard := NewWizard(handler, 3)
	wizard.AddPrefilledField(TestName, TestValue)
	wizard.AddEmptyField(TestName2, Text)
	wizard.AddEmptyField(TestName3, Text)
	form := wizard.(*Form)

	assert.Equal(t, 0, form.Index)
	assert.False(t, form.Fields[0].WasRequested)
	assert.Equal(t, TestValue, form.Fields[0].Data)
	assert.False(t, form.Fields[1].WasRequested)
	assert.False(t, actionFlagCont.flag)

	form.ProcessNextField(reqenv, msg)

	assert.Equal(t, 1, form.Index)
	assert.False(t, form.Fields[0].WasRequested)
	assert.True(t, form.Fields[1].WasRequested)
	assert.Nil(t, form.Fields[1].Data)
	assert.False(t, actionFlagCont.flag)

	form.Fields[1].extractor = textExtractor
	form.ProcessNextField(reqenv, msg) // validation must fail

	assert.Equal(t, 1, form.Index)
	assert.Nil(t, form.Fields[1].Data)
	assert.False(t, actionFlagCont.flag)

	msg.Text = TestValue
	form.ProcessNextField(reqenv, msg)

	assert.Equal(t, 3, form.Index)
	assert.Equal(t, TestValue, form.Fields[1].Data)
	assert.True(t, actionFlagCont.flag)
}

func tAction(_ *base.RequestEnv, _ *tgbotapi.Message, fields Fields) {
	f3 := fields.FindField(TestName3)
	if f3.Data != nil {
		panic(TestName3 + " must be skipped and equals to nil!") // assertion without access to `t *testing.T`
	}
}

type testHandler struct{}

func (testHandler) CanHandle(*base.RequestEnv, *tgbotapi.Message) bool { return false }
func (testHandler) Handle(*base.RequestEnv, *tgbotapi.Message)         {}
func (testHandler) GetWizardAction() FormAction                        { return tAction }

func (testHandler) GetWizardEnv() *Env {
	return NewEnv(&base.ApplicationEnv{
		Bot: &base.FakeBotAPI{},
		Ctx: ctx,
	}, nil)
}

func (h testHandler) GetWizardDescriptor() *FormDescriptor {
	desc := NewWizardDescriptor(tAction)
	desc.AddField(TestName, TestPromptDesc)
	return desc
}

type testHandler2 struct {
	testHandler
}

type flagContainer struct {
	flag bool
}
type testHandlerWithAction struct {
	testHandler

	stateStorage     StateStorage
	actionWasRunFlag *flagContainer
}

func (handler testHandlerWithAction) GetWizardDescriptor() *FormDescriptor {
	desc := NewWizardDescriptor(func(*base.RequestEnv, *tgbotapi.Message, Fields) {
		handler.actionWasRunFlag.flag = true
	})
	desc.AddField(TestName, TestPromptDesc)
	f2 := desc.AddField(TestName2, TestPromptDesc)
	f2.Validator = func(msg *tgbotapi.Message, _ *loc.Context) error {
		if msg.Text != TestValue {
			return errors.New("not " + TestValue)
		}
		return nil
	}
	f3 := desc.AddField(TestName3, TestPromptDesc)
	f3.SkipIf = &SkipOnFieldValue{
		Name:  TestName2,
		Value: TestValue,
	}
	return desc
}

func (handler testHandlerWithAction) GetWizardEnv() *Env {
	return NewEnv(&base.ApplicationEnv{
		Bot: &base.FakeBotAPI{},
		Ctx: ctx,
	}, handler.stateStorage)
}
