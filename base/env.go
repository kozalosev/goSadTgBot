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
		log.Error("error while initializing an environment variable",
			logconst.FieldFunc, "init",
			logconst.FieldConst, "BUTTONS_PER_ROW",
			logconst.FieldError, err)
	} else {
		buttonsPerRow = buttonsPerRowEnv
	}
}
