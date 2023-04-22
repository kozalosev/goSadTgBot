package wizard

import (
	"encoding/json"
	"errors"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/kozalosev/goSadTgBot/base"
	"github.com/loctools/go-l10n/loc"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFields_FindField(t *testing.T) {
	tnField := Field{Name: TestName}
	tn2Field := Field{Name: TestName2}
	fields := Fields{&tnField, &tn2Field}

	res := fields.FindField(TestName)

	assert.Equal(t, &tnField, res)
	assert.NotEqual(t, &tn2Field, res)
}

func TestFields_FindField_MultipleItems(t *testing.T) {
	tnField := Field{Name: TestName}
	tn2Field := Field{Name: TestName}
	tn3Field := Field{Name: TestName2}
	fields := Fields{&tnField, &tn2Field, &tn3Field}

	res := fields.FindField(TestName)

	assert.Equal(t, &tnField, res)
	assert.NotSame(t, &tn2Field, res)
	assert.NotEqual(t, &tn3Field, res)
}

func TestFields_FindField_NotExistentField(t *testing.T) {
	tnField := Field{Name: TestName}
	fields := Fields{&tnField}

	res := fields.FindField(TestName2)

	assert.Nil(t, res)
}

func TestFieldMarshalling(t *testing.T) {
	field := Field{
		Name:         TestName,
		Data:         TestValue,
		WasRequested: true,
		Type:         Text,
	}

	jsonBytes, err := json.Marshal(field)
	assert.NoError(t, err)
	jsn := string(jsonBytes)

	entities := []string{
		TestName, TestValue, "true", string(Text),
	}
	for _, e := range entities {
		assert.Contains(t, jsn, e)
	}

	var restoredField Field
	err = json.Unmarshal(jsonBytes, &restoredField)
	assert.NoError(t, err)
	assert.Equal(t, field, restoredField)
}

func TestField_validate(t *testing.T) {
	reqenv := &base.RequestEnv{
		Lang: loc.NewPool("en").GetContext("en"),
	}
	expectedError := errors.New("validation failed")

	validMsg := &tgbotapi.Message{Text: TestValue}
	invalidMsg := &tgbotapi.Message{Text: TestValue + "2"}
	fSimpleValidation := Field{
		descriptor: &FieldDescriptor{
			Validator: func(msg *tgbotapi.Message, _ *loc.Context) error {
				if msg.Text != TestValue {
					return expectedError
				}
				return nil
			},
		},
	}

	assert.NoError(t, fSimpleValidation.validate(reqenv, validMsg))
	assert.Error(t, expectedError, fSimpleValidation.validate(reqenv, invalidMsg))

	fReplyKeyboard := Field{
		descriptor: &FieldDescriptor{
			ReplyKeyboardBuilder: func(*base.RequestEnv, *tgbotapi.Message) []string {
				return []string{TestValue}
			},
		},
	}

	assert.NoError(t, fReplyKeyboard.validate(reqenv, validMsg))
	assert.Error(t, expectedError, fReplyKeyboard.validate(reqenv, invalidMsg))

	fInlineKeyboard := Field{
		descriptor: &FieldDescriptor{
			InlineKeyboardAnswers: []string{TestValue},
		},
	}

	assert.NoError(t, fInlineKeyboard.validate(reqenv, validMsg))
	assert.Error(t, expectedError, fInlineKeyboard.validate(reqenv, invalidMsg))
}
