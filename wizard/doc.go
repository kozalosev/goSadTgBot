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
The cleanup after the forms is up to the bot: [SetGroupCleanupPolicy] gets the dialogue of a form when a question is
asked and when the form is completed, and decides what to delete, when, and whether to warn about it. [CleanupIfAllowed]
deletes the whole dialogue where the bot may delete messages of other users and nothing elsewhere. Without a policy,
nothing is deleted.

# Migration from v0.6

  - SetSelfDestructionDelayInGroups(delay) and SetUnansweredQuestionLifetimeInGroups(ttl) →
    SetGroupCleanupPolicy(CleanupIfAllowed(delay, ttl)). The policy deletes the whole dialogue by one task now, and
    nothing where the bot may not delete messages of other users, even an unanswered question.

# Migration from v0.4

  - Field.Data is of the [Value] type now: one of [Txt], [File], [LocData], [Int] or [Bool].
  - AddPrefilledField(name, "string") → AddPrefilledText(name, "string").
  - Other prefilled values must be wrapped: AddPrefilledField(name, wizard.Int(id)), wizard.Bool(true).
  - FixDataTypes() was removed since it's not needed anymore.
  - GetData(field) and FieldData(fields, name) from v0.4.1 were merged into GetData(fields, name).
*/
package wizard
