package wizard

import (
	"fmt"
	"github.com/kozalosev/goSadTgBot/logconst"
	log "log/slog"
)

// SkipCondition is the condition type for [FieldDescriptor.SkipIf] field.
type SkipCondition interface {
	ShouldBeSkipped(form *Form) bool
}

// SkipOnFieldValue is a [SkipCondition] implementation that skips the field if the value of another field is equal to Value.
type SkipOnFieldValue struct {
	Name  string
	Value string
}

func (s SkipOnFieldValue) ShouldBeSkipped(form *Form) bool {
	f := form.Fields.FindField(s.Name)
	if f == nil {
		log.Warn(fmt.Sprintf("Field '%s' was not found to check if '%s' should be skipped!", s.Name, form.Fields[form.Index].Name),
			logconst.FieldObject, "SkipOnFieldValue",
			logconst.FieldCalledMethod, "ShouldBeSkipped")
		return false
	}
	txtData, ok := f.Data.(Txt)
	return ok && txtData.Value == s.Value
}

// SkipIfFieldNotEmpty is another [SkipCondition] implementation which gives a way to express the intention to fill
// one of two fields but not both.
type SkipIfFieldNotEmpty struct {
	Name string
}

func (s SkipIfFieldNotEmpty) ShouldBeSkipped(form *Form) bool {
	return form.Fields.FindField(s.Name).Data != nil
}
