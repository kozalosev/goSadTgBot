package base

import (
	log "log/slog"
	"os"
	"strconv"

	"github.com/kozalosev/goSadTgBot/logconst"
)

var buttonsPerRow = 6

func init() {
	if buttonsPerRowEnv, err := strconv.Atoi(os.Getenv("BUTTONS_PER_ROW")); err != nil {
		logconst.LogFailToParseEnvironmentVariableWithContext(
			log.With(logconst.FieldFunc, "init"),
			"BUTTONS_PER_ROW", err)
	} else {
		buttonsPerRow = buttonsPerRowEnv
	}
}
