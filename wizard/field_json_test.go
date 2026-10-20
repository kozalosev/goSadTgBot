package wizard

import (
	"encoding/json"
	"testing"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/stretchr/testify/assert"
)

// restore simulates a round trip of the field through a StateStorage.
func restore(t *testing.T, f *Field) *Field {
	raw, err := json.Marshal(f)
	assert.NoError(t, err)
	var restored Field
	assert.NoError(t, json.Unmarshal(raw, &restored))
	return &restored
}

func TestField_JSONRoundTrip(t *testing.T) {
	values := []Value{
		Txt{Value: TestValue, Entities: []tgbotapi.MessageEntity{{Type: "bold", Length: 4}}},
		File{ID: TestFileID, UniqueID: TestFileUniqueID, Caption: "caption"},
		LocData{Latitude: 1.5, Longitude: 2.5},
		Int(42),
		Bool(true),
		nil,
	}
	for _, value := range values {
		// the field type is not set intentionally to check that dataType is used
		field := &Field{Name: TestName, Data: value, WasRequested: true}
		assert.Equal(t, field, restore(t, field))
	}
}

func TestField_UnmarshalJSON_Legacy(t *testing.T) {
	var txtField Field
	assert.NoError(t, json.Unmarshal([]byte(`{"name":"n","data":{"Value":"v","Entities":null},"type":"text"}`), &txtField))
	assert.Equal(t, Txt{Value: "v"}, txtField.Data)

	var locField Field
	assert.NoError(t, json.Unmarshal([]byte(`{"name":"n","data":{"Latitude":1,"Longitude":2},"type":"location"}`), &locField))
	assert.Equal(t, LocData{Latitude: 1, Longitude: 2}, locField.Data)

	var unknownField Field
	err := json.Unmarshal([]byte(`{"name":"n","data":42,"type":""}`), &unknownField)
	assert.ErrorIs(t, err, errUnknownValueType)
}

func TestFieldMarshalling(t *testing.T) {
	field := Field{
		Name:         TestName,
		Data:         Txt{Value: TestValue},
		WasRequested: true,
		Type:         Text,
	}

	jsonBytes, err := json.Marshal(&field)
	assert.NoError(t, err)
	jsn := string(jsonBytes)

	entities := []string{
		TestName, TestValue, "true", string(Text), `"dataType":"txt"`,
	}
	for _, e := range entities {
		assert.Contains(t, jsn, e)
	}

	var restoredField Field
	err = json.Unmarshal(jsonBytes, &restoredField)
	assert.NoError(t, err)
	assert.Equal(t, field, restoredField)
}
