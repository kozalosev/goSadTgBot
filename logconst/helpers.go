package logconst

import log "log/slog"

const (
	botApiRequestFailed   = "couldn't execute a request to Telegram API"
	scanDatabaseRowFailed = "couldn't deserialize a row got from a database table"
)

func FailedToScanDatabaseRow(serviceName, methodName string, err error) {
	NewLoggerForDatabaseService(serviceName).FailedToScanDatabaseRow(methodName, err)
}

func FailedToParseEnvironmentVariable(name string, err error) {
	FailedToParseEnvironmentVariableWithContext(log.Default(), name, err)
}

func FailedToParseEnvironmentVariableWithContext(logger *log.Logger, name string, err error) {
	logger.Error("parsing of an environment variable failed",
		FieldConst, name,
		FieldError, err)
}

type LoggerBuilder struct {
	logger *log.Logger
}

func NewLoggerForHandler(name string) LoggerBuilder {
	return LoggerBuilder{
		logger: log.With(
			FieldHandler, name,
			FieldMethod, "Handle",
		),
	}
}

func NewLoggerForDatabaseService(name string) LoggerBuilder {
	return LoggerBuilder{
		logger: log.With(FieldService, name),
	}
}

func (builder LoggerBuilder) ForMethod(name string) *log.Logger {
	return builder.With(FieldMethod, name)
}

func (builder LoggerBuilder) With(args ...any) *log.Logger {
	return builder.logger.With(args...)
}

func (builder LoggerBuilder) Use() *log.Logger {
	return builder.logger
}

func (builder LoggerBuilder) FailedTelegramApiRequest(err error) {
	builder.logger.Error(botApiRequestFailed,
		FieldCalledObject, "BotAPI",
		FieldCalledMethod, "Request",
		FieldError, err)
}

func (builder LoggerBuilder) FailedToScanDatabaseRow(methodName string, err error) {
	builder.logger.Error(scanDatabaseRowFailed,
		FieldMethod, methodName,
		FieldCalledObject, "Rows",
		FieldCalledMethod, "Scan",
		FieldError, err)
}
