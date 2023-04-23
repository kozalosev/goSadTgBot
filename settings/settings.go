// Package settings consists of an interface that must provide user settings to the bot.
package settings

// LangCode is a language code like 'ru' or 'en'.
type LangCode string

// UserOptions stored in the database.
type UserOptions any

type OptionsFetcher interface {
	FetchUserOptions(uid int64, defaultLang string) (LangCode, UserOptions)
}
