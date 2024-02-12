package wizard

import (
	"errors"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/kozalosev/goSadTgBot/base"
	"github.com/kozalosev/goSadTgBot/logconst"
	"github.com/loctools/go-l10n/loc"
	log "github.com/sirupsen/logrus"
	"github.com/thoas/go-funk"
	"golang.org/x/exp/slices"
)

const ValidErrNotInListTr = "errors.validation.option.not.in.list"

// FieldValidator is, obviously, a validation function. Returned error will be sent to the user and may be a key for
// the translation mechanism.
type FieldValidator func(msg *tgbotapi.Message, lc *loc.Context) error

type Fields []*Field
type FieldType string

const (
	Auto      FieldType = "<auto>" // will be automatically resolved from the type of message sent by the user
	Text      FieldType = "text"
	Sticker   FieldType = "sticker"
	Image     FieldType = "image"
	Voice     FieldType = "voice"
	Audio     FieldType = "audio"
	Video     FieldType = "video"
	VideoNote FieldType = "video_note"
	Gif       FieldType = "gif"
	Document  FieldType = "document"
	Location  FieldType = "location"
)

type Field struct {
	Name         string      `json:"name"`
	Data         interface{} `json:"data,omitempty"` // the value
	WasRequested bool        `json:"wasRequested"`
	Type         FieldType   `json:"type"`

	Form *Form `json:"-"`

	extractor  fieldExtractor
	descriptor *FieldDescriptor
}

// FindField is useful in a [FormAction] function to get values of the fields.
func (fs Fields) FindField(name string) *Field {
	found := funk.Filter(fs, func(f *Field) bool { return f.Name == name }).([]*Field)
	if len(found) == 0 {
		return nil
	}
	if len(found) > 1 {
		log.WithField(logconst.FieldObject, "Fields").
			WithField(logconst.FieldCalledMethod, "FindField").
			Warning("More than needed: ", found)
	}
	return found[0]
}

// Send a prompt message to the user.
func (f *Field) askUser(reqenv *base.RequestEnv, msg *tgbotapi.Message) {
	promptDescription := reqenv.Lang.Tr(f.descriptor.promptDescription)
	var inlineKeyboardAnswers []string
	if len(f.descriptor.InlineKeyboardAnswers) > 0 {
		inlineKeyboardAnswers = f.descriptor.InlineKeyboardAnswers
	} else if f.descriptor.InlineKeyboardBuilder != nil {
		inlineKeyboardAnswers = f.descriptor.InlineKeyboardBuilder(reqenv, msg, f.Form)
	}
	if len(inlineKeyboardAnswers) > 0 {
		inlineAnswers := funk.Map(inlineKeyboardAnswers, func(s string) tgbotapi.InlineKeyboardButton {
			btn := tgbotapi.InlineKeyboardButton{Text: reqenv.Lang.Tr(s)}
			if customizer, ok := f.descriptor.inlineButtonCustomizers[s]; ok {
				customizer(&btn, f)
			} else {
				data := CallbackDataFieldPrefix + f.Name + callbackDataSep + s
				btn.CallbackData = &data
			}
			return btn
		}).([]tgbotapi.InlineKeyboardButton)
		f.Form.resources.appEnv.Bot.ReplyWithInlineKeyboard(msg, promptDescription, inlineAnswers)
	} else if f.descriptor.ReplyKeyboardBuilder != nil {
		f.Form.resources.appEnv.Bot.ReplyWithKeyboard(msg, promptDescription, f.descriptor.ReplyKeyboardBuilder(reqenv, msg))
	} else {
		f.Form.resources.appEnv.Bot.Reply(msg, promptDescription)
	}
}

func (f *Field) validate(reqenv *base.RequestEnv, msg *tgbotapi.Message) error {
	if !f.descriptor.DisableKeyboardValidation {
		notInReplyKeyboardOptionsIfExists := f.descriptor.ReplyKeyboardBuilder != nil &&
			!slices.Contains(f.descriptor.ReplyKeyboardBuilder(reqenv, msg), msg.Text)
		notInInlineKeyboardOptionsIfExists := len(f.descriptor.InlineKeyboardAnswers) > 0 &&
			!slices.Contains(f.descriptor.InlineKeyboardAnswers, msg.Text) &&
			!slices.Contains(translateList(f.descriptor.InlineKeyboardAnswers, reqenv.Lang), msg.Text)
		if notInReplyKeyboardOptionsIfExists || notInInlineKeyboardOptionsIfExists {
			return errors.New(ValidErrNotInListTr)
		}
	}
	if f.descriptor.Validator == nil {
		return nil
	}
	return f.descriptor.Validator(msg, reqenv.Lang)
}

func translateList(arr []string, lc *loc.Context) []string {
	return funk.Map(arr, func(s string) string {
		return lc.Tr(s)
	}).([]string)
}
