package base

import (
	"github.com/kozalosev/goSadTgBot/logconst"
	log "github.com/sirupsen/logrus"
	"os"
	"strconv"
)

var buttonsPerRow = 6

func init() {
	if buttonsPerRowEnv, err := strconv.Atoi(os.Getenv("BUTTONS_PER_ROW")); err != nil {
		log.WithField(logconst.FieldFunc, "init").
			WithField(logconst.FieldConst, "BUTTONS_PER_ROW").
			Error(err)
	} else {
		buttonsPerRow = buttonsPerRowEnv
	}
}
