goSadTgBot — SadBot's Framework for development of Telegram bots
================================================================
[![CI Build](https://github.com/kozalosev/goSadTgBot/actions/workflows/ci-build.yml/badge.svg?branch=main&event=push)](https://github.com/kozalosev/goSadTgBot/actions/workflows/ci-build.yml)

This framework is based on:
* the [go-telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api) library;
* PostgreSQL as database (by using [pgx](https://github.com/jackc/pgx)).

Packages
--------

* [base](base) contains base types like container for request environment or object for sending requests.
* [app](app) provides an additional container for the application environment and method for processing the updates.
* [server](server) allows you to use a [WebHook][setWebhook].
* [metrics](metrics) allows you to publish the `/metrics` endpoint for Prometheus.
* [storage](storage) creates a database connection and runs the migrations located in the `db/migrations` directory.
* [settings](settings) consists of an interface that must provide user settings to the bot.
* [wizard](wizard) provides facilities to create forms with fields of different types.
* [logconst](logconst) is just a set of constants for use in `log.WithField(logconst.*, ...)`.

Examples of usage
-----------------
* [SadFavBot][SadFavBot-repo] ([main.go][SadFavBot-main], [@bot][SadFavBot-bot])
* [PostSuggesterBot][PostSuggesterBot-repo] ([main.go][PostSuggesterBot-main], [@WellOfDesiresBot][WellOfDesiresBot])

[setWebhook]: https://core.telegram.org/bots/api#setwebhook

[SadFavBot-repo]: https://github.com/kozalosev/SadFavBot
[SadFavBot-main]: https://github.com/kozalosev/SadFavBot/blob/main/main.go
[SadFavBot-bot]: https://t.me/SadFavBot

[PostSuggesterBot-repo]: https://github.com/kozalosev/PostSuggesterBot
[PostSuggesterBot-main]: https://github.com/kozalosev/PostSuggesterBot/blob/main/main.go
[WellOfDesiresBot]: https://t.me/WellOfDesiresBot
