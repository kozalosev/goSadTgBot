package wizard

import (
	"encoding/json"
	"errors"
	"fmt"
)

var errUnknownValueType = errors.New("unknown type of the field value")

// fieldAlias has the same fields as [Field] but none of its methods, to avoid recursion in the JSON methods.
type fieldAlias Field

// fieldJSON shadows Data of the embedded [Field] to store the value along with its type.
type fieldJSON struct {
	*fieldAlias
	Data     json.RawMessage `json:"data,omitempty"`
	DataType valueType       `json:"dataType,omitempty"`
}

func (f *Field) MarshalJSON() ([]byte, error) {
	aux := fieldJSON{fieldAlias: (*fieldAlias)(f)}
	if f.Data != nil {
		data, err := json.Marshal(f.Data)
		if err != nil {
			return nil, err
		}
		aux.Data = data
		aux.DataType = f.Data.valueType()
	}
	return json.Marshal(aux)
}

func (f *Field) UnmarshalJSON(b []byte) error {
	aux := fieldJSON{fieldAlias: (*fieldAlias)(f)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	if len(aux.Data) == 0 || string(aux.Data) == "null" {
		f.Data = nil
		return nil
	}

	dataType := aux.DataType
	if dataType == "" {
		// states saved by older versions have no dataType
		dataType = valueTypeOfField(f.Type)
	}
	var err error
	switch dataType {
	case txtValueType:
		f.Data, err = decodeAs[Txt](aux.Data)
	case fileValueType:
		f.Data, err = decodeAs[File](aux.Data)
	case locValueType:
		f.Data, err = decodeAs[LocData](aux.Data)
	case intValueType:
		f.Data, err = decodeAs[Int](aux.Data)
	case boolValueType:
		f.Data, err = decodeAs[Bool](aux.Data)
	default:
		err = fmt.Errorf("%w: field %q", errUnknownValueType, f.Name)
	}
	return err
}

func decodeAs[T Value](raw json.RawMessage) (Value, error) {
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func valueTypeOfField(fieldType FieldType) valueType {
	switch fieldType {
	case Text:
		return txtValueType
	case Sticker, Image, Voice, Audio, Video, VideoNote, Gif, Document:
		return fileValueType
	case Location:
		return locValueType
	case Integer:
		return intValueType
	case Boolean:
		return boolValueType
	default:
		return ""
	}
}
