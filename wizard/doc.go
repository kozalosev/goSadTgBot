/*
Package wizard provides support for field based forms defined by client code. A key-value storage is used to save
the state of a form between messages.

To add a form to your [github.com/kozalosev/goSadTgBot/base.MessageHandler], it must implement the [WizardMessageHandler]
interface and create a [Wizard] in its [github.com/kozalosev/goSadTgBot/base.MessageHandler.Handle] method.

Values of fields implement the [Value] interface and keep their types when restored from a [StateStorage].
Read them with [GetData].

# Migration from v0.4

  - Field.Data is of the [Value] type now: one of [Txt], [File], [LocData], [Int] or [Bool].
  - AddPrefilledField(name, "string") → AddPrefilledText(name, "string").
  - Other prefilled values must be wrapped: AddPrefilledField(name, wizard.Int(id)), wizard.Bool(true).
  - FixDataTypes() was removed since it's not needed anymore.
  - GetData(field) and FieldData(fields, name) from v0.4.1 were merged into GetData(fields, name).
*/
package wizard
