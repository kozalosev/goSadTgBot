package wizard

import (
	"errors"
	"testing"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/kozalosev/goSadTgBot/base"
	"github.com/loctools/go-l10n/loc"
	"github.com/stretchr/testify/assert"
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

func TestGetData(t *testing.T) {
	const emptyName = "empty"
	fields := Fields{
		&Field{Name: TestName, Data: Txt{Value: TestValue}},
		restore(t, &Field{Name: TestName2, Data: Int(42)}),
		&Field{Name: emptyName},
	}

	t.Run("found", func(t *testing.T) {
		val, ok := GetData[Txt](fields, TestName)
		assert.True(t, ok)
		assert.Equal(t, Txt{Value: TestValue}, val)
	})
	t.Run("restored", func(t *testing.T) {
		val, ok := GetData[Int](fields, TestName2)
		assert.True(t, ok)
		assert.Equal(t, Int(42), val)
	})
	t.Run("type mismatch", func(t *testing.T) {
		_, ok := GetData[Bool](fields, TestName)
		assert.False(t, ok)
	})
	t.Run("empty", func(t *testing.T) {
		_, ok := GetData[Txt](fields, emptyName)
		assert.False(t, ok)
	})
	t.Run("not found", func(t *testing.T) {
		_, ok := GetData[Txt](fields, "missing")
		assert.False(t, ok)
	})
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
