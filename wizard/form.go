package wizard

import (
	log "log/slog"
	"reflect"
	"strings"
	"time"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/kozalosev/goSadTgBot/base"
	"github.com/kozalosev/goSadTgBot/logconst"
	"github.com/kozalosev/goSadTgBot/metrics"
	"github.com/kozalosev/goSadTgBot/selfdestruct"
)

// localization keys
const (
	InvalidFieldValueErrorTr     = "wizard.errors.field.invalid.value"
	InvalidFieldValueTypeErrorTr = "wizard.errors.field.invalid.type"
	MissingStateErrorTr          = "wizard.errors.state.missing"
)

// Form is an implementation of the [Wizard] interface.
type Form struct {
	Fields     Fields `json:"fields"`
	Index      int    `json:"index"`            // index of the current field
	WizardType string `json:"wizardType"`       // name of the form
	ChatID     int64  `json:"chatID,omitempty"` // chat where the form is filled; 0 for the states saved by older versions

	resources  *Env
	descriptor *FormDescriptor
}

func (form *Form) AddEmptyField(name string, fieldType FieldType) {
	if form.descriptor == nil {
		panic("No descriptor was set for the form: " + form.WizardType)
	}
	fieldDesc := form.descriptor.findFieldDescriptor(name)
	if fieldDesc == nil {
		panic("No descriptor was set for the field: " + name)
	}
	field := &Field{
		Name:       name,
		Type:       fieldType,
		Form:       form,
		descriptor: fieldDesc,
	}
	form.Fields = append(form.Fields, field)
}

func (form *Form) AddPrefilledField(name string, value Value) {
	field := &Field{Name: name, Data: value, Form: form}
	form.Fields = append(form.Fields, field)
}

func (form *Form) AddPrefilledText(name string, value string) {
	field := &Field{Name: name, Data: Txt{Value: value}, Type: Text, Form: form}
	form.Fields = append(form.Fields, field)
}

func (form *Form) AddPrefilledAutoField(name string, msg *tgbotapi.Message) {
	form.AddEmptyField(name, Auto)
	field := form.Fields.FindField(name)
	field.fillFrom(msg)
}

func (form *Form) AllRequiredFieldsFilled() bool {
	if len(form.Fields) < cap(form.Fields) {
		return false
	}
	for _, field := range form.Fields {
		if field.Data == nil && !shouldBeSkipped(field, form) {
			return false
		}
	}
	return true
}

func (form *Form) ProcessNextField(reqenv *base.RequestEnv, msg *tgbotapi.Message) {
	maxIndex := len(form.Fields) - 1
start:
	if form.Index > maxIndex {
		form.doAction(reqenv, msg)
		return
	}

	if form.Fields[form.Index].Data != nil || shouldBeSkipped(form.Fields[form.Index], form) {
		form.Index++
		goto start
	}

	currentField := form.Fields[form.Index]
	if currentField.WasRequested {
		value := currentField.extractor(msg)
		form.deleteAnswer(msg)
		if value == nil {
			text := reqenv.Lang.Tr(InvalidFieldValueTypeErrorTr) + reqenv.Lang.Tr(string(currentField.Type))
			form.reply(msg, text, base.NoOpCustomizer)
			return
		} else if err := currentField.validate(reqenv, msg); err != nil {
			text := reqenv.Lang.Tr(InvalidFieldValueErrorTr) + reqenv.Lang.Tr(err.Error())
			form.reply(msg, text, base.MarkdownCustomizer)
			return
		}
		currentField.Data = value
		form.Index++
		goto start
	}
	currentField.askUser(reqenv, msg)
	currentField.WasRequested = true
	form.ChatID = msg.Chat.ID

	if err := form.resources.stateStorage.SaveState(msg.From.ID, form); err != nil {
		log.Error("Failed to save the state of a wizard",
			logconst.FieldObject, "Form",
			logconst.FieldMethod, "ProcessNextField",
			logconst.FieldCalledObject, "StateStorage",
			logconst.FieldCalledMethod, "SaveState",
			logconst.FieldError, err)
	}
}

func (form *Form) doAction(reqenv *base.RequestEnv, msg *tgbotapi.Message) {
	if form.descriptor.action == nil {
		form.resources.appEnv.Bot.Reply(msg, reqenv.Lang.Tr(MissingStateErrorTr))
		return
	}
	metrics.IncWizardCompletedCounter(form.WizardType)
	form.descriptor.action(reqenv, msg, form.Fields)
}

// BelongsTo tells if the form may be continued by a message from the chat. The forms saved before the chat was
// stored belong to any chat.
func (form *Form) BelongsTo(chatID int64) bool {
	return form.ChatID == 0 || form.ChatID == chatID
}

// PopulateRestored sets non-storable fields of the form restored from [StateStorage].
func (form *Form) PopulateRestored(msg *tgbotapi.Message, resources *Env) {
	form.resources = resources
	form.Fields[form.Index].restoreExtractor(msg)
	form.descriptor = findFormDescriptor(form.WizardType)
	for _, field := range form.Fields {
		field.Form = form
		field.descriptor = form.descriptor.findFieldDescriptor(field.Name)
	}
}

// NewWizard is a constructor for [Wizard].
// The fields parameter is used only for array initialization.
func NewWizard(handler WizardMessageHandler, fields int) Wizard {
	wizardName := getWizardName(handler)
	return &Form{
		resources:  handler.GetWizardEnv(),
		Fields:     make(Fields, 0, fields),
		WizardType: wizardName,
		descriptor: findFormDescriptor(wizardName),
	}
}

func NewEnv(appEnv *base.ApplicationEnv, stateStorage StateStorage) *Env {
	return &Env{
		appEnv:       appEnv,
		stateStorage: stateStorage,
	}
}

// selfDestructionDelayInGroups is how long the questions of the wizard and the answers to them live in group chats.
var selfDestructionDelayInGroups time.Duration

// SetSelfDestructionDelayInGroups makes the questions of the wizards and the answers of the users to them disappear
// from group chats after the delay, so that filling a form doesn't litter the chat. Zero disables it. The answers of
// the users are deleted only if the bot has the right to delete messages in the group.
func SetSelfDestructionDelayInGroups(delay time.Duration) {
	selfDestructionDelayInGroups = delay
}

// reply sends the text of the wizard in reply to the message: with ForceReply and self-destroying in group chats.
func (form *Form) reply(msg *tgbotapi.Message, text string, customizer base.MessageCustomizer) {
	customizer = replyCustomizer(msg, customizer)
	if !shouldSelfDestruct(msg) {
		form.resources.appEnv.Bot.ReplyWithMessageCustomizer(msg, text, customizer)
		return
	}
	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ReplyParameters.MessageID = msg.MessageID
	customizer(&reply)
	selfdestruct.SendAndDelete(form.resources.appEnv.Bot, reply, selfDestructionDelayInGroups)
}

// deleteAnswer deletes the answer of the user to a question of the wizard in group chats.
func (form *Form) deleteAnswer(msg *tgbotapi.Message) {
	if shouldSelfDestruct(msg) {
		selfdestruct.DeleteAfter(form.resources.appEnv.Bot, msg.Chat.ID, msg.MessageID, selfDestructionDelayInGroups)
	}
}

// shouldSelfDestruct is true for non-ephemeral messages in group chats if the self-destruction is enabled.
// Ephemeral messages disappear by themselves.
func shouldSelfDestruct(msg *tgbotapi.Message) bool {
	return selfDestructionDelayInGroups > 0 && base.IsGroupChat(&msg.Chat) && msg.EphemeralMessageID == 0
}

// replyCustomizer adds ForceReply to the replies of the wizard in group chats. The client of the user opens the reply
// interface, so the answer is a reply to the bot, which is delivered even in privacy mode. Selective limits it to the
// user the bot replies to. Ephemeral replies are left as they are: the user can't answer them anyway.
func replyCustomizer(msg *tgbotapi.Message, customizer base.MessageCustomizer) base.MessageCustomizer {
	if !base.IsGroupChat(&msg.Chat) || msg.EphemeralMessageID != 0 {
		return customizer
	}
	return func(msgConfig *tgbotapi.MessageConfig) {
		customizer(msgConfig)
		msgConfig.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true, Selective: true}
	}
}

func shouldBeSkipped(field *Field, form *Form) bool {
	skipPredicate := field.descriptor.SkipIf
	if skipPredicate == nil {
		return false
	}
	return skipPredicate.ShouldBeSkipped(form)
}

// SomeHandler -> SomeWizard
func getWizardName(handler WizardMessageHandler) string {
	t := reflect.TypeOf(handler)
	var handlerName string
	if t.Kind() == reflect.Pointer {
		handlerName = reflect.Indirect(reflect.ValueOf(handler)).Type().Name()
	} else {
		handlerName = t.Name()
	}
	return strings.TrimSuffix(handlerName, "Handler") + "Wizard"
}
