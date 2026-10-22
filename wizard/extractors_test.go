package wizard

import (
	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/stretchr/testify/assert"

	"reflect"
	"testing"
)

func TestRestoreExtractor(t *testing.T) {
	audioMsg := &tgbotapi.Message{
		Audio: &tgbotapi.Audio{FileID: TestFileID, FileUniqueID: TestFileUniqueID},
	}

	f := Field{Type: Image}
	f.restoreExtractor(audioMsg)
	assert.Equal(t, getFuncPtr(imageExtractor), getFuncPtr(f.extractor))

	f = Field{Type: Auto}
	f.restoreExtractor(audioMsg)
	assert.Equal(t, getFuncPtr(audioExtractor), getFuncPtr(f.extractor))
	expectedAudio := File{ID: TestFileID, UniqueID: TestFileUniqueID}
	assert.Equal(t, expectedAudio, f.extractor(audioMsg))
}

func TestIntegerExtractor(t *testing.T) {
	assert.Equal(t, Int(42), integerExtractor(&tgbotapi.Message{Text: " 42 "}))
	assert.Nil(t, integerExtractor(&tgbotapi.Message{Text: "4.2"}))
}

func TestBooleanExtractor(t *testing.T) {
	assert.Equal(t, Bool(true), booleanExtractor(&tgbotapi.Message{Text: "true"}))
	assert.Equal(t, Bool(false), booleanExtractor(&tgbotapi.Message{Text: "0"}))
	assert.Nil(t, booleanExtractor(&tgbotapi.Message{Text: "yes"}))
}

func getFuncPtr(f any) uintptr {
	return reflect.ValueOf(f).Pointer()
}

func TestNewAutoField(t *testing.T) {
	entities := []tgbotapi.MessageEntity{{Type: "bold", Length: 4}}

	txt := NewAutoField(TestName, &tgbotapi.Message{Text: TestValue, Entities: entities})
	assert.Equal(t, TestName, txt.Name)
	assert.Equal(t, Text, txt.Type)
	assert.Equal(t, Txt{Value: TestValue, Entities: entities}, txt.Data)
	assert.Nil(t, txt.Form)

	sticker := NewAutoField(TestName, &tgbotapi.Message{Sticker: &tgbotapi.Sticker{FileID: TestFileID, FileUniqueID: TestFileUniqueID}})
	assert.Equal(t, Sticker, sticker.Type)
	assert.Equal(t, File{ID: TestFileID, UniqueID: TestFileUniqueID}, sticker.Data)

	photo := NewAutoField(TestName, &tgbotapi.Message{
		Photo:   []tgbotapi.PhotoSize{{FileID: "small", FileUniqueID: "smallU"}, {FileID: TestFileID, FileUniqueID: TestFileUniqueID}},
		Caption: TestValue,
	})
	assert.Equal(t, Image, photo.Type)
	assert.Equal(t, File{ID: TestFileID, UniqueID: TestFileUniqueID, Caption: TestValue}, photo.Data)

	location := NewAutoField(TestName, &tgbotapi.Message{Location: &tgbotapi.Location{Latitude: 1.5, Longitude: 2.5}})
	assert.Equal(t, Location, location.Type)
	assert.Equal(t, LocData{Latitude: 1.5, Longitude: 2.5}, location.Data)
}
