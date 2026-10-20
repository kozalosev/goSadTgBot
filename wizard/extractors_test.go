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
