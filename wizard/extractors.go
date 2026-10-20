package wizard

import (
	log "log/slog"
	"strconv"
	"strings"

	tgbotapi "github.com/OvyFlash/telegram-bot-api"
	"github.com/kozalosev/goSadTgBot/logconst"
)

type fieldExtractor func(msg *tgbotapi.Message) Value

// Txt is a structure for formatted text consisting of non-formatted text and 'entities'
// https://core.telegram.org/bots/api#messageentity
type Txt struct {
	Value    string
	Entities []tgbotapi.MessageEntity
}

// File is a representation of Telegram cached files.
// https://core.telegram.org/bots/api#file
type File struct {
	ID       string // file_id
	UniqueID string // file_unique_id
	Caption  string // optional, not for all types
	Entities []tgbotapi.MessageEntity
}

// LocData represents a point on the map.
// https://core.telegram.org/bots/api#location
type LocData struct {
	Latitude  float64
	Longitude float64
}

// Int is an integer value. It may be used to prefill fields with IDs and other numbers.
type Int int64

// Bool is a boolean value.
type Bool bool

// Value is a value of a [Field]. It's implemented only by [Txt], [File], [LocData], [Int] and [Bool].
type Value interface {
	valueType() valueType
}

// valueType is saved along with the value to restore it from a [StateStorage] with the right type.
type valueType string

const (
	txtValueType  valueType = "txt"
	fileValueType valueType = "file"
	locValueType  valueType = "location"
	intValueType  valueType = "int"
	boolValueType valueType = "bool"
)

func (Txt) valueType() valueType     { return txtValueType }
func (File) valueType() valueType    { return fileValueType }
func (LocData) valueType() valueType { return locValueType }
func (Int) valueType() valueType     { return intValueType }
func (Bool) valueType() valueType    { return boolValueType }

func nilExtractor(*tgbotapi.Message) Value { return nil }
func textExtractor(m *tgbotapi.Message) Value {
	return Txt{Value: m.Text, Entities: m.Entities}
}
func stickerExtractor(m *tgbotapi.Message) Value {
	if m.Sticker == nil {
		return nil
	}
	return File{ID: m.Sticker.FileID, UniqueID: m.Sticker.FileUniqueID}
}
func voiceExtractor(m *tgbotapi.Message) Value {
	if m.Voice == nil {
		return nil
	}
	return File{ID: m.Voice.FileID, UniqueID: m.Voice.FileUniqueID, Caption: m.Caption, Entities: m.CaptionEntities}
}
func audioExtractor(m *tgbotapi.Message) Value {
	if m.Audio == nil {
		return nil
	}
	return File{ID: m.Audio.FileID, UniqueID: m.Audio.FileUniqueID, Caption: m.Caption, Entities: m.CaptionEntities}
}
func videoExtractor(m *tgbotapi.Message) Value {
	if m.Video == nil {
		return nil
	}
	return File{ID: m.Video.FileID, UniqueID: m.Video.FileUniqueID, Caption: m.Caption, Entities: m.CaptionEntities}
}
func videoNoteExtractor(m *tgbotapi.Message) Value {
	if m.VideoNote == nil {
		return nil
	}
	return File{ID: m.VideoNote.FileID, UniqueID: m.VideoNote.FileUniqueID}
}
func gifExtractor(m *tgbotapi.Message) Value {
	if m.Animation == nil {
		return nil
	}
	return File{ID: m.Animation.FileID, UniqueID: m.Animation.FileUniqueID, Caption: m.Caption, Entities: m.CaptionEntities}
}
func documentExtractor(m *tgbotapi.Message) Value {
	if m.Document == nil {
		return nil
	}
	return File{ID: m.Document.FileID, UniqueID: m.Document.FileUniqueID, Caption: m.Caption, Entities: m.CaptionEntities}
}
func imageExtractor(m *tgbotapi.Message) Value {
	if m.Photo == nil || len(m.Photo) == 0 {
		return nil
	}
	photo := m.Photo[len(m.Photo)-1]
	return File{ID: photo.FileID, UniqueID: photo.FileUniqueID, Caption: m.Caption, Entities: m.CaptionEntities}
}
func locationExtractor(m *tgbotapi.Message) Value {
	if m.Location == nil {
		return nil
	}
	return LocData{Latitude: m.Location.Latitude, Longitude: m.Location.Longitude}
}
func integerExtractor(m *tgbotapi.Message) Value {
	val, err := strconv.ParseInt(strings.TrimSpace(m.Text), 10, 64)
	if err != nil {
		return nil
	}
	return Int(val)
}
func booleanExtractor(m *tgbotapi.Message) Value {
	val, err := strconv.ParseBool(strings.TrimSpace(m.Text))
	if err != nil {
		return nil
	}
	return Bool(val)
}

func determineMessageType(msg *tgbotapi.Message) FieldType {
	if msg.Sticker != nil {
		return Sticker
	}
	if msg.Photo != nil {
		return Image
	}
	if msg.Voice != nil {
		return Voice
	}
	if msg.Audio != nil {
		return Audio
	}
	if msg.Video != nil {
		return Video
	}
	if msg.VideoNote != nil {
		return VideoNote
	}
	if msg.Animation != nil {
		return Gif
	}
	if msg.Document != nil {
		return Document
	}
	if msg.Location != nil {
		return Location
	}
	return Text
}

func (f *Field) restoreExtractor(msg *tgbotapi.Message) {
	if f.extractor != nil {
		return
	}
	switch f.Type {
	case Auto:
		msgType := determineMessageType(msg)
		f.Type = msgType
		f.restoreExtractor(msg)
	case Text:
		f.extractor = textExtractor
	case Sticker:
		f.extractor = stickerExtractor
	case Image:
		f.extractor = imageExtractor
	case Voice:
		f.extractor = voiceExtractor
	case Audio:
		f.extractor = audioExtractor
	case Video:
		f.extractor = videoExtractor
	case VideoNote:
		f.extractor = videoNoteExtractor
	case Gif:
		f.extractor = gifExtractor
	case Document:
		f.extractor = documentExtractor
	case Location:
		f.extractor = locationExtractor
	case Integer:
		f.extractor = integerExtractor
	case Boolean:
		f.extractor = booleanExtractor
	default:
		log.Warn("No extractor was found for the field type",
			logconst.FieldObject, "Field",
			logconst.FieldCalledMethod, "restoreExtractor",
			logconst.FieldName, f.Name,
			logconst.FieldMessage, msg)
		f.extractor = nilExtractor
	}
}
