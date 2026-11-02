/*
Package wizard provides support for field based forms defined by client code. A key-value storage is used to save
the state of a form between messages.

To add a form to your [github.com/kozalosev/goSadTgBot/base.MessageHandler], it must implement the [WizardMessageHandler]
interface and create a [Wizard] in its [github.com/kozalosev/goSadTgBot/base.MessageHandler.Handle] method.

Values of fields implement the [Value] interface and keep their types when restored from a [StateStorage].
Read them with [GetData], or with [GetText] for text values.

# Group chats

A form works in group chats too. Each form is bound to the chat where it was started ([Form.ChatID]), and in a group
it's continued only by replies of the user to the bot. The questions are sent with ForceReply, so the client of the user
opens the reply interface, and the answer is delivered even in privacy mode. Reply keyboards are not shown in groups.
Call [SetSelfDestructionDelayInGroups] and [SetUnansweredQuestionLifetimeInGroups] to clean up after the forms: if
the bot may delete messages of other users, an answered question disappears after the delay together with the answer;
otherwise, the answered questions are kept along with the answers. A question nobody has answered disappears when
the form expires.

# Migration from v0.4

  - Field.Data is of the [Value] type now: one of [Txt], [File], [LocData], [Int] or [Bool].
  - AddPrefilledField(name, "string") → AddPrefilledText(name, "string").
  - Other prefilled values must be wrapped: AddPrefilledField(name, wizard.Int(id)), wizard.Bool(true).
  - FixDataTypes() was removed since it's not needed anymore.
  - GetData(field) and FieldData(fields, name) from v0.4.1 were merged into GetData(fields, name).
*/
package wizard
