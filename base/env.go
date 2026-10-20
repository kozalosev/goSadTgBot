package base

import (
	log "log/slog"
	"os"
	"strconv"

	"github.com/kozalosev/goSadTgBot/logconst"
)

var buttonsPerRow = 6

func init() {
	val, ok := os.LookupEnv("BUTTONS_PER_ROW")
	if !ok {
		return
	}
	if buttonsPerRowEnv, err := strconv.Atoi(val); err != nil {
		logconst.LogFailToParseEnvironmentVariableWithContext(
			log.With(logconst.FieldFunc, "init"),
			"BUTTONS_PER_ROW", err)
	} else if buttonsPerRowEnv < 1 {
		log.Error("The environment variable must be positive",
			logconst.FieldFunc, "init",
			logconst.FieldConst, "BUTTONS_PER_ROW",
			logconst.FieldValue, buttonsPerRowEnv)
	} else {
		buttonsPerRow = buttonsPerRowEnv
	}
}
