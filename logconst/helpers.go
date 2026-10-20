package logconst

import log "log/slog"

const (
	botApiRequestFailed   = "Failed to execute a request to Telegram API"
	scanDatabaseRowFailed = "Failed to deserialize a row, got from a database table"
	envParsingFailed      = "Failed to parse an environment variable"
)

// LogFailToScanDatabaseRow is used to log errors occurred while scanning a row, got from a database table.
func LogFailToScanDatabaseRow(serviceName, methodName string, err error) {
	NewLoggerForDatabaseService(serviceName).LogFailToScanDatabaseRow(methodName, err)
}

// LogFailToParseEnvironmentVariable is used to log errors occurred while parsing values of environment variables.
func LogFailToParseEnvironmentVariable(name string, err error) {
	LogFailToParseEnvironmentVariableWithContext(log.Default(), name, err)
}

func LogFailToParseEnvironmentVariableWithContext(logger *log.Logger, name string, err error) {
	logger.Error(envParsingFailed,
		FieldConst, name,
		FieldError, err)
}

// LoggerBuilder is a helper struct to provide a fluent and concise API of logger arguments context creation.
// It stores only the arguments and resolves [log.Default] on every use, so it's safe to create builders
// in package-level variables before the application calls [log.SetDefault].
type LoggerBuilder struct {
	args []any
}

// NewLoggerForHandler is a constructor of LoggerBuilder with FieldHandler == 'name' and FieldMethod == "Handle".
func NewLoggerForHandler(name string) LoggerBuilder {
	return LoggerBuilder{
		args: []any{
			FieldHandler, name,
			FieldMethod, "Handle",
		},
	}
}

// NewLoggerForDatabaseService is a constructor of LoggerBuilder with FieldService == 'name'.
func NewLoggerForDatabaseService(name string) LoggerBuilder {
	return LoggerBuilder{
		args: []any{FieldService, name},
	}
}

// ForMethod is a helper function to set FieldMethod to 'name' and return an internal logger.
func (builder LoggerBuilder) ForMethod(name string) *log.Logger {
	return builder.With(FieldMethod, name)
}

// With is a helper function to set arguments context and return an internal logger.
func (builder LoggerBuilder) With(args ...any) *log.Logger {
	return builder.Use().With(args...)
}

// Use returns an internal logger.
func (builder LoggerBuilder) Use() *log.Logger {
	return log.Default().With(builder.args...)
}

// LogFailedTelegramApiRequest is used to log errors occurred while sending a Telegram API request.
func (builder LoggerBuilder) LogFailedTelegramApiRequest(err error) {
	builder.Use().Error(botApiRequestFailed,
		FieldCalledObject, "BotAPI",
		FieldCalledMethod, "Request",
		FieldError, err)
}

// LogFailToScanDatabaseRow is used to log errors occurred while scanning a row, got from a database table.
func (builder LoggerBuilder) LogFailToScanDatabaseRow(methodName string, err error) {
	builder.Use().Error(scanDatabaseRowFailed,
		FieldMethod, methodName,
		FieldCalledObject, "Rows",
		FieldCalledMethod, "Scan",
		FieldError, err)
}
