package wizard

import (
	"encoding/json"
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

// restore simulates a round trip of the field through a StateStorage.
func restore(t *testing.T, f *Field) *Field {
	raw, err := json.Marshal(f)
	assert.NoError(t, err)
	var restored Field
	assert.NoError(t, json.Unmarshal(raw, &restored))
	return &restored
}

func TestGetData(t *testing.T) {
	txt := Txt{Value: TestValue}
	file := File{ID: "id", UniqueID: "uid", Caption: "caption"}
	loc := LocData{Latitude: 1.5, Longitude: 2.5}

	t.Run("direct", func(t *testing.T) {
		val, ok := GetData[Txt](&Field{Data: txt})
		assert.True(t, ok)
		assert.Equal(t, txt, val)
	})
	t.Run("restored struct", func(t *testing.T) {
		restoredTxt := restore(t, &Field{Data: txt})
		assert.IsType(t, map[string]any{}, restoredTxt.Data)
		val, ok := GetData[Txt](restoredTxt)
		assert.True(t, ok)
		assert.Equal(t, txt, val)

		fileVal, ok := GetData[File](restore(t, &Field{Data: file}))
		assert.True(t, ok)
		assert.Equal(t, file, fileVal)

		locVal, ok := GetData[LocData](restore(t, &Field{Data: loc}))
		assert.True(t, ok)
		assert.Equal(t, loc, locVal)
	})
	t.Run("restored number", func(t *testing.T) {
		val, ok := GetData[int](restore(t, &Field{Data: 42}))
		assert.True(t, ok)
		assert.Equal(t, 42, val)
	})
	t.Run("type mismatch", func(t *testing.T) {
		_, ok := GetData[Txt](&Field{Data: true})
		assert.False(t, ok)
		_, ok = GetData[int](&Field{Data: 1.5})
		assert.False(t, ok)
		_, ok = GetData[File](restore(t, &Field{Data: loc}))
		assert.False(t, ok)
	})
	t.Run("empty", func(t *testing.T) {
		_, ok := GetData[Txt](nil)
		assert.False(t, ok)
		_, ok = GetData[Txt](&Field{})
		assert.False(t, ok)
	})
}

func TestFieldData(t *testing.T) {
	fields := Fields{&Field{Name: TestName, Data: Txt{Value: TestValue}}}

	val, ok := FieldData[Txt](fields, TestName)
	assert.True(t, ok)
	assert.Equal(t, TestValue, val.Value)

	_, ok = FieldData[Txt](fields, TestName2)
	assert.False(t, ok)
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
