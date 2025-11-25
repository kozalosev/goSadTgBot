package base

import (
	"github.com/kozalosev/goSadTgBot/logconst"
	log "log/slog"
	"os"
	"strconv"
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
