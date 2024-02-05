package wizard

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/kozalosev/goSadTgBot/base"
)

// FormAction is a function that will be executed when all of required fields are filled in.
type FormAction func(reqenv *base.RequestEnv, msg *tgbotapi.Message, fields Fields)

// Wizard is another name for a form. Create a Wizard instance in your handler with [NewWizard] function and add fields to it.
type Wizard interface {
	// AddEmptyField creates a new empty field of fieldType type.
	AddEmptyField(name string, fieldType FieldType)
	// AddPrefilledField creates a field with already filled in value. It may be useful when some arguments were passed in a command immediately.
	AddPrefilledField(name string, value interface{})
	// AddPrefilledAutoField is supposed for a special case when you want to fill the field with a value of unknown type, determined by the content of some message (ReplyToMessage, for example).
	AddPrefilledAutoField(name string, msg *tgbotapi.Message)
	// AllRequiredFieldsFilled returns true if all required fields are already prefilled.
	AllRequiredFieldsFilled() bool
	// ProcessNextField runs the form machinery. Call this method when all fields were created.
	ProcessNextField(reqenv *base.RequestEnv, msg *tgbotapi.Message)
}

// WizardMessageHandler is an extended interface of [base.MessageHandler] which your handler must implement if you want to use this package facilities
//
//goland:noinspection GoNameStartsWithPackageName
type WizardMessageHandler interface {
	base.MessageHandler

	// GetWizardEnv should return the application environment and an implementation of the storage (an instance of [RedisStateStorage] for example).
	GetWizardEnv() *Env
	// GetWizardDescriptor should return the description of all non-storable parameters of your form.
	GetWizardDescriptor() *FormDescriptor
}

//goland:noinspection GoNameStartsWithPackageName
type Env struct {
	appEnv       *base.ApplicationEnv
	stateStorage StateStorage
}
